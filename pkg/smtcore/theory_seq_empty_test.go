package smtcore

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func astTestString(t *testing.T, manager *ast.ASTManager, value string) ast.ASTHandle {
	t.Helper()
	result, err := manager.MkString(value)
	astTestMust(t, err)
	return result
}

func astTestInteger(t *testing.T, manager *ast.ASTManager, value int64) ast.ASTHandle {
	t.Helper()
	result, err := manager.ArithmeticIntegerNumeralText(strconv.FormatInt(value, 10))
	astTestMust(t, err)
	return result
}

func astTestStringConst(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	empty := astTestString(t, manager, "")
	sort, err := manager.GetSort(empty)
	astTestMust(t, err)
	result, err := manager.MkConst(astTestSymbol(t, manager, name), sort)
	astTestMust(t, err)
	return result
}

func TestSMTSequenceStructuredModelValues(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-model-x"), sort)
	astTestMust(t, err)
	concat, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	context, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(concat, false))
	astTestMust(t, context.SetRelevant(x, true))
	astTestMust(t, context.SetRelevant(concat, true))
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, generator.BuildStateValues(state))
	xValue, err := generator.Value(context.enodes.get(x))
	astTestMust(t, err)
	concatValue, err := generator.Value(context.enodes.get(concat))
	astTestMust(t, err)
	xText, xString, err := m.SequenceStringValue(xValue)
	astTestMust(t, err)
	concatText, concatString, err := m.SequenceStringValue(concatValue)
	astTestMust(t, err)
	if !xString || !concatString || concatText != xText+"a" {
		t.Fatal("structured concat model value", xText, concatText)
	}

	byteValue, err := m.BitVectorNumeralUnsigned('A', 8)
	astTestMust(t, err)
	integerValue, err := m.ArithmeticNumeralRat(big.NewRat(12, 1), true)
	astTestMust(t, err)
	procedure := &smtSequenceValueProcedure{
		sort:         sort,
		sources:      []smtSequenceModelSource{{kind: smtSequenceModelUnit}, {kind: smtSequenceModelInteger}},
		dependencies: make([]SMTModelValueDependency, 2),
	}
	value, err := procedure.MakeValue(generator, []ast.ASTHandle{byteValue, integerValue})
	astTestMust(t, err)
	if text, matched, textErr := m.SequenceStringValue(value); textErr != nil || !matched || text != "A12" {
		t.Fatal("unit and integer model sources", text, matched, textErr)
	}
}

func TestSMTEmptySequenceTheoryScopesAndModelFactory(t *testing.T) {
	m := astTestManager(t)
	_, err := m.MkString("")
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTEmptySequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	if theory.Name() != "seq-empty" || theory.UseDisequalities() {
		t.Fatal("empty sequence theory metadata", theory.Name(), theory.UseDisequalities())
	}
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("unused sequence theory", status)
	}
	astTestMust(t, state.PushScope())
	if internalized, e := theory.InternalizeTerm(ast.ASTHandle{}); e != nil || internalized {
		t.Fatal("empty sequence term", internalized, e)
	}
	status, err = theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckGiveUp {
		t.Fatal("used sequence theory", status)
	}
	astTestMust(t, state.PopScopes(1))
	status, err = theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckDone {
		t.Fatal("sequence usage did not roll back", status)
	}
	fresh, err := theory.Fresh()
	astTestMust(t, err)
	if fresh == theory || fresh.FamilyID() != family {
		t.Fatal("fresh sequence theory", fresh)
	}
}

func TestSMTSequenceLogicsInstallSafeCheckpointTheory(t *testing.T) {
	for _, logic := range []string{"QF_BVRE", "QF_S", "ALL"} {
		t.Run(logic, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			value, err := m.MkString("a")
			astTestMust(t, err)
			sort, err := m.GetSort(value)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-checkpoint-"+logic), sort)
			astTestMust(t, err)
			equality, err := m.MkEq(x, value)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, logic, SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			family, err := m.SequenceFamilyID()
			astTestMust(t, err)
			if logic == "QF_S" {
				if _, ok := solver.kernel.internalizer.theories[family].(*SMTStringTheory); !ok {
					t.Fatal("native QF_S String theory was not installed")
				}
			} else if _, ok := solver.kernel.internalizer.theories[family].(*SMTSequenceTheory); !ok {
				t.Fatal("sequence theory was not installed")
			}
			astTestMust(t, solver.Assert(equality))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal("sequence equality fragment did not return sat", status)
			}
		})
	}
}

func TestSMTSequenceTheoryLiteralConflictAndConcatModel(t *testing.T) {
	for _, logic := range []string{"QF_S", "ALL"} {
		t.Run(logic, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			a, err := m.MkString("a")
			astTestMust(t, err)
			b, err := m.MkString("b")
			astTestMust(t, err)
			equality, err := m.MkEq(a, b)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, logic, SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(equality))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("distinct sequence literals", status)
			}
		})
	}

	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-symbolic-part"), sort)
	astTestMust(t, err)
	concat, err := m.MkSequenceConcat(y, a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-unsupported"), sort)
	astTestMust(t, err)
	equality, err := m.MkEq(x, concat)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(equality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("solved symbolic concatenation model", status)
	}
}

func TestSMTSequenceTheoryAuditsUnresolvedWordEquation(t *testing.T) {
	m := astTestManager(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-commute-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-commute-y"), sort)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, theory.equations.Push(0, []ast.ASTHandle{x, y}, []ast.ASTHandle{y, x}, nil))
	unresolved, err := theory.hasUnresolvedWordEquation()
	astTestMust(t, err)
	if !unresolved {
		t.Fatal("commuting word equation reported solved")
	}
}

func TestSMTSequenceTheoryPropagatesTailEmptyMaximumLength(t *testing.T) {
	m := astTestManager(t)
	emptyString, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(emptyString)
	astTestMust(t, err)
	source, err := m.MkConst(astTestSymbol(t, m, "sequence-tail-bound-source"), sort)
	astTestMust(t, err)
	position, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	tail, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{source, position}, sort)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	progress, err := theory.addTailEmptyMaximum([]ast.ASTHandle{tail}, nil)
	astTestMust(t, err)
	if !progress {
		t.Fatal("tail-empty maximum-length lemma not added")
	}
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	tailEmpty, err := m.MkEq(tail, empty)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(source)
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	atMostThree, err := m.MkArithmeticLE(length, three)
	astTestMust(t, err)
	want, err := m.MkImplies(tailEmpty, atMostThree)
	astTestMust(t, err)
	got, err := theory.axioms.Front()
	astTestMust(t, err)
	if got != want {
		t.Fatal("tail-empty maximum-length lemma", got, want)
	}
	progress, err = theory.addTailEmptyMaximum(nil, []ast.ASTHandle{tail})
	astTestMust(t, err)
	if progress {
		t.Fatal("duplicate tail-empty maximum-length lemma")
	}

	m2, modelConfig := smtModelGeneratorFixture(t)
	emptyString, err = m2.MkString("")
	astTestMust(t, err)
	sort, err = m2.GetSort(emptyString)
	astTestMust(t, err)
	source, err = m2.MkConst(astTestSymbol(t, m2, "sequence-tail-bound-integration-source"), sort)
	astTestMust(t, err)
	position, err = m2.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	tailName, err = m2.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	tail, err = m2.MkSequenceSkolem(tailName, []ast.ASTHandle{source, position}, sort)
	astTestMust(t, err)
	empty, err = m2.MkSequenceEmpty(sort)
	astTestMust(t, err)
	tailEmpty, err = m2.MkEq(tail, empty)
	astTestMust(t, err)
	length, err = m2.MkSequenceLength(source)
	astTestMust(t, err)
	four, err := m2.ArithmeticIntegerNumeralText("4")
	astTestMust(t, err)
	atLeastFour, err := m2.MkArithmeticGE(length, four)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m2, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(tailEmpty))
	astTestMust(t, solver.Assert(atLeastFour))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("empty tail failed to bound source length", status)
	}
}

func TestSMTSequenceTheoryConcreteWordEquationConflict(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	ba, err := m.MkString("ba")
	astTestMust(t, err)
	c, err := m.MkString("c")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-word-x"), sort)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(word, ba)
	astTestMust(t, err)
	valueEquality, err := m.MkEq(x, c)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	astTestMust(t, solver.Assert(valueEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("concrete sequence word conflict", status)
	}
}

func TestSMTSequenceTheoryConcreteWordDisequationConflict(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	ba, err := m.MkString("ba")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-diseq-x"), sort)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(word, ba)
	astTestMust(t, err)
	wordDisequality, err := m.MkNot(wordEquality)
	astTestMust(t, err)
	valueEquality, err := m.MkEq(x, b)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordDisequality))
	astTestMust(t, solver.Assert(valueEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("concrete sequence word disequality conflict", status)
	}
}

func TestSMTSequenceTheoryReducedWordDisequationConflict(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	ab, err := m.MkString("ab")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-reduced-diseq-x"), sort)
	astTestMust(t, err)
	xa, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(xa, b)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(x, ab)
	astTestMust(t, err)
	equality, err := m.MkEq(left, right)
	astTestMust(t, err)
	disequality, err := m.MkNot(equality)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(disequality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("associatively equal words satisfied a disequality", status)
	}
}

func TestSMTSequenceTheoryPropagatesResidualWordDisequality(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	ab, err := m.MkString("ab")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-residual-diseq-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-residual-diseq-y"), sort)
	astTestMust(t, err)
	xa, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(xa, b)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(y, ab)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	wordDisequality, err := m.MkNot(wordEquality)
	astTestMust(t, err)
	variablesEqual, err := m.MkEq(x, y)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordDisequality))
	astTestMust(t, solver.Assert(variablesEqual))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("residual symbolic disequality was not propagated", status)
	}
}

func TestSMTSequenceTheoryConcreteNonContainsConflict(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	abc, err := m.MkString("abc")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(abc)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-contains-x"), sort)
	astTestMust(t, err)
	contains, err := m.MkSequenceContains(x, b)
	astTestMust(t, err)
	notContains, err := m.MkNot(contains)
	astTestMust(t, err)
	valueEquality, err := m.MkEq(x, abc)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notContains))
	astTestMust(t, solver.Assert(valueEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("concrete non-contains conflict", status)
	}
}

func TestSMTSequenceTheorySymbolicNonContainsGivesUp(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	literal, err := m.MkString("z")
	astTestMust(t, err)
	sort, err := m.GetSort(literal)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-unresolved-noncontains-x"), sort)
	astTestMust(t, err)
	contains, err := m.MkSequenceContains(x, literal)
	astTestMust(t, err)
	notContains, err := m.MkNot(contains)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notContains))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("unresolved symbolic negative containment", status, solver.kernel.lastFailure)
	}
}

func TestSMTSequenceTheoryCancelsWordBoundaryAndPropagatesSolution(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	ab, err := m.MkString("ab")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-cancel-x"), sort)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(a, x)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(word, ab)
	astTestMust(t, err)
	xEqualsB, err := m.MkEq(x, b)
	astTestMust(t, err)
	xNotB, err := m.MkNot(xEqualsB)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	astTestMust(t, solver.Assert(xNotB))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("cancelled word solution did not conflict", status)
	}
}

func TestSMTSequenceTheoryCancelsEquivalentEnodeBoundaries(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-enode-cancel-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-enode-cancel-y"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(y, b)
	astTestMust(t, err)
	variablesEqual, err := m.MkEq(x, y)
	astTestMust(t, err)
	wordsEqual, err := m.MkEq(left, right)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(variablesEqual))
	astTestMust(t, solver.Assert(wordsEqual))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("equivalent e-node boundary cancellation", status)
	}
}

func TestSMTSequenceTheoryReducesEqualLengthBoundaries(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-length-cancel-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-length-cancel-y"), sort)
	astTestMust(t, err)
	xLength, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	yLength, err := m.MkSequenceLength(y)
	astTestMust(t, err)
	lengthsEqual, err := m.MkEq(xLength, yLength)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(y, b)
	astTestMust(t, err)
	wordsEqual, err := m.MkEq(left, right)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lengthsEqual))
	astTestMust(t, solver.Assert(wordsEqual))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("equal-length boundary reduction", status)
	}
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, ok := solver.kernel.internalizer.theories[family].(*SMTSequenceTheory)
	if !ok || !theory.solutions.IsRoot(x) || !theory.solutions.IsRoot(y) {
		t.Fatal("equal-length reduction installed a word substitution")
	}
}

func TestSMTSequenceTheorySplitsFixedLengthPrefixes(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-length-prefix-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-length-prefix-y"), sort)
	astTestMust(t, err)
	z, err := m.MkConst(astTestSymbol(t, m, "sequence-length-prefix-z"), sort)
	astTestMust(t, err)
	yz, err := m.MkSequenceConcat(y, z)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	rightTail, err := m.MkSequenceConcat(z, b)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(y, rightTail)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	prefixEquality, err := m.MkEq(x, yz)
	astTestMust(t, err)
	notPrefixEquality, err := m.MkNot(prefixEquality)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, fixed := range []struct {
		expression ast.ASTHandle
		value      ast.ASTHandle
	}{{x, two}, {y, one}, {z, one}} {
		length, makeErr := m.MkSequenceLength(fixed.expression)
		astTestMust(t, makeErr)
		equality, makeErr := m.MkEq(length, fixed.value)
		astTestMust(t, makeErr)
		astTestMust(t, solver.Assert(equality))
	}
	astTestMust(t, solver.Assert(wordEquality))
	astTestMust(t, solver.Assert(notPrefixEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed-length prefix split", status)
	}
}

func TestSMTSequenceTheoryRejectsFixedLengthWordMismatch(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-mismatch-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-mismatch-y"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(y, a)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, fixed := range []struct {
		expression ast.ASTHandle
		value      ast.ASTHandle
	}{{x, one}, {y, two}} {
		length, makeErr := m.MkSequenceLength(fixed.expression)
		astTestMust(t, makeErr)
		equality, makeErr := m.MkEq(length, fixed.value)
		astTestMust(t, makeErr)
		astTestMust(t, solver.Assert(equality))
	}
	astTestMust(t, solver.Assert(wordEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed-length word mismatch", status)
	}
}

func TestSMTSequenceTheoryBranchesLeadingVariableOverlap(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-leading-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-leading-y"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(y, b)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	xLength, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	yLength, err := m.MkSequenceLength(y)
	astTestMust(t, err)
	lengthOrder, err := m.MkArithmeticLE(xLength, yLength)
	astTestMust(t, err)
	leftName, err := m.Symbols.Intern("seq.left")
	astTestMust(t, err)
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	shared, err := m.MkSequenceSkolem(leftName, []ast.ASTHandle{x, empty, y}, sort)
	astTestMust(t, err)
	xDefinition, err := m.MkEq(x, shared)
	astTestMust(t, err)
	notXDefinition, err := m.MkNot(xDefinition)
	astTestMust(t, err)
	// Keep the source formulas used again by the second solver alive even if
	// the first solver's preprocessing replaces them.
	for _, formula := range []ast.ASTHandle{wordEquality, lengthOrder, xDefinition} {
		astTestPin(t, m, formula)
	}
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	astTestMust(t, solver.Assert(lengthOrder))
	astTestMust(t, solver.Assert(notXDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("leading variable overlap branch", status)
	}
	rightName, err := m.Symbols.Intern("seq.right")
	astTestMust(t, err)
	remainder, err := m.MkSequenceSkolem(rightName, []ast.ASTHandle{x, empty, y}, sort)
	astTestMust(t, err)
	yWord, err := m.MkSequenceConcat(shared, remainder)
	astTestMust(t, err)
	yDefinition, err := m.MkEq(y, yWord)
	astTestMust(t, err)
	notYDefinition, err := m.MkNot(yDefinition)
	astTestMust(t, err)
	secondSolver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, secondSolver.Close()) })
	astTestMust(t, secondSolver.Assert(wordEquality))
	astTestMust(t, secondSolver.Assert(lengthOrder))
	astTestMust(t, secondSolver.Assert(xDefinition))
	astTestMust(t, secondSolver.Assert(notYDefinition))
	status, err = secondSolver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("leading variable overlap remainder branch", status)
	}
}

func TestSMTSequenceTheoryBranchesPrefixedVariableOverlap(t *testing.T) {
	m, _ := smtModelGeneratorFixture(t)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integerSort, err := m.GetSort(one)
	astTestMust(t, err)
	prefixInteger, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-prefix-integer"), integerSort)
	astTestMust(t, err)
	prefix, err := m.MkIntToString(prefixInteger)
	astTestMust(t, err)
	sort, err := m.GetSort(prefix)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-prefixed-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-prefixed-y"), sort)
	astTestMust(t, err)
	leftTail, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-prefixed-left-tail"), sort)
	astTestMust(t, err)
	rightTail, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-prefixed-right-tail"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, leftTail)
	astTestMust(t, err)
	yAndTail, err := m.MkSequenceConcat(y, rightTail)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(prefix, yAndTail)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	xLength, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	prefixLength, err := m.MkSequenceLength(prefix)
	astTestMust(t, err)
	// theory_seq compares lengths with mk_simplified_literal(mk_le(mk_sub(a, b), 0)).
	xAtMostPrefix, err := theory.seqCompareLE(xLength, prefixLength)
	astTestMust(t, err)
	xAbovePrefix, err := m.MkNot(xAtMostPrefix)
	astTestMust(t, err)
	yLength, err := m.MkSequenceLength(y)
	astTestMust(t, err)
	prefixAndYLength, err := m.MkArithmeticAdd(prefixLength, yLength)
	astTestMust(t, err)
	xWithinY, err := theory.seqCompareLE(xLength, prefixAndYLength)
	astTestMust(t, err)
	leftName, err := m.Symbols.Intern("seq.left")
	astTestMust(t, err)
	arguments := []ast.ASTHandle{x, prefix, y}
	shared, err := m.MkSequenceSkolem(leftName, arguments, sort)
	astTestMust(t, err)
	prefixAndShared, err := m.MkSequenceConcat(prefix, shared)
	astTestMust(t, err)
	xDefinition, err := m.MkEq(x, prefixAndShared)
	astTestMust(t, err)
	rightName, err := m.Symbols.Intern("seq.right")
	astTestMust(t, err)
	remainder, err := m.MkSequenceSkolem(rightName, arguments, sort)
	astTestMust(t, err)
	yValue, err := m.MkSequenceConcat(shared, remainder)
	astTestMust(t, err)
	yDefinition, err := m.MkEq(y, yValue)
	astTestMust(t, err)
	definitions, err := m.MkAnd(xDefinition, yDefinition)
	astTestMust(t, err)
	antecedent, err := m.MkAndArray([]ast.ASTHandle{wordEquality, xAbovePrefix, xWithinY})
	astTestMust(t, err)
	expected, err := m.MkImplies(antecedent, definitions)
	astTestMust(t, err)
	branched, err := theory.branchLeadingVariableOverlap(
		[]ast.ASTHandle{x, leftTail}, []ast.ASTHandle{prefix, y, rightTail})
	astTestMust(t, err)
	if !branched {
		t.Fatal("prefixed variable overlap did not branch")
	}
	actual, err := theory.axioms.Front()
	astTestMust(t, err)
	if actual != expected {
		t.Fatal("prefixed variable overlap axiom", actual, expected)
	}
	branched, err = theory.branchLeadingVariableOverlap(
		[]ast.ASTHandle{x, leftTail}, []ast.ASTHandle{prefix, y, rightTail})
	astTestMust(t, err)
	if !branched {
		t.Fatal("prefixed overlap did not advance to the next variable candidate")
	}
}

func TestSMTSequenceTheoryBranchesVariablePrefixEqualities(t *testing.T) {
	m, _ := smtModelGeneratorFixture(t)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integerSort, err := m.GetSort(one)
	astTestMust(t, err)
	n, err := m.MkConst(astTestSymbol(t, m, "sequence-branch-prefix-integer"), integerSort)
	astTestMust(t, err)
	prefix, err := m.MkIntToString(n)
	astTestMust(t, err)
	sort, err := m.GetSort(prefix)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-branch-prefix-x"), sort)
	astTestMust(t, err)
	leftTail, err := m.MkConst(astTestSymbol(t, m, "sequence-branch-prefix-left-tail"), sort)
	astTestMust(t, err)
	rightTail, err := m.MkConst(astTestSymbol(t, m, "sequence-branch-prefix-right-tail"), sort)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	branched, err := theory.branchVariablePrefixEquality(1,
		[]ast.ASTHandle{x, leftTail}, []ast.ASTHandle{prefix, rightTail}, nil)
	astTestMust(t, err)
	if !branched {
		t.Fatal("variable did not branch to empty")
	}
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	emptyEquality, err := m.MkEq(x, empty)
	astTestMust(t, err)
	emptyLiteral, err := state.Literal(emptyEquality)
	astTestMust(t, err)
	emptyData, err := state.BooleanData(emptyLiteral.Var())
	astTestMust(t, err)
	if !emptyData.TryTrueFirst() || !state.relevancy.IsRelevant(emptyEquality) {
		t.Fatal("empty equality lacks native branch preference")
	}
	astTestMust(t, state.assignDecisionLiteral(emptyLiteral.Negated()))
	branched, err = theory.branchVariablePrefixEquality(1,
		[]ast.ASTHandle{x, leftTail}, []ast.ASTHandle{prefix, rightTail}, nil)
	astTestMust(t, err)
	if !branched {
		t.Fatal("variable did not advance to the first word prefix")
	}
	prefixEquality, err := m.MkEq(prefix, x)
	astTestMust(t, err)
	prefixLiteral, err := state.Literal(prefixEquality)
	astTestMust(t, err)
	prefixData, err := state.BooleanData(prefixLiteral.Var())
	astTestMust(t, err)
	if !prefixData.TryTrueFirst() || !state.relevancy.IsRelevant(prefixEquality) {
		t.Fatal("word-prefix equality lacks native branch preference")
	}
}

func TestSMTSequenceTheorySkipsInfeasibleVariablePrefixes(t *testing.T) {
	m, _ := smtModelGeneratorFixture(t)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	integerSort, err := m.GetSort(one)
	astTestMust(t, err)
	n, err := m.MkConst(astTestSymbol(t, m, "sequence-feasible-prefix-integer"), integerSort)
	astTestMust(t, err)
	prefix, err := m.MkIntToString(n)
	astTestMust(t, err)
	sort, err := m.GetSort(prefix)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-feasible-prefix-x"), sort)
	astTestMust(t, err)
	element, err := m.BitVectorNumeralUnsigned('a', 8)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(element)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	branched, err := theory.branchVariablePrefixEquality(1,
		[]ast.ASTHandle{x}, []ast.ASTHandle{prefix, unit}, nil)
	astTestMust(t, err)
	if !branched {
		t.Fatal("variable did not reach feasible complete prefix")
	}
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	emptyEquality, err := m.MkEq(x, empty)
	astTestMust(t, err)
	if _, internalized := state.BoolVar(emptyEquality); internalized {
		t.Fatal("infeasible empty prefix was internalized")
	}
	word, err := m.MkSequenceConcat(prefix, unit)
	astTestMust(t, err)
	prefixEquality, err := m.MkEq(x, word)
	astTestMust(t, err)
	literal, err := state.Literal(prefixEquality)
	astTestMust(t, err)
	data, err := state.BooleanData(literal.Var())
	astTestMust(t, err)
	if !data.TryTrueFirst() || !state.relevancy.IsRelevant(prefixEquality) {
		t.Fatal("feasible complete prefix lacks branch preference")
	}
}

func TestSMTSequenceTheoryClosesExhaustedUnitPrefixBranches(t *testing.T) {
	m, _ := smtModelGeneratorFixture(t)
	a, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	b, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	unitA, err := m.MkSequenceUnit(a)
	astTestMust(t, err)
	unitB, err := m.MkSequenceUnit(b)
	astTestMust(t, err)
	sort, err := m.GetSort(unitA)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-exhausted-prefix-x"), sort)
	astTestMust(t, err)
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(unitA, unitB)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	for _, candidate := range []ast.ASTHandle{empty, word} {
		equality, equalityErr := m.MkEq(x, candidate)
		astTestMust(t, equalityErr)
		astTestMust(t, state.Internalize(equality, false))
		literal, literalErr := state.Literal(equality)
		astTestMust(t, literalErr)
		astTestMust(t, state.assignDecisionLiteral(literal.Negated()))
	}
	branched, err := theory.branchVariablePrefixEquality(1,
		[]ast.ASTHandle{x}, []ast.ASTHandle{unitA, unitB}, nil)
	astTestMust(t, err)
	if !branched || !state.Inconsistent() {
		t.Fatal("exhausted finite unit prefixes did not close the branch", branched, state.Inconsistent())
	}
}

func TestSMTSequenceTheoryLengthCoherenceDecomposition(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sort is not a sequence")
	}
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-coherence-x"), sort)
	astTestMust(t, err)
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	xEmpty, err := m.MkEq(x, empty)
	astTestMust(t, err)
	xNonempty, err := m.MkNot(xEmpty)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	headName, err := m.Symbols.Intern("seq.nth")
	astTestMust(t, err)
	head, err := m.MkSequenceSkolem(headName, []ast.ASTHandle{x, zero}, elementSort)
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	tail, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{x, zero}, sort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(head)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(unit, tail)
	astTestMust(t, err)
	decomposition, err := m.MkEq(x, word)
	astTestMust(t, err)
	notDecomposition, err := m.MkNot(decomposition)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(xNonempty))
	astTestMust(t, solver.Assert(notDecomposition))
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	one := astTestInteger(t, m, 1)
	lowerBound, err := m.MkArithmeticGE(length, one)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(lowerBound))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("nonempty sequence lacks head-tail decomposition", status)
	}
}

func TestSMTSequenceTheoryExposesExtensionalityEquality(t *testing.T) {
	m := astTestManager(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-extensionality-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-extensionality-y"), sort)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	context, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(x, false))
	astTestMust(t, state.Internalize(y, false))
	smtSequenceInterfaceFixture{m: m, theory: theory, state: state, sort: sort}.parent(t, y)
	astTestMust(t, context.SetRelevant(x, true))
	astTestMust(t, context.SetRelevant(y, true))
	status, err := theory.FinalCheck()
	astTestMust(t, err)
	if status != SMTFinalCheckContinue {
		t.Fatal("extensionality did not request an equality split", status)
	}
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	if _, found := state.BoolVar(equality); !found || !context.relevant.get(equality) {
		t.Fatal("extensionality equality is not a relevant Boolean atom", found, context.relevant.get(equality))
	}
}

func TestSMTSequenceTheoryFixedLengthOneDecomposition(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sort is not a sequence")
	}
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-one-x"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	lengthOne, err := m.MkEq(length, one)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	headName, err := m.Symbols.Intern("seq.nth")
	astTestMust(t, err)
	head, err := m.MkSequenceSkolem(headName, []ast.ASTHandle{x, zero}, elementSort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(head)
	astTestMust(t, err)
	singleton, err := m.MkEq(x, unit)
	astTestMust(t, err)
	notSingleton, err := m.MkNot(singleton)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lengthOne))
	astTestMust(t, solver.Assert(notSingleton))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed length one sequence is not a singleton", status)
	}
}

func TestSMTSequenceTheoryFixedLengthDecomposition(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sort is not a sequence")
	}
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-length-x"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	lowerBound, err := m.MkArithmeticGE(length, two)
	astTestMust(t, err)
	upperBound, err := m.MkArithmeticLE(length, two)
	astTestMust(t, err)
	nthName, err := m.Symbols.Intern("seq.nth")
	astTestMust(t, err)
	parts := make([]ast.ASTHandle, 0, 2)
	for index := 0; index < 2; index++ {
		position, makeErr := m.ArithmeticIntegerNumeralText(strconv.Itoa(index))
		astTestMust(t, makeErr)
		element, makeErr := m.MkSequenceSkolem(nthName, []ast.ASTHandle{x, position}, elementSort)
		astTestMust(t, makeErr)
		unit, makeErr := m.MkSequenceUnit(element)
		astTestMust(t, makeErr)
		parts = append(parts, unit)
	}
	word, err := m.MkSequenceConcat(parts[0], parts[1])
	astTestMust(t, err)
	definition, err := m.MkEq(x, word)
	astTestMust(t, err)
	notDefinition, err := m.MkNot(definition)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lowerBound))
	astTestMust(t, solver.Assert(upperBound))
	astTestMust(t, solver.Assert(notDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed-length sequence lacks exact nth decomposition", status)
	}
}

func TestSMTSequenceTheoryLengthLowerBoundDecomposition(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sort is not a sequence")
	}
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-lower-bound-x"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	lowerBound, err := m.MkArithmeticGE(length, two)
	astTestMust(t, err)
	nthName, err := m.Symbols.Intern("seq.nth")
	astTestMust(t, err)
	parts := make([]ast.ASTHandle, 0, 3)
	for index := 0; index < 2; index++ {
		position, makeErr := m.ArithmeticIntegerNumeralText(strconv.Itoa(index))
		astTestMust(t, makeErr)
		element, makeErr := m.MkSequenceSkolem(nthName, []ast.ASTHandle{x, position}, elementSort)
		astTestMust(t, makeErr)
		unit, makeErr := m.MkSequenceUnit(element)
		astTestMust(t, makeErr)
		parts = append(parts, unit)
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	remainder, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{x, one}, sort)
	astTestMust(t, err)
	parts = append(parts, remainder)
	tail, err := m.MkSequenceConcat(parts[1], parts[2])
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(parts[0], tail)
	astTestMust(t, err)
	definition, err := m.MkEq(x, word)
	astTestMust(t, err)
	notDefinition, err := m.MkNot(definition)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lowerBound))
	astTestMust(t, solver.Assert(notDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("length lower bound lacks required prefix decomposition", status)
	}
}

func TestSMTSequenceTheoryBranchesUnboundedLengthAtLowerBound(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-unbounded-lower-branch-x"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	lowerBound, err := m.MkArithmeticGE(length, two)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lowerBound))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, _ := solver.ReasonUnknown()
		t.Fatal("unbounded lower-length model", status, reason)
	}
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, ok := solver.kernel.internalizer.theories[family].(*SMTSequenceTheory)
	if !ok {
		t.Fatal("sequence theory was not installed")
	}
	_, err = theory.addLengthLowerBoundDecomposition()
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	remainder, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{x, one}, sort)
	astTestMust(t, err)
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	minimalLiteral := theory.equalityLiteral(remainder, empty)
	if minimalLiteral == SMTNullLiteral {
		t.Fatal("unbounded remainder did not expose the native minimum-length branch")
	}
	data, err := solver.kernel.internalizer.BooleanData(minimalLiteral.Var())
	astTestMust(t, err)
	if !data.TryTrueFirst() {
		t.Fatal("minimum-length remainder branch lacks true-first preference")
	}
}

func TestSMTSequenceTheoryLengthBoundsConstrainRemainder(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-bounded-remainder-x"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	lowerBound, err := m.MkArithmeticGE(length, two)
	astTestMust(t, err)
	upperBound, err := m.MkArithmeticLE(length, three)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	remainder, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{x, one}, sort)
	astTestMust(t, err)
	remainderLength, err := m.MkSequenceLength(remainder)
	astTestMust(t, err)
	remainderTooLong, err := m.MkArithmeticGE(remainderLength, two)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lowerBound))
	astTestMust(t, solver.Assert(upperBound))
	astTestMust(t, solver.Assert(remainderTooLong))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("sequence upper bound did not constrain the exposed remainder", status)
	}
}

func TestSMTSequenceTheoryNativeWordVariableClassification(t *testing.T) {
	m := astTestManager(t)
	literal, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(literal)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-variable-classification-x"), sort)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	extract, err := m.MkSequenceExtract(x, zero, one)
	astTestMust(t, err)
	at, err := m.MkSequenceAt(x, zero)
	astTestMust(t, err)
	replace, err := m.MkSequenceReplace(x, literal, x)
	astTestMust(t, err)
	concat, err := m.MkSequenceConcat(x, literal)
	astTestMust(t, err)
	converted, err := m.MkIntToString(one)
	astTestMust(t, err)
	skolemName, err := m.Symbols.Intern("sequence.variable.classification.skolem")
	astTestMust(t, err)
	skolem, err := m.MkSequenceSkolem(skolemName, []ast.ASTHandle{x}, sort)
	astTestMust(t, err)
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	character, err := m.BitVectorNumeralUnsigned('a', 8)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(character)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	condition, err := m.MkConst(astTestSymbol(t, m, "sequence-variable-condition"), boolean)
	astTestMust(t, err)
	conditional, err := m.MkIte(condition, x, literal)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	for _, test := range []struct {
		name       string
		expression ast.ASTHandle
		want       bool
	}{
		{name: "uninterpreted", expression: x, want: true},
		{name: "extract", expression: extract, want: true},
		{name: "at", expression: at, want: true},
		{name: "replace", expression: replace, want: true},
		{name: "internal-skolem", expression: skolem, want: true},
		{name: "empty", expression: empty},
		{name: "unit", expression: unit},
		{name: "ite", expression: conditional},
		{name: "scalar", expression: one},
		{name: "literal", expression: literal},
		{name: "concat", expression: concat},
		{name: "int-to-string", expression: converted},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, variableErr := theory.sequenceVariable(test.expression)
			astTestMust(t, variableErr)
			if got != test.want {
				t.Fatal("word-variable classification", got, test.want)
			}
		})
	}
}

func TestSMTSequenceTheoryBranchesBinaryUnitEquation(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-binary-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-binary-y"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(b, y)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.binary.tail")
	astTestMust(t, err)
	tail, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{x, y, a, b}, sort)
	astTestMust(t, err)
	xValue, err := m.MkSequenceConcat(b, tail)
	astTestMust(t, err)
	xEquality, err := m.MkEq(x, xValue)
	astTestMust(t, err)
	notXEquality, err := m.MkNot(xEquality)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	astTestMust(t, solver.Assert(notXEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("binary unit equation branch", status)
	}
}

func TestSMTSequenceTheoryRejectsIncompatibleCyclicUnits(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	b, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	unitA, err := m.MkSequenceUnit(a)
	astTestMust(t, err)
	unitB, err := m.MkSequenceUnit(b)
	astTestMust(t, err)
	sequenceSort, err := m.GetSort(unitA)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-cyclic-x"), sequenceSort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, unitA)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(unitB, x)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("incompatible cyclic sequence units", status)
	}
}

func TestSMTSequenceTheoryBranchesVariableAcrossFixedUnitWord(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	truth, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	falsehood, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	first, err := m.MkSequenceUnit(truth)
	astTestMust(t, err)
	second, err := m.MkSequenceUnit(falsehood)
	astTestMust(t, err)
	sort, err := m.GetSort(first)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-unit-split-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-unit-split-y"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, y)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(first, second)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	parts := []ast.ASTHandle{first, second}
	word := func(parts []ast.ASTHandle) (ast.ASTHandle, error) {
		if len(parts) == 0 {
			return m.MkSequenceEmpty(sort)
		}
		result := parts[len(parts)-1]
		var makeErr error
		for index := len(parts) - 2; index >= 0; index-- {
			result, makeErr = m.MkSequenceConcat(parts[index], result)
			if makeErr != nil {
				return ast.ASTHandle{}, makeErr
			}
		}
		return result, nil
	}
	for split := 0; split <= len(parts); split++ {
		prefix, makeErr := word(parts[:split])
		astTestMust(t, makeErr)
		suffix, makeErr := word(parts[split:])
		astTestMust(t, makeErr)
		xEquality, makeErr := m.MkEq(x, prefix)
		astTestMust(t, makeErr)
		yEquality, makeErr := m.MkEq(y, suffix)
		astTestMust(t, makeErr)
		alternative, makeErr := m.MkAnd(xEquality, yEquality)
		astTestMust(t, makeErr)
		excluded, makeErr := m.MkNot(alternative)
		astTestMust(t, makeErr)
		astTestMust(t, solver.Assert(excluded))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed unit-word variable splits", status)
	}
}

func TestSMTSequenceTheoryBranchesFixedStringOverlaps(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("abc")
	astTestMust(t, err)
	b, err := m.MkString("ab")
	astTestMust(t, err)
	empty, err := m.MkString("")
	astTestMust(t, err)
	c, err := m.MkString("c")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-overlap-y"), sort)
	astTestMust(t, err)
	left, err := m.MkSequenceConcat(x, a)
	astTestMust(t, err)
	right, err := m.MkSequenceConcat(b, y)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(left, right)
	astTestMust(t, err)
	shortLeft, err := m.MkEq(x, empty)
	astTestMust(t, err)
	shortRight, err := m.MkEq(y, c)
	astTestMust(t, err)
	shortAlternative, err := m.MkAnd(shortLeft, shortRight)
	astTestMust(t, err)
	notShort, err := m.MkNot(shortAlternative)
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.binary.tail")
	astTestMust(t, err)
	tail, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{x, y, a, b}, sort)
	astTestMust(t, err)
	longX, err := m.MkSequenceConcat(b, tail)
	astTestMust(t, err)
	longY, err := m.MkSequenceConcat(tail, a)
	astTestMust(t, err)
	longLeft, err := m.MkEq(x, longX)
	astTestMust(t, err)
	longRight, err := m.MkEq(y, longY)
	astTestMust(t, err)
	longAlternative, err := m.MkAnd(longLeft, longRight)
	astTestMust(t, err)
	notLong, err := m.MkNot(longAlternative)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	astTestMust(t, solver.Assert(notShort))
	astTestMust(t, solver.Assert(notLong))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed string overlap alternatives", status)
	}
}

func TestSMTSequenceTheoryBranchesTwoVariablesAcrossFixedWord(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	fixed, err := m.MkString("ab")
	astTestMust(t, err)
	sort, err := m.GetSort(fixed)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-split-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-split-y"), sort)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(x, y)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(word, fixed)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	for split := 0; split <= 2; split++ {
		prefix, makeErr := m.MkString("ab"[:split])
		astTestMust(t, makeErr)
		suffix, makeErr := m.MkString("ab"[split:])
		astTestMust(t, makeErr)
		xEquality, makeErr := m.MkEq(x, prefix)
		astTestMust(t, makeErr)
		yEquality, makeErr := m.MkEq(y, suffix)
		astTestMust(t, makeErr)
		alternative, makeErr := m.MkAnd(xEquality, yEquality)
		astTestMust(t, makeErr)
		excluded, makeErr := m.MkNot(alternative)
		astTestMust(t, makeErr)
		astTestMust(t, solver.Assert(excluded))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed word split alternatives were incomplete", status)
	}
}

func TestSMTSequenceTheoryBranchesVariablePrefixAcrossFixedWord(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	fixed, err := m.MkString("ab")
	astTestMust(t, err)
	sort, err := m.GetSort(fixed)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-prefix-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-prefix-y"), sort)
	astTestMust(t, err)
	z, err := m.MkConst(astTestSymbol(t, m, "sequence-fixed-prefix-z"), sort)
	astTestMust(t, err)
	rest, err := m.MkSequenceConcat(y, z)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(x, rest)
	astTestMust(t, err)
	wordEquality, err := m.MkEq(word, fixed)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(wordEquality))
	for split := 0; split <= 2; split++ {
		prefix, makeErr := m.MkString("ab"[:split])
		astTestMust(t, makeErr)
		suffix, makeErr := m.MkString("ab"[split:])
		astTestMust(t, makeErr)
		xEquality, makeErr := m.MkEq(x, prefix)
		astTestMust(t, makeErr)
		restEquality, makeErr := m.MkEq(rest, suffix)
		astTestMust(t, makeErr)
		alternative, makeErr := m.MkAnd(xEquality, restEquality)
		astTestMust(t, makeErr)
		excluded, makeErr := m.MkNot(alternative)
		astTestMust(t, makeErr)
		astTestMust(t, solver.Assert(excluded))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed word prefix split alternatives were incomplete", status)
	}
}

func TestSMTSequenceTheoryRejectsImpossibleCyclicBinaryEquations(t *testing.T) {
	for _, test := range []struct {
		name  string
		left  string
		right string
	}{
		{name: "unequal-tail-lengths", left: "a", right: "bb"},
		{name: "tails-are-not-rotations", left: "ab", right: "aa"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			leftTail, err := m.MkString(test.left)
			astTestMust(t, err)
			rightHead, err := m.MkString(test.right)
			astTestMust(t, err)
			sort, err := m.GetSort(leftTail)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-cyclic-binary-x"), sort)
			astTestMust(t, err)
			left, err := m.MkSequenceConcat(x, leftTail)
			astTestMust(t, err)
			right, err := m.MkSequenceConcat(rightHead, x)
			astTestMust(t, err)
			equality, err := m.MkEq(left, right)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(equality))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("impossible cyclic binary equation", status)
			}
		})
	}
}

func TestSMTSequenceTheoryConcreteRelationConflicts(t *testing.T) {
	for _, test := range []struct {
		name  string
		op    int32
		left  string
		right string
		value bool
	}{
		{name: "prefix-positive", op: 3, left: "bc", right: "abc", value: true},
		{name: "suffix-negative", op: 4, left: "bc", right: "abc", value: false},
		{name: "contains-positive", op: 5, left: "abc", right: "z", value: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			left, err := m.MkString(test.left)
			astTestMust(t, err)
			right, err := m.MkString(test.right)
			astTestMust(t, err)
			sort, err := m.GetSort(left)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-relation-"+test.name), sort)
			astTestMust(t, err)
			var relation, value ast.ASTHandle
			switch test.op {
			case 3:
				relation, err = m.MkSequencePrefix(left, x)
				value = right
			case 4:
				relation, err = m.MkSequenceSuffix(left, x)
				value = right
			case 5:
				relation, err = m.MkSequenceContains(x, right)
				value = left
			}
			astTestMust(t, err)
			binding, err := m.MkEq(x, value)
			astTestMust(t, err)
			assertion := relation
			if !test.value {
				assertion, err = m.MkNot(relation)
				astTestMust(t, err)
			}
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(assertion))
			astTestMust(t, solver.Assert(binding))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("concrete relation contradiction", status)
			}
		})
	}
}

func TestSMTSequenceTheoryRelationWitnessAndNonemptyAxioms(t *testing.T) {
	for _, test := range []struct {
		name      string
		operation int32
		skolem    string
	}{
		{name: "prefix", operation: 3, skolem: "seq.p.suffix"},
		{name: "suffix", operation: 4, skolem: "seq.s.prefix"},
		{name: "contains", operation: 5, skolem: "seq.idx.left"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			left, err := m.MkConst(astTestSymbol(t, m, "sequence-relation-left-"+test.name), sort)
			astTestMust(t, err)
			right, err := m.MkConst(astTestSymbol(t, m, "sequence-relation-right-"+test.name), sort)
			astTestMust(t, err)
			var relation ast.ASTHandle
			switch test.operation {
			case 3:
				relation, err = m.MkSequencePrefix(left, right)
			case 4:
				relation, err = m.MkSequenceSuffix(left, right)
			case 5:
				relation, err = m.MkSequenceContains(left, right)
			}
			astTestMust(t, err)
			name, err := m.Symbols.Intern(test.skolem)
			astTestMust(t, err)
			first, err := m.MkSequenceSkolem(name, []ast.ASTHandle{left, right}, sort)
			astTestMust(t, err)
			var word ast.ASTHandle
			switch test.operation {
			case 3:
				word, err = m.MkSequenceConcat(left, first)
			case 4:
				word, err = m.MkSequenceConcat(first, left)
			case 5:
				rightName, symbolErr := m.Symbols.Intern("seq.idx.right")
				astTestMust(t, symbolErr)
				last, makeErr := m.MkSequenceSkolem(rightName, []ast.ASTHandle{left, right}, sort)
				astTestMust(t, makeErr)
				tail, makeErr := m.MkSequenceConcat(right, last)
				astTestMust(t, makeErr)
				word, err = m.MkSequenceConcat(first, tail)
			}
			astTestMust(t, err)
			target := right
			if test.operation == 5 {
				target = left
			}
			definition, err := m.MkEq(word, target)
			astTestMust(t, err)
			notDefinition, err := m.MkNot(definition)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(relation))
			astTestMust(t, solver.Assert(notDefinition))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("relation witness contradiction", status)
			}
		})
	}

	m, modelConfig := smtModelGeneratorFixture(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "sequence-empty-prefix-right"), sort)
	astTestMust(t, err)
	prefix, err := m.MkSequencePrefix(empty, y)
	astTestMust(t, err)
	notPrefix, err := m.MkNot(prefix)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notPrefix))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("empty sequence is always a prefix", status)
	}
}

func TestSMTSequenceTheoryNonContainsExcludesBoundaryMatches(t *testing.T) {
	for _, boundary := range []string{"prefix", "suffix"} {
		t.Run(boundary, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			haystack, err := m.MkConst(astTestSymbol(t, m, "sequence-nc-haystack-"+boundary), sort)
			astTestMust(t, err)
			needle, err := m.MkConst(astTestSymbol(t, m, "sequence-nc-needle-"+boundary), sort)
			astTestMust(t, err)
			contains, err := m.MkSequenceContains(haystack, needle)
			astTestMust(t, err)
			notContains, err := m.MkNot(contains)
			astTestMust(t, err)
			var relation ast.ASTHandle
			if boundary == "prefix" {
				relation, err = m.MkSequencePrefix(needle, haystack)
			} else {
				relation, err = m.MkSequenceSuffix(needle, haystack)
			}
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(notContains))
			astTestMust(t, solver.Assert(relation))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("non-containment boundary match", status)
			}
		})
	}
}

func TestSMTSequenceTheoryNegativePrefixPropagatesToTails(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sort is not a sequence")
	}
	boundary, err := m.MkConst(astTestSymbol(t, m, "sequence-negative-prefix-boundary"), sort)
	astTestMust(t, err)
	container, err := m.MkConst(astTestSymbol(t, m, "sequence-negative-prefix-container"), sort)
	astTestMust(t, err)
	prefix, err := m.MkSequencePrefix(boundary, container)
	astTestMust(t, err)
	notPrefix, err := m.MkNot(prefix)
	astTestMust(t, err)
	empty, err := m.MkSequenceEmpty(sort)
	astTestMust(t, err)
	containerEmpty, err := m.MkEq(container, empty)
	astTestMust(t, err)
	containerNonempty, err := m.MkNot(containerEmpty)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	nthName, err := m.Symbols.Intern("seq.nth")
	astTestMust(t, err)
	boundaryHead, err := m.MkSequenceSkolem(nthName, []ast.ASTHandle{boundary, zero}, elementSort)
	astTestMust(t, err)
	containerHead, err := m.MkSequenceSkolem(nthName, []ast.ASTHandle{container, zero}, elementSort)
	astTestMust(t, err)
	headsEqual, err := m.MkEq(boundaryHead, containerHead)
	astTestMust(t, err)
	tailName, err := m.Symbols.Intern("seq.tail")
	astTestMust(t, err)
	boundaryTail, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{boundary, zero}, sort)
	astTestMust(t, err)
	containerTail, err := m.MkSequenceSkolem(tailName, []ast.ASTHandle{container, zero}, sort)
	astTestMust(t, err)
	tailPrefix, err := m.MkSequencePrefix(boundaryTail, containerTail)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notPrefix))
	astTestMust(t, solver.Assert(containerNonempty))
	astTestMust(t, solver.Assert(headsEqual))
	astTestMust(t, solver.Assert(tailPrefix))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("negative prefix did not propagate to sequence tails", status)
	}
}

// Native assign_eh selects propagate_not_suffix, not the #else recursive
// first/last alternative. An empty container must not expose that predicate.
func TestSMTSequenceTheoryNegativeSuffixDoesNotExposeInactiveInitialSegments(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	empty := astTestString(t, m, "")
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	boundary, err := m.MkConst(astTestSymbol(t, m, "suffix-active-boundary"), sort)
	astTestMust(t, err)
	container, err := m.MkConst(astTestSymbol(t, m, "suffix-active-container"), sort)
	astTestMust(t, err)
	suffix, err := m.MkSequenceSuffix(boundary, container)
	astTestMust(t, err)
	negative, err := m.MkNot(suffix)
	astTestMust(t, err)
	binding, err := m.MkEq(container, empty)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: config.Context, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(binding))
	astTestMust(t, solver.Assert(negative))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("negative suffix with an empty container", status)
	}
	name, err := m.Symbols.Intern("seq.first")
	astTestMust(t, err)
	left, err := m.MkSequenceSkolem(name, []ast.ASTHandle{boundary}, sort)
	astTestMust(t, err)
	right, err := m.MkSequenceSkolem(name, []ast.ASTHandle{container}, sort)
	astTestMust(t, err)
	inactive, err := m.MkSequenceSuffix(left, right)
	astTestMust(t, err)
	if _, found := solver.kernel.internalizer.BoolVar(inactive); found {
		t.Fatal("inactive first/last suffix alternative was instantiated")
	}
}

func TestSMTSequenceTheoryNegativeBoundaryWitnesses(t *testing.T) {
	name := "suffix"
	m, modelConfig := smtModelGeneratorFixture(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sequence sort")
	}
	boundary, err := m.MkConst(astTestSymbol(t, m, "sequence-negative-boundary-"+name), sort)
	astTestMust(t, err)
	container, err := m.MkConst(astTestSymbol(t, m, "sequence-negative-container-"+name), sort)
	astTestMust(t, err)
	var relation ast.ASTHandle

	relation, err = m.MkSequenceSuffix(boundary, container)

	astTestMust(t, err)
	notRelation, err := m.MkNot(relation)
	astTestMust(t, err)
	containerEmpty, err := m.MkEq(container, empty)
	astTestMust(t, err)
	containerNonempty, err := m.MkNot(containerEmpty)
	astTestMust(t, err)
	prefix := "seq.suffix."

	prefix = "seq.suffix."

	arguments := []ast.ASTHandle{boundary, container}
	xName, err := m.Symbols.Intern(prefix + "x")
	astTestMust(t, err)
	x, err := m.MkSequenceSkolem(xName, arguments, sort)
	astTestMust(t, err)
	yName, err := m.Symbols.Intern(prefix + "y")
	astTestMust(t, err)
	y, err := m.MkSequenceSkolem(yName, arguments, sort)
	astTestMust(t, err)
	cName, err := m.Symbols.Intern(prefix + "c")
	astTestMust(t, err)
	c, err := m.MkSequenceSkolem(cName, arguments, elementSort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(c)
	astTestMust(t, err)
	var word ast.ASTHandle

	tail, makeErr := m.MkSequenceConcat(unit, x)
	astTestMust(t, makeErr)
	word, err = m.MkSequenceConcat(y, tail)

	astTestMust(t, err)
	definition, err := m.MkEq(boundary, word)
	astTestMust(t, err)
	notDefinition, err := m.MkNot(definition)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notRelation))
	astTestMust(t, solver.Assert(containerNonempty))
	astTestMust(t, solver.Assert(notDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("negative boundary witness", status)
	}
}

func TestSMTSequenceTheoryAtBoundaryAxioms(t *testing.T) {
	for _, test := range []struct {
		name  string
		index string
		word  string
	}{
		{name: "negative-index", index: "-1", word: "symbolic"},
		{name: "past-end", index: "1", word: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			seed, err := m.MkString(test.word)
			astTestMust(t, err)
			sort, err := m.GetSort(seed)
			astTestMust(t, err)
			sequence := seed
			if test.word == "symbolic" {
				sequence, err = m.MkConst(astTestSymbol(t, m, "sequence-at-negative"), sort)
				astTestMust(t, err)
			}
			index, err := m.ArithmeticIntegerNumeralText(test.index)
			astTestMust(t, err)
			at, err := m.MkSequenceAt(sequence, index)
			astTestMust(t, err)
			empty, err := m.MkSequenceEmpty(sort)
			astTestMust(t, err)
			equality, err := m.MkEq(at, empty)
			astTestMust(t, err)
			notEmpty, err := m.MkNot(equality)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(notEmpty))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("at boundary value", status)
			}
		})
	}

	m, modelConfig := smtModelGeneratorFixture(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	sequence, err := m.MkConst(astTestSymbol(t, m, "sequence-at-valid"), sort)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	at, err := m.MkSequenceAt(sequence, zero)
	astTestMust(t, err)
	prefixName, err := m.Symbols.Intern("seq.pre")
	astTestMust(t, err)
	prefix, err := m.MkSequenceSkolem(prefixName, []ast.ASTHandle{sequence, zero}, sort)
	astTestMust(t, err)
	suffixName, err := m.Symbols.Intern("seq.post")
	astTestMust(t, err)
	suffix, err := m.MkSequenceSkolem(suffixName, []ast.ASTHandle{sequence, zero}, sort)
	astTestMust(t, err)
	tail, err := m.MkSequenceConcat(at, suffix)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(prefix, tail)
	astTestMust(t, err)
	definition, err := m.MkEq(sequence, word)
	astTestMust(t, err)
	notDefinition, err := m.MkNot(definition)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(sequence)
	astTestMust(t, err)
	lengthOne, err := m.MkEq(length, one)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lengthOne))
	astTestMust(t, solver.Assert(notDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("at in-bounds decomposition", status)
	}
}

func TestSMTSequenceTheoryExtractBoundaryAndDecompositionAxioms(t *testing.T) {
	for _, test := range []struct{ name, offset, length string }{
		{name: "negative-offset", offset: "-1", length: "1"},
		{name: "zero-length", offset: "0", length: "0"},
		{name: "negative-length", offset: "0", length: "-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			sequence, err := m.MkConst(astTestSymbol(t, m, "sequence-extract-"+test.name), sort)
			astTestMust(t, err)
			offset, err := m.ArithmeticIntegerNumeralText(test.offset)
			astTestMust(t, err)
			length, err := m.ArithmeticIntegerNumeralText(test.length)
			astTestMust(t, err)
			extract, err := m.MkSequenceExtract(sequence, offset, length)
			astTestMust(t, err)
			equality, err := m.MkEq(extract, empty)
			astTestMust(t, err)
			notEmpty, err := m.MkNot(equality)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(notEmpty))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("extract boundary value", status)
			}
		})
	}

	m, modelConfig := smtModelGeneratorFixture(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	sequence, err := m.MkConst(astTestSymbol(t, m, "sequence-extract-valid"), sort)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	extract, err := m.MkSequenceExtract(sequence, zero, one)
	astTestMust(t, err)
	sequenceLength, err := m.MkSequenceLength(sequence)
	astTestMust(t, err)
	remaining, err := m.MkArithmeticSub(sequenceLength, zero)
	astTestMust(t, err)
	postLength, err := m.MkArithmeticSub(remaining, one)
	astTestMust(t, err)
	prefixName, err := m.Symbols.Intern("seq.pre")
	astTestMust(t, err)
	prefix, err := m.MkSequenceSkolem(prefixName, []ast.ASTHandle{sequence, zero}, sort)
	astTestMust(t, err)
	suffixName, err := m.Symbols.Intern("seq.post")
	astTestMust(t, err)
	suffix, err := m.MkSequenceSkolem(suffixName, []ast.ASTHandle{sequence, postLength}, sort)
	astTestMust(t, err)
	tail, err := m.MkSequenceConcat(extract, suffix)
	astTestMust(t, err)
	word, err := m.MkSequenceConcat(prefix, tail)
	astTestMust(t, err)
	definition, err := m.MkEq(sequence, word)
	astTestMust(t, err)
	notDefinition, err := m.MkNot(definition)
	astTestMust(t, err)
	nonnegative, err := m.MkArithmeticGE(zero, zero)
	astTestMust(t, err)
	pastEnd, err := m.MkArithmeticGE(zero, sequenceLength)
	astTestMust(t, err)
	inside, err := m.MkNot(pastEnd)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(nonnegative))
	astTestMust(t, solver.Assert(inside))
	astTestMust(t, solver.Assert(notDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("extract in-bounds decomposition", status)
	}
}

func TestSMTSequenceTheoryExtractOversizedPrefixIsSource(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	value, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-prefix-source"), sort)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	extract, err := m.MkSequenceExtract(x, zero, two)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	lengthOne, err := m.MkEq(length, one)
	astTestMust(t, err)
	definition, err := m.MkEq(extract, x)
	astTestMust(t, err)
	notDefinition, err := m.MkNot(definition)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(lengthOne))
	astTestMust(t, solver.Assert(notDefinition))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("oversized zero-offset extract differs from source", status)
	}
}

func TestSMTSequenceTheorySpecialTailAndDropLastDecompositions(t *testing.T) {
	for _, test := range []struct {
		name   string
		offset string
		tail   bool
	}{
		{name: "tail", offset: "1", tail: true},
		{name: "drop-last", offset: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			value, err := m.MkString("a")
			astTestMust(t, err)
			sort, err := m.GetSort(value)
			astTestMust(t, err)
			_, sequenceSort, err := m.SequenceSortElement(sort)
			astTestMust(t, err)
			if !sequenceSort {
				t.Fatal("string sort is not a sequence")
			}
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-special-extract-"+test.name), sort)
			astTestMust(t, err)
			length, err := m.MkSequenceLength(x)
			astTestMust(t, err)
			one, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			requested, err := m.MkArithmeticSub(length, one)
			astTestMust(t, err)
			offset, err := m.ArithmeticIntegerNumeralText(test.offset)
			astTestMust(t, err)
			extract, err := m.MkSequenceExtract(x, offset, requested)
			astTestMust(t, err)
			var witness ast.ASTHandle
			if test.tail {
				zero, makeErr := m.ArithmeticIntegerNumeralText("0")
				astTestMust(t, makeErr)
				name, makeErr := m.Symbols.Intern("seq.tail")
				astTestMust(t, makeErr)
				witness, makeErr = m.MkSequenceSkolem(name, []ast.ASTHandle{x, zero}, sort)
				astTestMust(t, makeErr)
			} else {
				name, makeErr := m.Symbols.Intern("seq.first")
				astTestMust(t, makeErr)
				witness, makeErr = m.MkSequenceSkolem(name, []ast.ASTHandle{x}, sort)
				astTestMust(t, makeErr)
			}
			definition, err := m.MkEq(extract, witness)
			astTestMust(t, err)
			notDefinition, err := m.MkNot(definition)
			astTestMust(t, err)
			empty, err := m.MkSequenceEmpty(sort)
			astTestMust(t, err)
			emptyEquality, err := m.MkEq(x, empty)
			astTestMust(t, err)
			nonempty, err := m.MkNot(emptyEquality)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(nonempty))
			astTestMust(t, solver.Assert(notDefinition))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("special extract lacks direct decomposition", status)
			}
		})
	}
}

func TestSMTSequenceTheoryIndexAxioms(t *testing.T) {
	for _, test := range []struct {
		name   string
		offset string
		empty  bool
		absent bool
		want   string
	}{
		{name: "negative-offset", offset: "-1", want: "-1"},
		{name: "empty-needle", offset: "0", empty: true, want: "0"},
		{name: "absent", offset: "0", absent: true, want: "-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			source, err := m.MkConst(astTestSymbol(t, m, "sequence-index-source-"+test.name), sort)
			astTestMust(t, err)
			needle := empty
			if !test.empty {
				needle, err = m.MkConst(astTestSymbol(t, m, "sequence-index-needle-"+test.name), sort)
				astTestMust(t, err)
			}
			offset, err := m.ArithmeticIntegerNumeralText(test.offset)
			astTestMust(t, err)
			index, err := m.MkSequenceIndex(source, needle, offset)
			astTestMust(t, err)
			want, err := m.ArithmeticIntegerNumeralText(test.want)
			astTestMust(t, err)
			equality, err := m.MkEq(index, want)
			astTestMust(t, err)
			notEquality, err := m.MkNot(equality)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			if test.absent {
				contains, makeErr := m.MkSequenceContains(source, needle)
				astTestMust(t, makeErr)
				notContains, makeErr := m.MkNot(contains)
				astTestMust(t, makeErr)
				astTestMust(t, solver.Assert(notContains))
			}
			astTestMust(t, solver.Assert(notEquality))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("index boundary result", status)
			}
		})
	}

	m, modelConfig := smtModelGeneratorFixture(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	source, err := m.MkConst(astTestSymbol(t, m, "sequence-index-witness-source"), sort)
	astTestMust(t, err)
	needle, err := m.MkConst(astTestSymbol(t, m, "sequence-index-witness-needle"), sort)
	astTestMust(t, err)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	index, err := m.MkSequenceIndex(source, needle, zero)
	astTestMust(t, err)
	contains, err := m.MkSequenceContains(source, needle)
	astTestMust(t, err)
	prefixName, err := m.Symbols.Intern("seq.idx.left")
	astTestMust(t, err)
	prefix, err := m.MkSequenceSkolem(prefixName, []ast.ASTHandle{source, needle}, sort)
	astTestMust(t, err)
	prefixLength, err := m.MkSequenceLength(prefix)
	astTestMust(t, err)
	indexLength, err := m.MkEq(index, prefixLength)
	astTestMust(t, err)
	notIndexLength, err := m.MkNot(indexLength)
	astTestMust(t, err)
	emptyNeedle, err := m.MkEq(needle, empty)
	astTestMust(t, err)
	notEmptyNeedle, err := m.MkNot(emptyNeedle)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(contains))
	astTestMust(t, solver.Assert(notEmptyNeedle))
	astTestMust(t, solver.Assert(notIndexLength))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("zero-offset index witness", status)
	}
}

func TestSMTSequenceTheoryIndexTightestPrefixAxioms(t *testing.T) {
	for _, test := range []struct {
		name          string
		excludePrefix bool
	}{
		{name: "decomposition", excludePrefix: true},
		{name: "minimality"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			elementSort, sequence, err := m.SequenceSortElement(sort)
			astTestMust(t, err)
			if !sequence {
				t.Fatal("string sort is not a sequence")
			}
			source, err := m.MkConst(astTestSymbol(t, m, "sequence-tightest-source-"+test.name), sort)
			astTestMust(t, err)
			needle, err := m.MkConst(astTestSymbol(t, m, "sequence-tightest-needle-"+test.name), sort)
			astTestMust(t, err)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			index, err := m.MkSequenceIndex(source, needle, zero)
			astTestMust(t, err)
			contains, err := m.MkSequenceContains(source, needle)
			astTestMust(t, err)
			needleEmpty, err := m.MkEq(needle, empty)
			astTestMust(t, err)
			needleNonempty, err := m.MkNot(needleEmpty)
			astTestMust(t, err)
			prefixName, err := m.Symbols.Intern("seq.idx.left")
			astTestMust(t, err)
			prefix, err := m.MkSequenceSkolem(prefixName, []ast.ASTHandle{source, needle}, sort)
			astTestMust(t, err)
			prefixEmpty, err := m.MkEq(prefix, empty)
			astTestMust(t, err)
			prefixNonempty, err := m.MkNot(prefixEmpty)
			astTestMust(t, err)
			prefixLength, err := m.MkSequenceLength(prefix)
			astTestMust(t, err)
			indexLength, err := m.MkEq(index, prefixLength)
			astTestMust(t, err)
			firstName, err := m.Symbols.Intern("seq.first")
			astTestMust(t, err)
			initial, err := m.MkSequenceSkolem(firstName, []ast.ASTHandle{prefix}, sort)
			astTestMust(t, err)
			lastName, err := m.Symbols.Intern("seq.last")
			astTestMust(t, err)
			last, err := m.MkSequenceSkolem(lastName, []ast.ASTHandle{prefix}, elementSort)
			astTestMust(t, err)
			unit, err := m.MkSequenceUnit(last)
			astTestMust(t, err)
			decomposition, err := m.MkSequenceConcat(initial, unit)
			astTestMust(t, err)
			prefixDefinition, err := m.MkEq(prefix, decomposition)
			astTestMust(t, err)
			violation := prefixDefinition
			if test.excludePrefix {
				violation, err = m.MkNot(prefixDefinition)
				astTestMust(t, err)
			} else {
				needleAndInitial, makeErr := m.MkSequenceConcat(needle, initial)
				astTestMust(t, makeErr)
				violation, makeErr = m.MkSequenceContains(needleAndInitial, prefix)
				astTestMust(t, makeErr)
			}
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			for _, assertion := range []ast.ASTHandle{contains, needleNonempty, prefixNonempty, indexLength, violation} {
				astTestMust(t, solver.Assert(assertion))
			}
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("index tightest-prefix axiom", status)
			}
		})
	}
}

func TestSMTSequenceTheoryReplaceAxioms(t *testing.T) {
	t.Run("empty-needle", func(t *testing.T) {
		m, modelConfig := smtModelGeneratorFixture(t)
		empty, err := m.MkString("")
		astTestMust(t, err)
		sort, err := m.GetSort(empty)
		astTestMust(t, err)
		source, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-empty-source"), sort)
		astTestMust(t, err)
		replacement, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-empty-value"), sort)
		astTestMust(t, err)
		result, err := m.MkSequenceReplace(source, empty, replacement)
		astTestMust(t, err)
		word, err := m.MkSequenceConcat(replacement, source)
		astTestMust(t, err)
		equality, err := m.MkEq(result, word)
		astTestMust(t, err)
		contradiction, err := m.MkNot(equality)
		astTestMust(t, err)
		solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, solver.Close()) })
		astTestMust(t, solver.Assert(contradiction))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("empty needle replacement", status)
		}
	})

	for _, found := range []bool{false, true} {
		name := "absent"
		if found {
			name = "found"
		}
		t.Run(name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			source, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-source-"+name), sort)
			astTestMust(t, err)
			needle, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-needle-"+name), sort)
			astTestMust(t, err)
			replacement, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-value-"+name), sort)
			astTestMust(t, err)
			result, err := m.MkSequenceReplace(source, needle, replacement)
			astTestMust(t, err)
			contains, err := m.MkSequenceContains(source, needle)
			astTestMust(t, err)
			assertion := contains
			want := source
			if found {
				prefixName, makeErr := m.Symbols.Intern("seq.idx.left")
				astTestMust(t, makeErr)
				prefix, makeErr := m.MkSequenceSkolem(prefixName, []ast.ASTHandle{source, needle}, sort)
				astTestMust(t, makeErr)
				suffixName, makeErr := m.Symbols.Intern("seq.idx.right")
				astTestMust(t, makeErr)
				suffix, makeErr := m.MkSequenceSkolem(suffixName, []ast.ASTHandle{source, needle}, sort)
				astTestMust(t, makeErr)
				tail, makeErr := m.MkSequenceConcat(replacement, suffix)
				astTestMust(t, makeErr)
				want, makeErr = m.MkSequenceConcat(prefix, tail)
				astTestMust(t, makeErr)
			} else {
				assertion, err = m.MkNot(contains)
				astTestMust(t, err)
			}
			equality, err := m.MkEq(result, want)
			astTestMust(t, err)
			notEquality, err := m.MkNot(equality)
			astTestMust(t, err)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(assertion))
			if found {
				sourceEmpty, makeErr := m.MkEq(source, empty)
				astTestMust(t, makeErr)
				needleEmpty, makeErr := m.MkEq(needle, empty)
				astTestMust(t, makeErr)
				notSourceEmpty, makeErr := m.MkNot(sourceEmpty)
				astTestMust(t, makeErr)
				notNeedleEmpty, makeErr := m.MkNot(needleEmpty)
				astTestMust(t, makeErr)
				astTestMust(t, solver.Assert(notSourceEmpty))
				astTestMust(t, solver.Assert(notNeedleEmpty))
			}
			astTestMust(t, solver.Assert(notEquality))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("replace definition", status)
			}
		})
	}
}

func TestSMTSequenceTheoryReplaceTightestPrefixAxioms(t *testing.T) {
	m, _ := smtModelGeneratorFixture(t)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	elementSort, sequence, err := m.SequenceSortElement(sort)
	astTestMust(t, err)
	if !sequence {
		t.Fatal("string sort is not a sequence")
	}
	source, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-tightest-source"), sort)
	astTestMust(t, err)
	needle, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-tightest-needle"), sort)
	astTestMust(t, err)
	replacement, err := m.MkConst(astTestSymbol(t, m, "sequence-replace-tightest-value"), sort)
	astTestMust(t, err)
	result, err := m.MkSequenceReplace(source, needle, replacement)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, theory.addReplaceAxioms(result))
	prefixName, err := m.Symbols.Intern("seq.idx.left")
	astTestMust(t, err)
	prefix, err := m.MkSequenceSkolem(prefixName, []ast.ASTHandle{source, needle}, sort)
	astTestMust(t, err)
	prefixEmpty, err := m.MkEq(prefix, empty)
	astTestMust(t, err)
	prefixNonempty, err := m.MkNot(prefixEmpty)
	astTestMust(t, err)
	firstName, err := m.Symbols.Intern("seq.first")
	astTestMust(t, err)
	initial, err := m.MkSequenceSkolem(firstName, []ast.ASTHandle{prefix}, sort)
	astTestMust(t, err)
	lastName, err := m.Symbols.Intern("seq.last")
	astTestMust(t, err)
	last, err := m.MkSequenceSkolem(lastName, []ast.ASTHandle{prefix}, elementSort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(last)
	astTestMust(t, err)
	decomposition, err := m.MkSequenceConcat(initial, unit)
	astTestMust(t, err)
	prefixDefinition, err := m.MkEq(prefix, decomposition)
	astTestMust(t, err)
	definitionAxiom, err := m.MkImplies(prefixNonempty, prefixDefinition)
	astTestMust(t, err)
	needleAndInitial, err := m.MkSequenceConcat(needle, initial)
	astTestMust(t, err)
	prematureMatch, err := m.MkSequenceContains(needleAndInitial, prefix)
	astTestMust(t, err)
	notPrematureMatch, err := m.MkNot(prematureMatch)
	astTestMust(t, err)
	minimalityAxiom, err := m.MkImplies(prefixNonempty, notPrematureMatch)
	astTestMust(t, err)
	if _, found := theory.axioms.set[definitionAxiom]; !found {
		t.Fatal("replace lacks tightest-prefix decomposition")
	}
	if _, found := theory.axioms.set[minimalityAxiom]; !found {
		t.Fatal("replace lacks tightest-prefix minimality")
	}
}

func TestSMTSequenceTheoryRegexMembershipReduction(t *testing.T) {
	t.Run("singleton-sat", func(t *testing.T) {
		m, modelConfig := smtModelGeneratorFixture(t)
		literal, err := m.MkString("ab")
		astTestMust(t, err)
		sort, err := m.GetSort(literal)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-singleton-sat"), sort)
		astTestMust(t, err)
		regex, err := m.MkSequenceToRegex(literal)
		astTestMust(t, err)
		membership, err := m.MkSequenceInRegex(x, regex)
		astTestMust(t, err)
		equality, err := m.MkEq(x, literal)
		astTestMust(t, err)
		solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, solver.Close()) })
		astTestMust(t, solver.Assert(membership))
		astTestMust(t, solver.Assert(equality))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("exact singleton membership remained incomplete", status)
		}
	})

	t.Run("singleton-symbolic", func(t *testing.T) {
		m, modelConfig := smtModelGeneratorFixture(t)
		literal, err := m.MkString("ab")
		astTestMust(t, err)
		sort, err := m.GetSort(literal)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-singleton"), sort)
		astTestMust(t, err)
		regex, err := m.MkSequenceToRegex(literal)
		astTestMust(t, err)
		membership, err := m.MkSequenceInRegex(x, regex)
		astTestMust(t, err)
		equality, err := m.MkEq(x, literal)
		astTestMust(t, err)
		disequality, err := m.MkNot(equality)
		astTestMust(t, err)
		solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, solver.Close()) })
		astTestMust(t, solver.Assert(membership))
		astTestMust(t, solver.Assert(disequality))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("singleton membership reduction", status)
		}
	})

	t.Run("concrete-union-rejection", func(t *testing.T) {
		m, modelConfig := smtModelGeneratorFixture(t)
		input, err := m.MkString("c")
		astTestMust(t, err)
		a, err := m.MkString("a")
		astTestMust(t, err)
		b, err := m.MkString("b")
		astTestMust(t, err)
		ra, err := m.MkSequenceToRegex(a)
		astTestMust(t, err)
		rb, err := m.MkSequenceToRegex(b)
		astTestMust(t, err)
		union, err := m.MkRegexUnion(ra, rb)
		astTestMust(t, err)
		membership, err := m.MkSequenceInRegex(input, union)
		astTestMust(t, err)
		solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, solver.Close()) })
		astTestMust(t, solver.Assert(membership))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("concrete regex rejection", status)
		}
	})
}

func TestSMTSequenceTheoryRetriesRegexAfterWordSolution(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-decomposed-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	xIsB, err := m.MkEq(x, b)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range []ast.ASTHandle{membership, xIsB} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("word-solved regex membership", status)
	}
}

func TestSMTSequenceTheoryRetriesRegexAfterFixedLengthDecomposition(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	aa, err := m.MkString("aa")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-fixed-length-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	fixedLength, err := m.MkEq(length, two)
	astTestMust(t, err)
	xIsAA, err := m.MkEq(x, aa)
	astTestMust(t, err)
	xIsNotAA, err := m.MkNot(xIsAA)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range []ast.ASTHandle{membership, fixedLength, xIsNotAA} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("fixed-length decomposed regex membership", status)
	}
}

func TestSMTSequenceTheoryCompletesFixedLengthRegexModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	ab, err := m.MkString("ab")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-fixed-length-model-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	fixedLength, err := m.MkEq(length, two)
	astTestMust(t, err)
	xIsAB, err := m.MkEq(x, ab)
	astTestMust(t, err)
	xIsNotAB, err := m.MkNot(xIsAB)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range []ast.ASTHandle{membership, fixedLength, xIsNotAB} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("fixed-length regex model remained incomplete", status)
	}
}

func TestSMTSequenceTheoryCompletesBoundedLengthRegexModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	aa, err := m.MkString("aa")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-bounded-length-model-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	lowerBound, err := m.MkArithmeticGE(length, one)
	astTestMust(t, err)
	upperBound, err := m.MkArithmeticLE(length, two)
	astTestMust(t, err)
	xIsA, err := m.MkEq(x, a)
	astTestMust(t, err)
	xIsNotA, err := m.MkNot(xIsA)
	astTestMust(t, err)
	xIsAA, err := m.MkEq(x, aa)
	astTestMust(t, err)
	xIsNotAA, err := m.MkNot(xIsAA)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range []ast.ASTHandle{membership, lowerBound, upperBound, xIsNotA, xIsNotAA} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("bounded-length regex model remained incomplete", status)
	}
}

func TestSMTSequenceTheoryCompletesMinimumLengthRegexModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-minimum-length-model-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	plus, err := m.MkRegexPlus(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, plus)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	lowerBound, err := m.MkArithmeticGE(length, two)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range []ast.ASTHandle{membership, lowerBound} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, reasonErr := solver.ReasonUnknown()
		astTestMust(t, reasonErr)
		t.Fatal("minimum-length regex model remained incomplete", status, reason)
	}
}

func TestSMTSequenceTheoryCompletesUnboundedRegexModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-unbounded-model-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(membership))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, reasonErr := solver.ReasonUnknown()
		astTestMust(t, reasonErr)
		t.Fatal("unbounded regex model remained incomplete", status, reason)
	}
}

func TestSMTSequenceTheoryAdvancesConcreteRegexWitness(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-next-witness-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	plus, err := m.MkRegexPlus(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, plus)
	astTestMust(t, err)
	xIsA, err := m.MkEq(x, a)
	astTestMust(t, err)
	xIsNotA, err := m.MkNot(xIsA)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range []ast.ASTHandle{membership, xIsNotA} {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("unbounded regex did not advance past its shortest witness", status)
	}
}

func TestSMTSequenceTheoryCompletesNegativeUnboundedRegexModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sequenceSort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-negative-unbounded-x"), sequenceSort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	notMembership, err := m.MkNot(membership)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notMembership))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, reasonErr := solver.ReasonUnknown()
		astTestMust(t, reasonErr)
		t.Fatal("negative unbounded regex model remained incomplete", status, reason)
	}
}

func TestSMTSequenceTheoryCompletesSymbolicUnboundedRegexModel(t *testing.T) {
	for _, rejectSingleton := range []bool{false, true} {
		name := "singleton"
		if rejectSingleton {
			name = "advance"
		}
		t.Run(name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			_, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, err = m.RegisterBitVectorPlugin()
			astTestMust(t, err)
			zero, err := m.BitVectorNumeralUnsigned(0, 8)
			astTestMust(t, err)
			byteSort, err := m.GetSort(zero)
			astTestMust(t, err)
			character, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-symbolic-byte-"+name), byteSort)
			astTestMust(t, err)
			unit, err := m.MkSequenceUnit(character)
			astTestMust(t, err)
			sequenceSort, err := m.GetSort(unit)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-symbolic-x-"+name), sequenceSort)
			astTestMust(t, err)
			unitRegex, err := m.MkSequenceToRegex(unit)
			astTestMust(t, err)
			plus, err := m.MkRegexPlus(unitRegex)
			astTestMust(t, err)
			membership, err := m.MkSequenceInRegex(x, plus)
			astTestMust(t, err)
			assertions := []ast.ASTHandle{membership}
			if rejectSingleton {
				length, makeErr := m.MkSequenceLength(x)
				astTestMust(t, makeErr)
				two, makeErr := m.ArithmeticIntegerNumeralText("2")
				astTestMust(t, makeErr)
				lowerBound, makeErr := m.MkArithmeticGE(length, two)
				astTestMust(t, makeErr)
				assertions = append(assertions, lowerBound)
			}
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			for _, assertion := range assertions {
				astTestMust(t, solver.Assert(assertion))
			}
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				reason, reasonErr := solver.ReasonUnknown()
				astTestMust(t, reasonErr)
				t.Fatal("symbolic unbounded regex model remained incomplete", status, reason)
			}
		})
	}
}

func TestSMTSequenceTheoryCompletesNegativeSymbolicUnboundedRegexModel(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	zero, err := m.BitVectorNumeralUnsigned(0, 8)
	astTestMust(t, err)
	byteSort, err := m.GetSort(zero)
	astTestMust(t, err)
	character, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-negative-symbolic-byte"), byteSort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(character)
	astTestMust(t, err)
	sequenceSort, err := m.GetSort(unit)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-negative-symbolic-x"), sequenceSort)
	astTestMust(t, err)
	unitRegex, err := m.MkSequenceToRegex(unit)
	astTestMust(t, err)
	star, err := m.MkRegexStar(unitRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	notMembership, err := m.MkNot(membership)
	astTestMust(t, err)
	// This case uses Seq(BitVec8) and the native QF_BVRE setup.
	// Ordinary ALL/mixed-arithmetic completion remains a separately tracked gap.
	solver, err := NewSMTSolver(m, nil, "QF_BVRE", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notMembership))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		reason, reasonErr := solver.ReasonUnknown()
		astTestMust(t, reasonErr)
		t.Fatal("negative symbolic unbounded regex model remained incomplete", status, reason)
	}
}

func TestSMTSequenceTheoryClosesEmptyNegativeRegexComplement(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sequenceSort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-negative-full-x"), sequenceSort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	regexSort, err := m.GetSort(aRegex)
	astTestMust(t, err)
	full, err := m.MkRegexFull(regexSort)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, full)
	astTestMust(t, err)
	notMembership, err := m.MkNot(membership)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(notMembership))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("empty negative regex complement remained open", status)
	}
}

func TestSMTSequenceTheoryClosesExhaustedSymbolicRegexLengths(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	zero, err := m.BitVectorNumeralUnsigned(0, 8)
	astTestMust(t, err)
	byteSort, err := m.GetSort(zero)
	astTestMust(t, err)
	characters := make([]ast.ASTHandle, 2)
	regexes := make([]ast.ASTHandle, 2)
	var sequenceSort ast.ASTHandle
	for index, name := range []string{"left", "right"} {
		characters[index], err = m.MkConst(astTestSymbol(t, m, "sequence-regex-symbolic-finite-"+name), byteSort)
		astTestMust(t, err)
		unit, makeErr := m.MkSequenceUnit(characters[index])
		astTestMust(t, makeErr)
		sequenceSort, makeErr = m.GetSort(unit)
		astTestMust(t, makeErr)
		regexes[index], makeErr = m.MkSequenceToRegex(unit)
		astTestMust(t, makeErr)
	}
	union, err := m.MkRegexUnion(regexes[0], regexes[1])
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-symbolic-finite-x"), sequenceSort)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, union)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	lengthOne, err := m.MkEq(length, one)
	astTestMust(t, err)
	notLengthOne, err := m.MkNot(lengthOne)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.Assert(membership))
	astTestMust(t, solver.Assert(notLengthOne))
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("exhausted symbolic regex lengths remained open", status)
	}
}

func TestSMTSequenceTheoryClosesExhaustedFiniteRegexLanguage(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	b, err := m.MkString("b")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-finite-witness-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	bRegex, err := m.MkSequenceToRegex(b)
	astTestMust(t, err)
	union, err := m.MkRegexUnion(aRegex, bRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, union)
	astTestMust(t, err)
	assertions := []ast.ASTHandle{membership}
	for _, value := range []ast.ASTHandle{a, b} {
		equality, makeErr := m.MkEq(x, value)
		astTestMust(t, makeErr)
		disequality, makeErr := m.MkNot(equality)
		astTestMust(t, makeErr)
		assertions = append(assertions, disequality)
	}
	solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	for _, assertion := range assertions {
		astTestMust(t, solver.Assert(assertion))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("exhausted finite regex language remained open", status)
	}
}

func TestSMTSequenceTheoryBranchesConcreteRegexWitness(t *testing.T) {
	m, _ := smtModelGeneratorFixture(t)
	a, err := m.MkString("a")
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "sequence-regex-witness-x"), sort)
	astTestMust(t, err)
	aRegex, err := m.MkSequenceToRegex(a)
	astTestMust(t, err)
	star, err := m.MkRegexStar(aRegex)
	astTestMust(t, err)
	membership, err := m.MkSequenceInRegex(x, star)
	astTestMust(t, err)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	astTestMust(t, state.Internalize(membership, true))
	literal, err := state.Literal(membership)
	astTestMust(t, err)
	astTestMust(t, state.assignDecisionLiteral(literal))
	branched, reduced, err := theory.branchConcreteRegexWitness(membership, x, star)
	astTestMust(t, err)
	if !branched || reduced {
		t.Fatal("regex witness branch", branched, reduced)
	}
	empty, err := m.MkString("")
	astTestMust(t, err)
	equality, err := m.MkEq(x, empty)
	astTestMust(t, err)
	equalityLiteral, err := state.Literal(equality)
	astTestMust(t, err)
	data, err := state.BooleanData(equalityLiteral.Var())
	astTestMust(t, err)
	if !data.TryTrueFirst() {
		t.Fatal("shortest regex witness lacks native true-first preference")
	}
}

func TestSMTSequenceTheoryLengthAxioms(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T, *ast.ASTManager) ast.ASTHandle
	}{
		{name: "string", build: func(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
			value, err := m.MkString("abc")
			astTestMust(t, err)
			length, err := m.MkSequenceLength(value)
			astTestMust(t, err)
			three, err := m.ArithmeticIntegerNumeralText("3")
			astTestMust(t, err)
			equality, err := m.MkEq(length, three)
			astTestMust(t, err)
			result, err := m.MkNot(equality)
			astTestMust(t, err)
			return result
		}},
		{name: "concat", build: func(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
			literal, err := m.MkString("ab")
			astTestMust(t, err)
			sort, err := m.GetSort(literal)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-length-x"), sort)
			astTestMust(t, err)
			word, err := m.MkSequenceConcat(x, literal)
			astTestMust(t, err)
			wordLength, err := m.MkSequenceLength(word)
			astTestMust(t, err)
			xLength, err := m.MkSequenceLength(x)
			astTestMust(t, err)
			two, err := m.ArithmeticIntegerNumeralText("2")
			astTestMust(t, err)
			sum, err := m.MkArithmeticAdd(xLength, two)
			astTestMust(t, err)
			equality, err := m.MkEq(wordLength, sum)
			astTestMust(t, err)
			result, err := m.MkNot(equality)
			astTestMust(t, err)
			return result
		}},
		{name: "nonnegative", build: func(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-nonnegative-x"), sort)
			astTestMust(t, err)
			length, err := m.MkSequenceLength(x)
			astTestMust(t, err)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			nonnegative, err := m.MkArithmeticGE(length, zero)
			astTestMust(t, err)
			result, err := m.MkNot(nonnegative)
			astTestMust(t, err)
			return result
		}},
		{name: "zero-iff-empty", build: func(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
			empty, err := m.MkString("")
			astTestMust(t, err)
			sort, err := m.GetSort(empty)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-zero-length-x"), sort)
			astTestMust(t, err)
			length, err := m.MkSequenceLength(x)
			astTestMust(t, err)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			zeroLength, err := m.MkEq(length, zero)
			astTestMust(t, err)
			emptyValue, err := m.MkEq(x, empty)
			astTestMust(t, err)
			notEmpty, err := m.MkNot(emptyValue)
			astTestMust(t, err)
			result, err := m.MkAnd(zeroLength, notEmpty)
			astTestMust(t, err)
			return result
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			assertion := test.build(t, m)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			astTestMust(t, solver.Assert(assertion))
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("sequence length axiom contradiction", status)
			}
		})
	}
}

func TestSMTSequenceTheoryIntegerStringLengthThreshold(t *testing.T) {
	for _, test := range []struct {
		name     string
		oneDigit bool
	}{
		{name: "one-digit", oneDigit: true},
		{name: "multiple-digits"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			one, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			ten, err := m.ArithmeticIntegerNumeralText("10")
			astTestMust(t, err)
			integerSort, err := m.GetSort(zero)
			astTestMust(t, err)
			n, err := m.MkConst(astTestSymbol(t, m, "sequence-itos-length-"+test.name), integerSort)
			astTestMust(t, err)
			converted, err := m.MkIntToString(n)
			astTestMust(t, err)
			length, err := m.MkSequenceLength(converted)
			astTestMust(t, err)
			nAtLeastTen, err := m.MkArithmeticGE(n, ten)
			astTestMust(t, err)
			lengthAtMostOne, err := m.MkArithmeticLE(length, one)
			astTestMust(t, err)
			assertions := []ast.ASTHandle{nAtLeastTen, lengthAtMostOne}
			if test.oneDigit {
				nNonnegative, makeErr := m.MkArithmeticGE(n, zero)
				astTestMust(t, makeErr)
				nBelowTen, makeErr := m.MkNot(nAtLeastTen)
				astTestMust(t, makeErr)
				lengthOne, makeErr := m.MkEq(length, one)
				astTestMust(t, makeErr)
				notLengthOne, makeErr := m.MkNot(lengthOne)
				astTestMust(t, makeErr)
				assertions = []ast.ASTHandle{nNonnegative, nBelowTen, notLengthOne}
			}
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			for _, assertion := range assertions {
				astTestMust(t, solver.Assert(assertion))
			}
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("integer string length threshold", status)
			}
		})
	}
}

func TestSMTSequenceTheoryIntegerStringModelLengthRefinement(t *testing.T) {
	for _, test := range []struct {
		name         string
		fixedInteger bool
	}{
		{name: "fixed-integer", fixedInteger: true},
		{name: "fixed-length"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			integerSort, err := m.GetSort(zero)
			astTestMust(t, err)
			n, err := m.MkConst(astTestSymbol(t, m, "sequence-itos-model-length-"+test.name), integerSort)
			astTestMust(t, err)
			converted, err := m.MkIntToString(n)
			astTestMust(t, err)
			length, err := m.MkSequenceLength(converted)
			astTestMust(t, err)
			three, err := m.ArithmeticIntegerNumeralText("3")
			astTestMust(t, err)
			assertions := make([]ast.ASTHandle, 0, 2)
			if test.fixedInteger {
				oneHundredTwentyThree, makeErr := m.ArithmeticIntegerNumeralText("123")
				astTestMust(t, makeErr)
				binding, makeErr := m.MkEq(n, oneHundredTwentyThree)
				astTestMust(t, makeErr)
				two, makeErr := m.ArithmeticIntegerNumeralText("2")
				astTestMust(t, makeErr)
				tooShort, makeErr := m.MkArithmeticLE(length, two)
				astTestMust(t, makeErr)
				assertions = append(assertions, binding, tooShort)
			} else {
				fixedLength, makeErr := m.MkEq(length, three)
				astTestMust(t, makeErr)
				oneThousand, makeErr := m.ArithmeticIntegerNumeralText("1000")
				astTestMust(t, makeErr)
				tooLarge, makeErr := m.MkArithmeticGE(n, oneThousand)
				astTestMust(t, makeErr)
				assertions = append(assertions, fixedLength, tooLarge)
			}
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			for _, assertion := range assertions {
				astTestMust(t, solver.Assert(assertion))
			}
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("integer string model length refinement", status)
			}
		})
	}
}

func TestSMTSequenceTheoryIntegerStringAxioms(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T, *ast.ASTManager) []ast.ASTHandle
	}{
		{name: "negative-is-empty", build: func(t *testing.T, m *ast.ASTManager) []ast.ASTHandle {
			minusOne, err := m.ArithmeticIntegerNumeralText("-1")
			astTestMust(t, err)
			sort, err := m.GetSort(minusOne)
			astTestMust(t, err)
			n, err := m.MkConst(astTestSymbol(t, m, "sequence-itos-negative"), sort)
			astTestMust(t, err)
			binding, err := m.MkEq(n, minusOne)
			astTestMust(t, err)
			converted, err := m.MkIntToString(n)
			astTestMust(t, err)
			empty, err := m.MkString("")
			astTestMust(t, err)
			equality, err := m.MkEq(converted, empty)
			astTestMust(t, err)
			result, err := m.MkNot(equality)
			astTestMust(t, err)
			return []ast.ASTHandle{binding, result}
		}},
		{name: "nonnegative-roundtrip", build: func(t *testing.T, m *ast.ASTManager) []ast.ASTHandle {
			twelve, err := m.ArithmeticIntegerNumeralText("12")
			astTestMust(t, err)
			sort, err := m.GetSort(twelve)
			astTestMust(t, err)
			n, err := m.MkConst(astTestSymbol(t, m, "sequence-itos-positive"), sort)
			astTestMust(t, err)
			binding, err := m.MkEq(n, twelve)
			astTestMust(t, err)
			converted, err := m.MkIntToString(n)
			astTestMust(t, err)
			back, err := m.MkStringToInt(converted)
			astTestMust(t, err)
			equality, err := m.MkEq(back, twelve)
			astTestMust(t, err)
			result, err := m.MkNot(equality)
			astTestMust(t, err)
			return []ast.ASTHandle{binding, result}
		}},
		{name: "stoi-lower-bound", build: func(t *testing.T, m *ast.ASTManager) []ast.ASTHandle {
			value, err := m.MkString("not-a-number")
			astTestMust(t, err)
			sort, err := m.GetSort(value)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "sequence-stoi-string"), sort)
			astTestMust(t, err)
			binding, err := m.MkEq(x, value)
			astTestMust(t, err)
			converted, err := m.MkStringToInt(x)
			astTestMust(t, err)
			minusOne, err := m.ArithmeticIntegerNumeralText("-1")
			astTestMust(t, err)
			lowerBound, err := m.MkArithmeticGE(converted, minusOne)
			astTestMust(t, err)
			result, err := m.MkNot(lowerBound)
			astTestMust(t, err)
			return []ast.ASTHandle{binding, result}
		}},
		{name: "stoi-nondigits-are-negative", build: func(t *testing.T, m *ast.ASTManager) []ast.ASTHandle {
			value, err := m.MkString("not-a-number")
			astTestMust(t, err)
			converted, err := m.MkStringToInt(value)
			astTestMust(t, err)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			astTestMust(t, err)
			nonnegative, err := m.MkArithmeticGE(converted, zero)
			astTestMust(t, err)
			return []ast.ASTHandle{nonnegative}
		}},
		{name: "ground-itos-value", build: func(t *testing.T, m *ast.ASTManager) []ast.ASTHandle {
			twelve, err := m.ArithmeticIntegerNumeralText("12")
			astTestMust(t, err)
			converted, err := m.MkIntToString(twelve)
			astTestMust(t, err)
			text, err := m.MkString("12")
			astTestMust(t, err)
			equality, err := m.MkEq(converted, text)
			astTestMust(t, err)
			wrong, err := m.MkNot(equality)
			astTestMust(t, err)
			return []ast.ASTHandle{wrong}
		}},
		{name: "ground-stoi-leading-zero-value", build: func(t *testing.T, m *ast.ASTManager) []ast.ASTHandle {
			text, err := m.MkString("0012")
			astTestMust(t, err)
			converted, err := m.MkStringToInt(text)
			astTestMust(t, err)
			twelve, err := m.ArithmeticIntegerNumeralText("12")
			astTestMust(t, err)
			equality, err := m.MkEq(converted, twelve)
			astTestMust(t, err)
			wrong, err := m.MkNot(equality)
			astTestMust(t, err)
			return []ast.ASTHandle{wrong}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, modelConfig := smtModelGeneratorFixture(t)
			assertions := test.build(t, m)
			solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			for _, assertion := range assertions {
				astTestMust(t, solver.Assert(assertion))
			}
			status, err := solver.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.False {
				t.Fatal("integer/string axiom contradiction", status)
			}
		})
	}
}

func TestSMTSequenceTheoryGroundIntegerStringConversionsAreComplete(t *testing.T) {
	for _, build := range []func(*testing.T, *ast.ASTManager) ast.ASTHandle{
		func(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
			value, err := m.ArithmeticIntegerNumeralText("123456789012345678901234567890")
			astTestMust(t, err)
			converted, err := m.MkIntToString(value)
			astTestMust(t, err)
			expected, err := m.MkString("123456789012345678901234567890")
			astTestMust(t, err)
			equality, err := m.MkEq(converted, expected)
			astTestMust(t, err)
			return equality
		},
		func(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
			value, err := m.MkString("000000000000000000000000000042")
			astTestMust(t, err)
			converted, err := m.MkStringToInt(value)
			astTestMust(t, err)
			expected, err := m.ArithmeticIntegerNumeralText("42")
			astTestMust(t, err)
			equality, err := m.MkEq(converted, expected)
			astTestMust(t, err)
			return equality
		},
	} {
		m, modelConfig := smtModelGeneratorFixture(t)
		solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
		astTestMust(t, err)
		assertion := build(t, m)
		astTestMust(t, solver.Assert(assertion))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		astTestMust(t, solver.Close())
		if status != lbool.True {
			t.Fatal("exact ground conversion remained incomplete", status)
		}
	}
}

func TestSMTSequenceTheoryGroundOperationsAreComplete(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T, *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle)
	}{
		{name: "concat-left-prefix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-concat-prefix-x")
			y := astTestStringConst(t, m, "sequence-exact-concat-prefix-y")
			word, err := m.MkSequenceConcat(x, y)
			astTestMust(t, err)
			operation, err := m.MkSequencePrefix(x, word)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "concat-right-suffix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-concat-suffix-x")
			y := astTestStringConst(t, m, "sequence-exact-concat-suffix-y")
			word, err := m.MkSequenceConcat(x, y)
			astTestMust(t, err)
			operation, err := m.MkSequenceSuffix(y, word)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "concat-operand-contained", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-concat-contains-x")
			y := astTestStringConst(t, m, "sequence-exact-concat-contains-y")
			word, err := m.MkSequenceConcat(x, y)
			astTestMust(t, err)
			operation, err := m.MkSequenceContains(word, y)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "reflexive-prefix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-reflexive-prefix-x")
			operation, err := m.MkSequencePrefix(x, x)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "reflexive-suffix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-reflexive-suffix-x")
			operation, err := m.MkSequenceSuffix(x, x)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "reflexive-contains", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-reflexive-contains-x")
			operation, err := m.MkSequenceContains(x, x)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "replace-pattern-with-itself", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-replace-source-x")
			pattern := astTestStringConst(t, m, "sequence-exact-replace-pattern-x")
			operation, err := m.MkSequenceReplace(x, pattern, pattern)
			astTestMust(t, err)
			return operation, x
		}},
		{name: "empty-prefix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-prefix-x")
			operation, err := m.MkSequencePrefix(astTestString(t, m, ""), x)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "empty-suffix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-suffix-x")
			operation, err := m.MkSequenceSuffix(astTestString(t, m, ""), x)
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "contains-empty", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-contains-x")
			operation, err := m.MkSequenceContains(x, astTestString(t, m, ""))
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			return operation, truth
		}},
		{name: "index-empty-at-zero", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-empty-index-x")
			operation, err := m.MkSequenceIndex(x, astTestString(t, m, ""), astTestInteger(t, m, 0))
			astTestMust(t, err)
			return operation, astTestInteger(t, m, 0)
		}},
		{name: "concat-empty-identity", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-concat-x")
			operation, err := m.MkSequenceConcat(astTestString(t, m, ""), x)
			astTestMust(t, err)
			return operation, x
		}},
		{name: "negative-at", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-at-x")
			operation, err := m.MkSequenceAt(x, astTestInteger(t, m, -1))
			astTestMust(t, err)
			return operation, astTestString(t, m, "")
		}},
		{name: "nonpositive-extract", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-extract-x")
			operation, err := m.MkSequenceExtract(x, astTestInteger(t, m, 4), astTestInteger(t, m, 0))
			astTestMust(t, err)
			return operation, astTestString(t, m, "")
		}},
		{name: "negative-index", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			x := astTestStringConst(t, m, "sequence-exact-index-x")
			operation, err := m.MkSequenceIndex(x, astTestString(t, m, "needle"), astTestInteger(t, m, -2))
			astTestMust(t, err)
			return operation, astTestInteger(t, m, -1)
		}},
		{name: "nested-concat", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			left, err := m.MkSequenceConcat(astTestString(t, m, "ab"), astTestString(t, m, "cd"))
			astTestMust(t, err)
			operation, err := m.MkSequenceConcat(left, astTestString(t, m, "ef"))
			astTestMust(t, err)
			return operation, astTestString(t, m, "abcdef")
		}},
		{name: "length", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			s := astTestString(t, m, "abcd")
			operation, err := m.MkSequenceLength(s)
			astTestMust(t, err)
			return operation, astTestInteger(t, m, 4)
		}},
		{name: "prefix", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			operation, err := m.MkSequencePrefix(astTestString(t, m, "ab"), astTestString(t, m, "abcd"))
			astTestMust(t, err)
			expected, err := m.MkTrue()
			astTestMust(t, err)
			return operation, expected
		}},
		{name: "at-out-of-bounds", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			operation, err := m.MkSequenceAt(astTestString(t, m, "abcd"), astTestInteger(t, m, 99))
			astTestMust(t, err)
			return operation, astTestString(t, m, "")
		}},
		{name: "extract", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			operation, err := m.MkSequenceExtract(astTestString(t, m, "abcdef"), astTestInteger(t, m, 2), astTestInteger(t, m, 3))
			astTestMust(t, err)
			return operation, astTestString(t, m, "cde")
		}},
		{name: "index", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			operation, err := m.MkSequenceIndex(astTestString(t, m, "ababa"), astTestString(t, m, "ba"), astTestInteger(t, m, 2))
			astTestMust(t, err)
			return operation, astTestInteger(t, m, 3)
		}},
		{name: "replace", build: func(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle) {
			operation, err := m.MkSequenceReplace(astTestString(t, m, "ababa"), astTestString(t, m, "ba"), astTestString(t, m, "X"))
			astTestMust(t, err)
			return operation, astTestString(t, m, "aXba")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, polarity := range []bool{true, false} {
				m, modelConfig := smtModelGeneratorFixture(t)
				operation, expected := test.build(t, m)
				assertion, err := m.MkEq(operation, expected)
				astTestMust(t, err)
				want := lbool.True
				if !polarity {
					assertion, err = m.MkNot(assertion)
					astTestMust(t, err)
					want = lbool.False
				}
				solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
				astTestMust(t, err)
				astTestMust(t, solver.Assert(assertion))
				status, err := solver.CheckSat(nil)
				astTestMust(t, err)
				astTestMust(t, solver.Close())
				if status != want {
					t.Fatal("exact ground sequence operation", polarity, status)
				}
			}
		})
	}
}

func TestSMTSequenceTheoryIntegerStringLanguageAxioms(t *testing.T) {
	m := astTestManager(t)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	integerSort, err := m.GetSort(zero)
	astTestMust(t, err)
	n, err := m.MkConst(astTestSymbol(t, m, "sequence-language-integer"), integerSort)
	astTestMust(t, err)
	converted, err := m.MkIntToString(n)
	astTestMust(t, err)
	nonnegative, err := m.MkArithmeticGE(n, zero)
	astTestMust(t, err)
	numeric, err := theory.numericStringMembership(converted)
	astTestMust(t, err)
	guard, err := m.MkImplies(nonnegative, numeric)
	astTestMust(t, err)
	astTestMust(t, theory.addIntToStringAxioms(converted))
	if _, found := theory.axioms.set[guard]; !found {
		t.Fatal("int.to.str lacks its nonnegative numeric-language axiom")
	}

	stringSort, err := m.GetSort(converted)
	astTestMust(t, err)
	s, err := m.MkConst(astTestSymbol(t, m, "sequence-language-string"), stringSort)
	astTestMust(t, err)
	parsed, err := m.MkStringToInt(s)
	astTestMust(t, err)
	parsedNonnegative, err := m.MkArithmeticGE(parsed, zero)
	astTestMust(t, err)
	stringNumeric, err := theory.numericStringMembership(s)
	astTestMust(t, err)
	equivalence, err := m.MkIff(parsedNonnegative, stringNumeric)
	astTestMust(t, err)
	astTestMust(t, theory.addStringToIntAxioms(parsed))
	if _, found := theory.axioms.set[equivalence]; !found {
		t.Fatal("str.to.int lacks its exact numeric-language axiom")
	}
}
