package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTLinearArithmeticStandaloneTermDefinition(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	two, err := m.ArithmeticRealNumeralText("2")
	astTestMust(t, err)
	realSort, err := m.GetSort(one)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "linear-definition-x"), realSort)
	astTestMust(t, err)
	sum, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, one})
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(sum, false))
	if feasible, rebuildErr := theory.rebuild(); rebuildErr != nil || !feasible {
		t.Fatal("standalone definition rebuild", feasible, rebuildErr)
	}
	node := state.context.enodes.get(sum)
	if node == nil || !theory.IsAttachedToVariable(node) {
		t.Fatal("standalone term was not attached")
	}
	variable := node.TheoryVar(family)
	value := theory.solver.Value(theory.simplexVariables[variable])
	if value.Main.Cmp(big.NewRat(1, 1)) != 0 || value.Epsilon.Sign() != 0 {
		t.Fatal("initial standalone term value", value.String())
	}
	bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, two})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(bound, true))
	boolean, found := state.BoolVar(bound)
	if !found {
		t.Fatal("bound Boolean variable")
	}
	astTestMust(t, theory.Assign(boolean, true))
	astTestMust(t, theory.Propagate())
	value = theory.solver.Value(theory.simplexVariables[variable])
	if value.Main.Cmp(big.NewRat(3, 1)) != 0 || value.Epsilon.Sign() != 0 {
		t.Fatal("updated standalone term value", value.String())
	}
}

func TestSMTLinearArithmeticEqualDefinitionRows(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "linear-definition-eq-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "linear-definition-eq-y"), realSort)
	astTestMust(t, err)
	first, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	second, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{y, x})
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(first, false))
	astTestMust(t, state.Internalize(second, false))
	firstNode, secondNode := state.context.enodes.get(first), state.context.enodes.get(second)
	if firstNode == nil || secondNode == nil || firstNode.Root() == secondNode.Root() {
		t.Fatal("definition rows started merged")
	}
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue || firstNode.Root() == secondNode.Root() {
		t.Fatal("equal definition row propagation", status)
	}
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if firstNode.Root() != secondNode.Root() {
		t.Fatal("queued equal definition rows were not merged")
	}
}

func TestSMTLinearArithmeticScopedColumnsAreReclaimed(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	astTestMust(t, m.IncRef(one))
	defer func() { astTestMust(t, m.DecRef(one)) }()
	baseline := theory.nextSimplex
	for i := 0; i < 8; i++ {
		astTestMust(t, state.PushScope())
		astTestMust(t, state.Internalize(one, false))
		if theory.nextSimplex <= baseline {
			t.Fatal("scoped term did not allocate a column")
		}
		astTestMust(t, state.PopScopes(1))
		if theory.nextSimplex != baseline {
			t.Fatalf("pop leaked column IDs: got %d want %d", theory.nextSimplex, baseline)
		}
	}
}
