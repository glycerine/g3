package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBoundIntervalDatalogPositiveFilterProjectConsumer(t *testing.T) {
	f := newDirectFactFixture(t)
	m, e := f.manager, f.engine
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		must(cause)
		declaration, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("bound-interval-filter-project-P", []ast.ASTHandle{integer, integer})
	q := declare("bound-interval-filter-project-Q", []ast.ASTHandle{integer})
	kinds := []string{"bound_relation", "interval_relation"}
	activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, []boundIntervalDatalogPredicate{{declaration: p, kinds: kinds}, {declaration: q, kinds: kinds}})
	must(err)
	if !selected {
		t.Fatal("exact product program was not selected")
	}
	defer func() { must(activation.close()) }()
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	five, err := m.ArithmeticIntegerNumeralText("5")
	must(err)
	ten, err := m.ArithmeticIntegerNumeralText("10")
	must(err)
	twenty, err := m.ArithmeticIntegerNumeralText("20")
	must(err)
	must(activation.addFact(p, []ast.ASTHandle{two, ten}))
	must(activation.addFact(p, []ast.ASTHandle{five, twenty}))
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	tail, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	condition, err := m.MkArithmeticLE(x, three)
	must(err)
	body, err := m.MkAnd(tail, condition)
	must(err)
	head, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(rule, "bound-interval-filter-project"))
	status, err := activation.query([]ast.ASTHandle{q})
	must(err)
	if status != lbool.Undef {
		t.Fatal("nonempty product rule result was not unknown", status)
	}
	result := activation.evaluated[q]
	if result == nil || !result.ContainsFact([]*big.Rat{big.NewRat(2, 1)}) || !result.ContainsFact([]*big.Rat{big.NewRat(3, 1)}) || result.ContainsFact([]*big.Rat{big.NewRat(4, 1)}) {
		t.Fatal("compiled product filter/project result", result)
	}
	if !activation.evaluated[p].ContainsFact([]*big.Rat{big.NewRat(5, 1), big.NewRat(20, 1)}) {
		t.Fatal("positive rule evaluation mutated its source tail")
	}
}
