package fp

import (
	"errors"
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
	must(e.addRuleEager(rule, "bound-interval-filter-project"))
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

func TestBoundIntervalDatalogTwoPositiveJoinFilterProjectAndRepeat(t *testing.T) {
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
		declaration, cause := m.MkFuncDecl(name, []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("bound-interval-two-tail-P")
	q := declare("bound-interval-two-tail-Q")
	r := declare("bound-interval-two-tail-R")
	pKinds := []string{"bound_relation", "interval_relation"}
	qKinds := []string{"interval_relation", "bound_relation"}
	activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, []boundIntervalDatalogPredicate{
		{declaration: p, kinds: pKinds},
		{declaration: q, kinds: qKinds},
		{declaration: r, kinds: pKinds},
	})
	must(err)
	if !selected {
		t.Fatal("two-tail exact product program was not selected")
	}
	defer func() { must(activation.close()) }()
	numeral := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		must(cause)
		return value
	}
	for _, values := range [][]ast.ASTHandle{{numeral("1"), numeral("10")}, {numeral("2"), numeral("20")}} {
		must(activation.addFact(p, values))
	}
	for _, values := range [][]ast.ASTHandle{{numeral("10"), numeral("100")}, {numeral("20"), numeral("200")}} {
		must(activation.addFact(q, values))
	}
	x, err := m.MkVar(0, integer)
	must(err)
	y, err := m.MkVar(1, integer)
	must(err)
	z, err := m.MkVar(2, integer)
	must(err)
	pTail, err := m.MkApp(p, []ast.ASTHandle{x, y})
	must(err)
	qTail, err := m.MkApp(q, []ast.ASTHandle{y, z})
	must(err)
	condition, err := m.MkArithmeticLE(z, numeral("150"))
	must(err)
	body, err := m.MkAnd3(pTail, qTail, condition)
	must(err)
	head, err := m.MkApp(r, []ast.ASTHandle{z, x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.addRuleEager(rule, "bound-interval-two-tail-filter-project"))

	check := func(maxX int64) {
		t.Helper()
		status, cause := activation.query([]ast.ASTHandle{r})
		must(cause)
		if status != lbool.Undef {
			t.Fatal("two-tail product result status", status)
		}
		result := activation.evaluated[r]
		if result == nil || !result.ContainsFact([]*big.Rat{big.NewRat(100, 1), big.NewRat(1, 1)}) ||
			!result.ContainsFact([]*big.Rat{big.NewRat(150, 1), big.NewRat(maxX, 1)}) ||
			result.ContainsFact([]*big.Rat{big.NewRat(151, 1), big.NewRat(1, 1)}) {
			t.Fatal("two-tail join/filter/project/reorder result", result, maxX)
		}
		if !activation.evaluated[p].ContainsFact([]*big.Rat{big.NewRat(maxX, 1), big.NewRat(20, 1)}) ||
			!activation.evaluated[q].ContainsFact([]*big.Rat{big.NewRat(20, 1), big.NewRat(200, 1)}) {
			t.Fatal("two-tail evaluation mutated a source snapshot")
		}
	}
	check(2)
	check(2)
	must(activation.addFact(p, []ast.ASTHandle{numeral("4"), numeral("20")}))
	check(4)
}

func TestBoundIntervalDatalogTwoPositiveDisjointAndThreeTailBoundary(t *testing.T) {
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
	p := declare("bound-interval-disjoint-P")
	q := declare("bound-interval-disjoint-Q")
	r := declare("bound-interval-disjoint-R")
	s := declare("bound-interval-disjoint-S")
	kinds := []string{"bound_relation", "interval_relation"}
	activation, selected, err := newBoundIntervalDatalogActivation(m, e.set, []boundIntervalDatalogPredicate{
		{declaration: p, kinds: kinds}, {declaration: q, kinds: kinds},
		{declaration: r, kinds: kinds}, {declaration: s, kinds: kinds},
	})
	must(err)
	if !selected {
		t.Fatal("disjoint exact product program was not selected")
	}
	defer func() { must(activation.close()) }()
	numeral := func(text string) ast.ASTHandle {
		value, cause := m.ArithmeticIntegerNumeralText(text)
		must(cause)
		return value
	}
	for _, value := range []string{"1", "2"} {
		must(activation.addFact(p, []ast.ASTHandle{numeral(value)}))
	}
	for _, value := range []string{"5", "6"} {
		must(activation.addFact(q, []ast.ASTHandle{numeral(value)}))
	}
	x, err := m.MkVar(0, integer)
	must(err)
	app := func(declaration ast.ASTHandle) ast.ASTHandle {
		result, cause := m.MkApp(declaration, []ast.ASTHandle{x})
		must(cause)
		return result
	}
	body, err := m.MkAnd(app(p), app(q))
	must(err)
	rule, err := m.MkImplies(body, app(r))
	must(err)
	must(e.addRuleEager(rule, "bound-interval-two-tail-disjoint"))
	status, err := activation.query([]ast.ASTHandle{r})
	must(err)
	// The join itself is empty, but C++ mk_project rebuilds a full result
	// without carrying the empty flag through head-column projection.
	if status != lbool.Undef || activation.evaluated[r] == nil || activation.evaluated[r].Empty() {
		t.Fatal("disjoint join projection did not match C++ behavior", status)
	}
	threeBody, err := m.MkAnd3(app(p), app(q), app(r))
	must(err)
	threeRule, err := m.MkImplies(threeBody, app(s))
	must(err)
	must(e.addRuleEager(threeRule, "bound-interval-three-tail-unsupported"))
	if _, cause := activation.query([]ast.ASTHandle{s}); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
		t.Fatal("three-positive-tail rule was not rejected", cause)
	}
	if _, cause := activation.relationFormula(r); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
		t.Fatal("failed three-tail execution exposed stale answer", cause)
	}
}
