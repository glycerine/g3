package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestKarrNegatedOrNormalizesToTwoGuards(t *testing.T) {
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
	p, q := decl("karr-notor-p"), decl("karr-notor-q")
	x, err := m.MkVar(0, integer)
	must(err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	lt, err := m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{x, zero})
	must(err)
	gt, err := m.MkFamilyApp(family, 5, nil, []ast.ASTHandle{x, three})
	must(err)
	either, err := m.MkOr(lt, gt)
	must(err)
	guard, err := m.MkNot(either)
	must(err)
	input, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	output, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	body, err := m.MkAnd(input, guard)
	must(err)
	formula, err := m.MkImplies(body, output)
	must(err)
	must(e.addRuleEager(formula, ""))
	must(e.flushPendingRules())
	n, err := e.set.Len()
	must(err)
	if n != 1 {
		t.Fatal("rules", n)
	}
	rule, err := e.set.Rule(0)
	must(err)
	defer func() { must(rule.Close()) }()
	n, err = rule.TailSize()
	must(err)
	if n != 3 {
		t.Fatal("tail size", n)
	}
	for i := 1; i < n; i++ {
		tail, neg, err := rule.Tail(i)
		must(err)
		op, err := m.GetDeclKind(tail)
		must(err)
		if neg || op != ast.ASTOpNot {
			t.Fatal("normalized comparison", i, op, neg)
		}
	}

}
