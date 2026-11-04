package fp

import (
	"errors"
	"math/big"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrMultipleGuardsPublicAndRetry(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string, n int) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		sorts := make([]ast.ASTHandle, n)
		for i := range sorts {
			sorts[i] = integer
		}
		d, err := m.MkFuncDecl(sym, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q, projected := decl("multi-guard-p", 2), decl("multi-guard-q", 2), decl("multi-guard-project", 1)
	must(e.AddTableFact(p, []uint32{2, 3}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	g1, err := m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{x, zero})
	must(err)
	g2, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{y, three})
	must(err)
	eq1, err := m.MkEq(x, one)
	must(err)
	eq3, err := m.MkEq(x, three)
	must(err)
	g3, err := m.MkOr(eq1, eq3)
	must(err)
	body, err := m.MkAnd(input, g1)
	must(err)
	body, err = m.MkAnd(body, g2)
	must(err)
	body, err = m.MkAnd(body, g3)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, args)
		must(err)
		return h
	}
	for _, h := range []ast.ASTHandle{app(q, x, y), app(projected, x)} {
		formula, err := m.MkImplies(body, h)
		must(err)
		must(e.AddRule(formula, ""))
	}
	for _, d := range []ast.ASTHandle{q, projected} {
		status, err := e.QueryPredicate(d)
		must(err)
		if status != lbool.Undef {
			t.Fatal("query", status)
		}
	}
	count, err := e.set.Len()
	must(err)
	var plan karrGuardRule
	found := false
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		must(err)
		d, p, err := e.karrDatalog.compileGuardRule(rule)
		must(err)
		must(rule.Close())
		if d == q {
			plan = p
			found = true
		}
		if d == projected && !reflect.DeepEqual(p.projection, []uint32{1}) {
			t.Fatal("projection", p.projection)
		}
	}
	if !found || len(plan.more) != 2 {
		t.Fatal("conjunction plan", plan)
	}
	filtered, err := e.karrDatalog.applyGuardRule(plan, rel.NewKarrFull(2, nil))
	must(err)
	rows, err := filtered.Inequalities()
	must(err)
	if rows.Len() != 4 {
		t.Fatal("conjunct rows", rows)
	}
	want := [][]int64{{1, 0, -1}, {-1, 0, 3}, {1, 0, 0}, {0, -1, 3}}
	for i, w := range want {
		if rows.A[i][0].Cmp(big.NewRat(w[0], 1)) != 0 || rows.A[i][1].Cmp(big.NewRat(w[1], 1)) != 0 || rows.B[i].Cmp(big.NewRat(w[2], 1)) != 0 {
			t.Fatal("row order", i, rows)
		}
	}
	answer, err := e.Answer(projected)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	expr, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	for _, tc := range []struct {
		val  string
		want bool
	}{{"2", true}, {"3", false}} {
		v, err := m.ArithmeticIntegerNumeralText(tc.val)
		must(err)
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(subst.Substitute(expr, []ast.ASTHandle{v}, ref))
		h, err := m.RefGet(ref)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		h, err = m.RefGet(reduced)
		must(err)
		if (tc.want && !m.IsTrue(h)) || (!tc.want && !m.IsFalse(h)) {
			t.Fatal("answer", tc, h)
		}
		must(m.DelRef(&ref))
		must(m.DelRef(&reduced))
	}
	armed, fired := true, false
	seed := rel.NewKarrEmpty(2, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(seed.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}))
	e.karrDatalog.seeds[p] = seed
	status, err := e.karrDatalog.query([]ast.ASTHandle{projected})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || e.karrDatalog.results != nil {
		t.Fatal("cancel", fired, status, err)
	}
	armed = false
	status, err = e.karrDatalog.query([]ast.ASTHandle{projected})
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
	large, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("numeral")
	}
	oversized := rel.NewKarrEmpty(2, nil)
	must(oversized.AddFact([]*big.Rat{large, big.NewRat(3, 1)}))
	e.karrDatalog.seeds[p] = oversized
	status, err = e.karrDatalog.query([]ast.ASTHandle{projected})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || e.karrDatalog.results != nil {
		t.Fatal("overflow", status, err)
	}
	e.karrDatalog.seeds[p] = seed
	status, err = e.karrDatalog.query([]ast.ASTHandle{projected})
	must(err)
	if status != lbool.Undef {
		t.Fatal("overflow retry", status)
	}
	// A later unsupported conjunct must reject the complete snapshot; after
	// removing that rule the same activation and seed remain reusable.
	mul, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{x, y})
	must(err)
	nonlinear, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{mul, three})
	must(err)
	badBody, err := m.MkAnd(input, g1)
	must(err)
	badBody, err = m.MkAnd(badBody, nonlinear)
	must(err)
	bad, err := m.MkImplies(badBody, app(q, x, y))
	must(err)
	prior, err := e.set.Len()
	must(err)
	must(e.AddRule(bad, ""))
	must(e.flushPendingRules())
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, ErrKarrDatalogFragment) || e.karrDatalog.results != nil {
		t.Fatal("unsupported conjunct", status, err)
	}
	rule, err := e.set.Rule(prior)
	must(err)
	must(e.set.Remove(rule))
	must(rule.Close())
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("unsupported conjunct retry", status)
	}
}
