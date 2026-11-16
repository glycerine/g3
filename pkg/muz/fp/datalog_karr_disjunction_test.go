package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestKarrDisjunctionRetainedRuleShape(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(err)
	defer func() { must(e.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := decl("karr-disj-shape-p"), decl("karr-disj-shape-q")
	x, err := m.MkVar(0, integer)
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	eq1, err := m.MkEq(x, one)
	must(err)
	eq2, err := m.MkEq(x, three)
	must(err)
	guard, err := m.MkOr(eq1, eq2)
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	formula, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(formula, ""))
	must(e.flushPendingRules())
	n, err := e.set.Len()
	must(err)
	if n != 1 {
		t.Fatalf("OR split into %d rules", n)
	}
	rule, err := e.set.Rule(0)
	must(err)
	defer func() { must(rule.Close()) }()
	n, err = rule.TailSize()
	must(err)
	u, err := rule.UninterpretedTailSize()
	must(err)
	if n != 2 || u != 1 {
		t.Fatalf("rule shape %d/%d", n, u)
	}
	tail, neg, err := rule.Tail(1)
	must(err)
	op, err := m.GetDeclKind(tail)
	must(err)
	if neg || op != ast.ASTOpOr {
		t.Fatalf("interpreted disjunction %v %d", neg, op)
	}
}
