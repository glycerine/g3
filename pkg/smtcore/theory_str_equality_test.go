package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringEqualityConcat(t *testing.T, m *ast.ASTManager, parts ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	value := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		var err error
		value, err = m.MkSequenceConcat(parts[i], value)
		astTestMust(t, err)
	}
	astTestPin(t, m, value)
	return value
}

func TestSMTStringEqualitySyntacticFeasibility(t *testing.T) {
	m := astTestManager(t)
	theory, _ := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-feasible-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "str-feasible-y"), sort)
	astTestMust(t, err)
	a, b, empty := astTestString(t, m, "a"), astTestString(t, m, "b"), astTestString(t, m, "")
	for _, test := range []struct {
		left, right ast.ASTHandle
		possible    bool
	}{
		{a, a, true}, {a, b, false}, {x, a, true},
		{stringEqualityConcat(t, m, a, x), b, false},
		{stringEqualityConcat(t, m, x, a), b, false},
		{stringEqualityConcat(t, m, a, x, a), a, false},
		{stringEqualityConcat(t, m, x, a, y), b, false},
		{stringEqualityConcat(t, m, a, x), stringEqualityConcat(t, m, b, y), false},
		{stringEqualityConcat(t, m, x, a), stringEqualityConcat(t, m, y, b), false},
		{stringEqualityConcat(t, m, a, x), stringEqualityConcat(t, m, astTestString(t, m, "ab"), y), true},
		{stringEqualityConcat(t, m, empty, b, x, a, empty), astTestString(t, m, "ab"), true},
	} {
		for _, pair := range [][2]ast.ASTHandle{{test.left, test.right}, {test.right, test.left}} {
			possible, err := theory.canTwoNodesEqual(pair[0], pair[1])
			astTestMust(t, err)
			if possible != test.possible {
				t.Fatal("source feasibility filter changed", possible, test.possible)
			}
		}
	}
}

func stringMixedEqualityFixture(t *testing.T, m *ast.ASTManager) (*SMTStringTheory, *SMTInternalizerState, *SMTTheoryArith) {
	t.Helper()
	_, _, state := smtInternalizerFixture(t, m, nil)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	arithmetic, err := NewSMTTheoryArith(family, false)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(arithmetic))
	family, err = m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTStringTheory(family, nil)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	return theory, state, arithmetic
}

func stringSetLength(t *testing.T, m *ast.ASTManager, state *SMTInternalizerState, expression ast.ASTHandle, number string) ast.ASTHandle {
	t.Helper()
	length, err := m.MkSequenceLength(expression)
	astTestMust(t, err)
	numeral, err := m.ArithmeticIntegerNumeralText(number)
	astTestMust(t, err)
	equality, err := m.MkEq(length, numeral)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	return length
}

func TestSMTStringLengthReaderRequiresMixedAndCoreRoots(t *testing.T) {
	m := astTestManager(t)
	integerTheory, _ := stringTheoryFixture(t, m)
	if _, known, err := integerTheory.lengthValue(astTestString(t, m, "ab")); err != nil || known {
		t.Fatal("source mixed-arithmetic cast gate was bypassed", known, err)
	}
	// Use another manager/context so its arithmetic owner is truly mixed.
	m = astTestManager(t)
	theory, state, arithmetic := stringMixedEqualityFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-root-only-length"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(length, false))
	variable := state.context.enodes.get(length).TheoryVar(arithmetic.FamilyID())
	arithmetic.value[variable].rational.SetInt64(7) // a candidate value, not a bound
	if _, known, err := theory.lengthValue(x); err != nil || known {
		t.Fatal("candidate arithmetic value became a length premise", known, err)
	}
	astTestMust(t, state.PushScope())
	stringSetLength(t, m, state, x, "7")
	value, known, err := theory.lengthValue(x)
	astTestMust(t, err)
	if !known || value.String() != "7" {
		t.Fatal("actual core numeral root was not read", value, known)
	}
	word := stringEqualityConcat(t, m, astTestString(t, m, "ab"), x)
	value, known, err = theory.lengthValue(word)
	astTestMust(t, err)
	if !known || value.String() != "9" {
		t.Fatal("concat length did not sum its known leaves", value, known)
	}
	astTestMust(t, state.PopScopes(1))
	if _, known, err := theory.lengthValue(x); err != nil || known {
		t.Fatal("popped numeral root remained a length premise", known, err)
	}
	y, err := m.MkConst(astTestSymbol(t, m, "str-unknown-length-leaf"), sort)
	astTestMust(t, err)
	unknownWord := stringEqualityConcat(t, m, x, y)
	wholeLength := stringSetLength(t, m, state, unknownWord, "11")
	whole, known, err := theory.arithmeticValue(wholeLength)
	astTestMust(t, err)
	if !known || whole.String() != "11" {
		t.Fatal("fixture did not establish the whole concat length root")
	}
	if _, known, err := theory.lengthValue(unknownWord); err != nil || known {
		t.Fatal("source concat-leaf traversal was replaced by a whole-word value", known, err)
	}
}

func TestSMTStringLengthConsistencyGuardedRefusalAndPop(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state, _ := stringMixedEqualityFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-length-guard-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "str-length-guard-y"), sort)
	astTestMust(t, err)
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	xLength := stringSetLength(t, m, state, x, "3")
	yLength := stringSetLength(t, m, state, y, "4")
	consistent, err := theory.checkLengthConsistency(x, y)
	astTestMust(t, err)
	if consistent {
		t.Fatal("different established lengths were accepted")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	five, err := m.ArithmeticIntegerNumeralText("5")
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	if !stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{xLength: five, yLength: five, equality: truth}) {
		t.Fatal("refusal lost the length premises needed in another branch")
	}
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if value, err := state.Assignment(literal); err != nil || value != lbool.False {
		t.Fatal("guarded length contradiction did not refuse equality", value, err)
	}
	astTestMust(t, state.PopScopes(1))
	if state.Inconsistent() {
		t.Fatal("child length refusal survived rollback")
	}
	if value, err := state.Assignment(literal); err != nil || value != lbool.Undef {
		t.Fatal("conditional refusal became a permanent parent disequality", value, err)
	}
	astTestMust(t, state.PushScope())
	stringSetLength(t, m, state, x, "5")
	stringSetLength(t, m, state, y, "5")
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if state.Inconsistent() {
		t.Fatal("refused child lengths poisoned a later compatible branch")
	}
	astTestMust(t, state.PopScopes(1))
}

func TestSMTStringEqualityChecksClassMembersBeforeMerge(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	var variables []ast.ASTHandle
	for _, name := range []string{"str-before-a", "str-before-b", "str-before-x", "str-before-y"} {
		variable, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		variables = append(variables, variable)
	}
	left := stringEqualityConcat(t, m, astTestString(t, m, "a"), variables[2])
	right := stringEqualityConcat(t, m, astTestString(t, m, "b"), variables[3])
	for _, pair := range [][2]ast.ASTHandle{{variables[0], left}, {variables[1], right}} {
		equality, err := m.MkEq(pair[0], pair[1])
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
	}
	astTestMust(t, state.PushScope())
	equality, err := m.MkEq(variables[0], variables[1])
	astTestMust(t, err)
	astTestMust(t, state.Internalize(equality, true))
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if !state.Inconsistent() {
		t.Fatal("incompatible concat members were missed by cross-class checking")
	}
	a := uint32(state.context.enodes.get(variables[0]).TheoryVar(theory.FamilyID()))
	b := uint32(state.context.enodes.get(variables[1]).TheoryVar(theory.FamilyID()))
	if theory.find.Find(a) != theory.find.Find(b) {
		t.Fatal("source post-handler merge was skipped after conflict")
	}
	astTestMust(t, state.PopScopes(1))
	if state.Inconsistent() || theory.find.Find(a) == theory.find.Find(b) {
		t.Fatal("class conflict or private merge survived the child scope")
	}
}

func TestSMTStringLengthConsistencyConcatCases(t *testing.T) {
	m := astTestManager(t)
	theory, state, _ := stringMixedEqualityFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	var variables []ast.ASTHandle
	for _, name := range []string{"str-sum-x", "str-sum-y", "str-sum-z", "str-sum-w"} {
		expression, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(expression, false))
		variables = append(variables, expression)
	}
	x, y, z, w := variables[0], variables[1], variables[2], variables[3]
	stringSetLength(t, m, state, x, "3")
	stringSetLength(t, m, state, y, "4")
	a, b := astTestString(t, m, "a"), astTestString(t, m, "b")
	for _, test := range []struct {
		left, right ast.ASTHandle
		consistent  bool
	}{
		{a, b, true}, // literal inequality belongs to the value filter.
		{x, astTestString(t, m, "ab"), false},
		{x, astTestString(t, m, "abc"), true},
		{z, astTestString(t, m, "ab"), true},
		{stringEqualityConcat(t, m, a, x), astTestString(t, m, "abc"), false},
		{stringEqualityConcat(t, m, a, z), astTestString(t, m, "abc"), true},
		{stringEqualityConcat(t, m, a, astTestString(t, m, "c")), astTestString(t, m, "abc"), true},
		{stringEqualityConcat(t, m, a, x), stringEqualityConcat(t, m, a, y), false},
		{stringEqualityConcat(t, m, y, z), stringEqualityConcat(t, m, a, b), false},
		{stringEqualityConcat(t, m, x, z), stringEqualityConcat(t, m, y, w), true},
		{stringEqualityConcat(t, m, x, z), y, true},
		{stringEqualityConcat(t, m, y, z), x, false},
	} {
		for _, pair := range [][2]ast.ASTHandle{{test.left, test.right}, {test.right, test.left}} {
			consistent, err := theory.checkLengthConsistency(pair[0], pair[1])
			astTestMust(t, err)
			if consistent != test.consistent {
				t.Fatal("source known/partial length case changed", consistent, test.consistent)
			}
		}
	}
}

func TestSMTStringLengthConcatVariableZeroDoesNotWeakenRefusal(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state, _ := stringMixedEqualityFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	var variables []ast.ASTHandle
	for _, name := range []string{"str-zero-piece", "str-long-piece", "str-short-target"} {
		expression, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		variables = append(variables, expression)
	}
	zeroLength := stringSetLength(t, m, state, variables[0], "0")
	longLength := stringSetLength(t, m, state, variables[1], "5")
	targetLength := stringSetLength(t, m, state, variables[2], "3")
	word := stringEqualityConcat(t, m, variables[0], variables[1])
	consistent, err := theory.checkLengthConsistency(word, variables[2])
	astTestMust(t, err)
	if consistent {
		t.Fatal("mandatory nonzero piece exceeded the target without refusal")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	equality, err := m.MkEq(word, variables[2])
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	five, err := m.ArithmeticIntegerNumeralText("5")
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	if stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{zeroLength: two, longLength: five, targetLength: three, equality: truth}) {
		t.Fatal("zero-length premise weakened a refusal that needs only nonnegativity")
	}
}
