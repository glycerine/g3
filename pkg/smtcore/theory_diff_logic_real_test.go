package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtRealDiffFixture struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	state   *SMTInternalizerState
	theory  *SMTRealDiffTheory
	sort    ast.ASTHandle
	x, y    ast.ASTHandle
}

func newSMTRealDiffFixture(t *testing.T) smtRealDiffFixture {
	t.Helper()
	manager := astTestManager(t)
	family, err := manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := manager.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := manager.MkConst(astTestSymbol(t, manager, "real-diff-x"), sort)
	astTestMust(t, err)
	y, err := manager.MkConst(astTestSymbol(t, manager, "real-diff-y"), sort)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, manager, nil)
	theory, err := NewSMTRealDiffTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	return smtRealDiffFixture{manager, family, state, theory, sort, x, y}
}

func (f smtRealDiffFixture) numeral(t *testing.T, numerator, denominator int64) ast.ASTHandle {
	t.Helper()
	numeral, err := f.manager.ArithmeticNumeralRat(big.NewRat(numerator, denominator), false)
	astTestMust(t, err)
	return numeral
}

func (f smtRealDiffFixture) difference(t *testing.T, left, right ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	minusOne := f.numeral(t, -1, 1)
	negative, err := f.manager.MkFamilyApp(f.family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, right})
	astTestMust(t, err)
	difference, err := f.manager.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{left, negative})
	astTestMust(t, err)
	return difference
}

func (f smtRealDiffFixture) bound(t *testing.T, op int32, left, right ast.ASTHandle, numerator, denominator int64) ast.ASTHandle {
	t.Helper()
	bound, err := f.manager.MkFamilyApp(f.family, op, nil, []ast.ASTHandle{f.difference(t, left, right), f.numeral(t, numerator, denominator)})
	astTestMust(t, err)
	return bound
}

func TestSMTRealDiffTheoryAtomUsesInfinitesimalComplement(t *testing.T) {
	f := newSMTRealDiffFixture(t)
	atomExpression := f.bound(t, smtArithOpLE, f.x, f.y, 1, 2)
	astTestMust(t, f.state.Internalize(atomExpression, true))
	if len(f.theory.realAtoms) != 1 {
		t.Fatal("real difference atom registration", f.theory.realAtoms)
	}
	atom := f.theory.realAtoms[0]
	positive := &f.theory.realGraph.edges[atom.positive].weight
	negative := &f.theory.realGraph.edges[atom.negative].weight
	if positive.rational.Cmp(big.NewRat(1, 2)) != 0 || positive.infinitesimal.Sign() != 0 ||
		negative.rational.Cmp(big.NewRat(-1, 2)) != 0 || negative.infinitesimal.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("real bound polarity", positive, negative)
	}
}

func TestSMTRealDiffTheoryStrictCycleConflict(t *testing.T) {
	f := newSMTRealDiffFixture(t)
	weak := f.bound(t, smtArithOpLE, f.x, f.y, 0, 1)
	strictForm := f.bound(t, smtArithOpGE, f.y, f.x, 0, 1)
	astTestMust(t, f.state.Internalize(weak, true))
	astTestMust(t, f.state.Internalize(strictForm, true))
	weakVariable, _ := f.state.BoolVar(weak)
	strictVariable, _ := f.state.BoolVar(strictForm)
	astTestMust(t, f.state.AssignLiteral(NewSMTLiteral(weakVariable, false), NewSMTBooleanAxiom()))
	// Negating y-x >= 0 gives y-x < 0. Together with x-y <= 0 it
	// creates a cycle whose rational part is zero and epsilon part is -1.
	astTestMust(t, f.state.AssignLiteral(NewSMTLiteral(strictVariable, true), NewSMTBooleanAxiom()))
	astTestMust(t, f.state.PropagateTheoryAssignments())
	astTestMust(t, f.theory.Propagate())
	conflict, found := f.state.Conflict()
	if !found || !f.state.Inconsistent() || conflict.Kind() != SMTBooleanJustificationObject {
		t.Fatal("strict real cycle conflict", found, conflict.Kind())
	}
}

func TestSMTRealDiffTheoryBuildsFiniteModelValues(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "model-rdl-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "model-rdl-y"), realSort)
	astTestMust(t, err)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	state, err := NewSMTInternalizerState(m, context, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.Close()) })
	theory, err := NewSMTRealDiffTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	fixture := smtRealDiffFixture{manager: m, family: family, state: state, theory: theory, sort: realSort, x: x, y: y}
	bound := fixture.bound(t, smtArithOpLE, x, y, 1, 2)
	astTestMust(t, state.Internalize(bound, true))
	variable, _ := state.BoolVar(bound)
	// Select the strict complement x-y > 1/2 to exercise epsilon scaling.
	astTestMust(t, state.AssignLiteral(NewSMTLiteral(variable, true), NewSMTBooleanAxiom()))
	astTestMust(t, state.PropagateTheoryAssignments())
	astTestMust(t, theory.Propagate())
	astTestMust(t, theory.realGraph.SetToZero(int32(theory.realZero)))
	astTestMust(t, context.SetRelevant(x, true))
	astTestMust(t, context.SetRelevant(y, true))
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildStateValues(state))
	xValue, err := generator.Value(context.enodes.get(x))
	astTestMust(t, err)
	yValue, err := generator.Value(context.enodes.get(y))
	astTestMust(t, err)
	xText, _, xMatched, err := m.ArithmeticNumeralText(xValue)
	astTestMust(t, err)
	yText, _, yMatched, err := m.ArithmeticNumeralText(yValue)
	astTestMust(t, err)
	xRat, xOK := new(big.Rat).SetString(xText)
	yRat, yOK := new(big.Rat).SetString(yText)
	if !xMatched || !yMatched || !xOK || !yOK || new(big.Rat).Sub(xRat, yRat).Cmp(big.NewRat(1, 2)) <= 0 {
		t.Fatal("strict real model values", xText, yText, theory.delta.String())
	}
}

func TestSMTSolverSetupUsesRealDifferenceLogic(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "solver-rdl-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "solver-rdl-y"), realSort)
	astTestMust(t, err)
	fixture := smtRealDiffFixture{manager: m, family: family, sort: realSort, x: x, y: y}
	weak := fixture.bound(t, smtArithOpLE, x, y, 0, 1)
	equivalent := fixture.bound(t, smtArithOpGE, y, x, 0, 1)
	strictOpposite, err := m.MkNot(equivalent)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_RDL", SMTSolverConfig{Params: config.Context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(weak))
	astTestMust(t, solver.Assert(strictOpposite))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("strict real difference cycle", status)
	}
}

func TestSMTSolverPublishesStrictRealDifferenceModel(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "solver-model-rdl-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "solver-model-rdl-y"), realSort)
	astTestMust(t, err)
	fixture := smtRealDiffFixture{manager: m, family: family, sort: realSort, x: x, y: y}
	bound := fixture.bound(t, smtArithOpLE, x, y, 1, 2)
	strict, err := m.MkNot(bound)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_RDL", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.SetProduceModels(true))
	astTestMust(t, solver.Assert(strict))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("strict real model status", status)
	}
	generated, err := solver.AcquireModel()
	astTestMust(t, err)
	if generated == nil {
		t.Fatal("real difference solver omitted generated model")
	}
	t.Cleanup(func() { astTestMust(t, generated.DecRef()) })
	values := make([]*big.Rat, 0, 2)
	for _, expression := range []ast.ASTHandle{x, y} {
		declaration, declarationErr := m.GetDecl(expression)
		astTestMust(t, declarationErr)
		value, valueErr := generated.ConstantInterpretation(declaration)
		astTestMust(t, valueErr)
		text, _, matched, textErr := m.ArithmeticNumeralText(value)
		astTestMust(t, textErr)
		rational, ok := new(big.Rat).SetString(text)
		if !matched || !ok {
			t.Fatal("non-rational real model value", value, text)
		}
		values = append(values, rational)
	}
	if new(big.Rat).Sub(values[0], values[1]).Cmp(big.NewRat(1, 2)) <= 0 {
		t.Fatal("strict real model does not satisfy asserted complement", values)
	}
}

func TestSMTSolverRealDifferenceEqualityAdapter(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	realSort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "solver-rdl-eq-x"), realSort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "solver-rdl-eq-y"), realSort)
	astTestMust(t, err)
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	fixture := smtRealDiffFixture{manager: m, family: family, sort: realSort, x: x, y: y}
	strict := fixture.bound(t, smtArithOpLE, x, y, -1, 3)
	solver, err := NewSMTSolver(m, nil, "QF_RDL", SMTSolverConfig{Params: config.Context})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(equality))
	astTestMust(t, solver.Assert(strict))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("real equality and strict difference", status)
	}
}
