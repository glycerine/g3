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

func TestDatalogKarrGuardedConstantTailPublicAndRetry(t *testing.T) {
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
	p, q := decl("guard-constant-p", 4), decl("guard-constant-q", 2)
	must(e.AddTableFact(p, []uint32{1, 2, 1, 3}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	num := func(s string) ast.ASTHandle { h, err := m.ArithmeticIntegerNumeralText(s); must(err); return h }
	bodyAtom, err := m.MkApp(p, []ast.ASTHandle{num("7"), x, num("8"), y})
	must(err)
	guard, err := m.MkFamilyApp(family, 5, nil, []ast.ASTHandle{y, num("0")})
	must(err)
	body, err := m.MkAnd(bodyAtom, guard)
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{y, x})
	must(err)
	formula, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(formula, ""))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("query", status)
	}
	rule, err := e.set.Rule(0)
	must(err)
	_, plan, err := e.karrDatalog.compileGuardRule(rule)
	must(err)
	must(rule.Close())
	if !reflect.DeepEqual(plan.inputConstants, []uint32{0, 1}) || len(plan.row) != 2 || plan.row[0].Sign() != 0 || plan.row[1].Cmp(big.NewRat(1, 1)) != 0 || plan.constant.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("column offsets", plan)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	expr, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	for _, tc := range []struct {
		vals []string
		want bool
	}{{[]string{"3", "2"}, true}, {[]string{"2", "3"}, false}} {
		args := []ast.ASTHandle{num(tc.vals[0]), num(tc.vals[1])}
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(subst.Substitute(expr, args, ref))
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
	// The source quirk filters both printed constants to 1, and filtering a
	// clone must leave the original predicate's fact intact.
	rows, err := e.karrDatalog.results[p].Inequalities()
	must(err)
	if rows.Len() != 4 {
		t.Fatal("source changed", rows)
	}
	empty, err := e.karrDatalog.applyGuardRule(plan, rel.NewKarrEmpty(4, nil))
	must(err)
	if !empty.Empty() || empty.Dimension() != 2 {
		t.Fatal("empty input", empty)
	}
	armed, fired := true, false
	seed := rel.NewKarrEmpty(4, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(seed.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(1, 1), big.NewRat(3, 1)}))
	e.karrDatalog.seeds[p] = seed
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	m.Limit.ResetCancel()
	if !fired || err == nil || status != lbool.Undef || e.karrDatalog.results != nil {
		t.Fatal("cancel", fired, status, err)
	}
	armed = false
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("retry", status)
	}
	large, ok := new(big.Rat).SetString("9223372036854775808")
	if !ok {
		t.Fatal("numeral")
	}
	oversized := rel.NewKarrEmpty(4, nil)
	must(oversized.AddFact([]*big.Rat{big.NewRat(1, 1), large, big.NewRat(1, 1), big.NewRat(3, 1)}))
	e.karrDatalog.seeds[p] = oversized
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	if status != lbool.Undef || !errors.Is(err, hilbert.ErrOverflow) || e.karrDatalog.results != nil {
		t.Fatal("overflow", status, err)
	}
	e.karrDatalog.seeds[p] = seed
	status, err = e.karrDatalog.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("overflow retry", status)
	}
}
