package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogActivationFactsCloneFormulaAndRuleBoundary(t *testing.T) {
	f := newDirectFactFixture(t)
	m, e := f.manager, f.engine
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(text string) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p, q := declare("bound-interval-product-P"), declare("bound-interval-product-Q")
	nullaryName, err := m.Symbols.Intern("bound-interval-product-unit")
	must(err)
	unit, err := m.MkFuncDecl(nullaryName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(unit))
	predicates := []boundIntervalDatalogPredicate{
		{declaration: p, kinds: []string{"bound_relation", "interval_relation"}},
		{declaration: q, kinds: []string{"interval_relation", "bound_relation"}},
		{declaration: unit, kinds: []string{"bound_relation", "interval_relation"}},
	}
	activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, predicates)
	must(err)
	if !selected || activation == nil {
		t.Fatal("explicit source product pair was not selected")
	}
	defer func() { must(activation.close()) }()
	if got := activation.entries[p].kinds; len(got) != 2 || got[0] != "bound_relation" || got[1] != "interval_relation" {
		t.Fatal("product component order was not preserved", got)
	}
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	must(activation.addFact(p, []ast.ASTHandle{two}))
	clone, err := activation.clone(e.set)
	must(err)
	defer func() { must(clone.close()) }()
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	must(activation.addFact(q, []ast.ASTHandle{three}))
	if status, cause := clone.query([]ast.ASTHandle{q}); cause != nil || status != lbool.False {
		t.Fatal("clone observed later product fact", status, cause)
	}
	if status, cause := clone.query([]ast.ASTHandle{p}); cause != nil || status != lbool.Undef {
		t.Fatal("nonempty imprecise product status", status, cause)
	}
	if status, cause := clone.query(nil); cause != nil || status != lbool.True {
		t.Fatal("zero-relation source query status", status, cause)
	}
	must(clone.addFact(unit, nil))
	if status, cause := clone.query([]ast.ASTHandle{unit}); cause != nil || status != lbool.Undef {
		t.Fatal("nonempty nullary product status", status, cause)
	}
	formula, err := clone.relationFormula(p)
	must(err)
	if _, err = m.RefGet(formula); err != nil {
		t.Fatal("product answer formula is not owned", err)
	}
	must(m.DelRef(&formula))

	qFact, err := m.MkApp(q, []ast.ASTHandle{three})
	must(err)
	must(e.AddRule(qFact, "bound-interval-ground-fact"))
	if status, cause := activation.query([]ast.ASTHandle{q}); cause != nil || status != lbool.Undef {
		t.Fatal("ground fact rule did not populate product", status, cause)
	}
	x, err := m.MkVar(0, integer)
	must(err)
	pTail, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qHead, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(pTail, qHead)
	must(err)
	must(e.AddRule(rule, "bound-interval-positive-pending"))
	if _, cause := activation.query([]ast.ASTHandle{q}); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
		t.Fatal("unfinished positive product rule silently approximated", cause)
	}
	if _, cause := activation.relationFormula(q); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
		t.Fatal("failed product execution exposed stale answer", cause)
	}
	m.Limit.Cancel()
	status, cause := clone.query([]ast.ASTHandle{p})
	m.Limit.ResetCancel()
	if status != lbool.Undef || cause == nil {
		t.Fatal("product execution ignored preexisting cancellation", status, cause)
	}
	if _, cause = clone.relationFormula(p); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
		t.Fatal("canceled product execution exposed stale answer", cause)
	}
}

func TestBoundIntervalDatalogActivationDeclinesOtherKinds(t *testing.T) {
	f := newDirectFactFixture(t)
	p := f.declarations["p"]
	activation, selected, err := newBoundIntervalDatalogActivation(f.manager, f.engine.set, []boundIntervalDatalogPredicate{{declaration: p, kinds: []string{"bound_relation"}}})
	if err != nil || selected || activation != nil {
		t.Fatal("single direct relation kind selected product activation", activation, selected, err)
	}
}
