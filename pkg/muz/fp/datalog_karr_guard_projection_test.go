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

func TestDatalogKarrFilterProjectPublicAndRetry(t *testing.T) {
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
	p, q, unit := decl("guard-project-p", 3), decl("guard-project-q", 2), decl("guard-project-unit", 0)
	must(e.AddTableFact(p, []uint32{2, 3, 4}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x, y, z})
	must(err)
	guard, err := m.MkFamilyApp(family, 5, nil, []ast.ASTHandle{y, one})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{z, x})
	must(err)
	formula, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(formula, ""))
	nullary, err := m.MkApp(unit, nil)
	must(err)
	formula, err = m.MkImplies(body, nullary)
	must(err)
	must(e.AddRule(formula, ""))
	for _, d := range []ast.ASTHandle{q, unit} {
		status, err := e.QueryPredicate(d)
		must(err)
		if status != lbool.Undef {
			t.Fatal("query", status)
		}
	}
	count, err := e.set.Len()
	must(err)
	found := 0
	var qPlan karrGuardRule
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		must(err)
		d, plan, err := e.karrDatalog.compileGuardRule(rule)
		must(err)
		must(rule.Close())
		if d == q {
			qPlan = plan
		}
		if d == q && !reflect.DeepEqual(plan.projection, []uint32{1}) {
			t.Fatal("projection", plan.projection)
		}
		if d == unit && !reflect.DeepEqual(plan.projection, []uint32{0, 1, 2}) {
			t.Fatal("all columns", plan.projection)
		}
		found++
	}
	if found != 2 {
		t.Fatal("rules", found)
	}
	// The native filter retains a cached basis even for a contradictory
	// constraint; the combined operation projects that same basis.
	cached := rel.NewKarrEmpty(3, nil)
	must(cached.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1), big.NewRat(4, 1)}))
	_, err = cached.Basis()
	must(err)
	contradictory := qPlan
	contradictory.constant = *big.NewRat(-100, 1)
	candidate, err := e.karrDatalog.applyGuardRule(contradictory, cached)
	must(err)
	basis, err := candidate.Basis()
	must(err)
	if basis.Len() != 1 || basis.A[0][0].Cmp(big.NewRat(4, 1)) != 0 || basis.A[0][1].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("cached filtered projection", basis)
	}
	original, err := cached.Inequalities()
	must(err)
	if original.Len() != 3 {
		t.Fatal("source mutated", original)
	}
	empty, err := e.karrDatalog.applyGuardRule(qPlan, rel.NewKarrEmpty(3, nil))
	must(err)
	if !empty.Empty() || empty.Dimension() != 2 {
		t.Fatal("empty projection", empty)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	h, err := m.RefGet(answer)
	must(err)
	subst, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	defer func() { must(subst.Close()) }()
	for _, tc := range []struct {
		vals []string
		want bool
	}{{[]string{"4", "2"}, true}, {[]string{"2", "4"}, false}} {
		args := make([]ast.ASTHandle, 2)
		for i, v := range tc.vals {
			args[i], err = m.ArithmeticIntegerNumeralText(v)
			must(err)
		}
		ref, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(subst.Substitute(h, args, ref))
		v, err := m.RefGet(ref)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(v, nil, reduced))
		v, err = m.RefGet(reduced)
		must(err)
		if (tc.want && !m.IsTrue(v)) || (!tc.want && !m.IsFalse(v)) {
			t.Fatal("answer", tc.vals, tc.want, v)
		}
		must(m.DelRef(&ref))
		must(m.DelRef(&reduced))
	}
	armed, fired := true, false
	seed := rel.NewKarrEmpty(3, func() bool {
		if armed {
			fired = true
			m.Limit.Cancel()
			return false
		}
		return true
	})
	must(seed.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1), big.NewRat(4, 1)}))
	e.karrDatalog.seeds[p] = seed
	status, err := e.karrDatalog.query([]ast.ASTHandle{q})
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
	oversized := rel.NewKarrEmpty(3, nil)
	must(oversized.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1), large}))
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
