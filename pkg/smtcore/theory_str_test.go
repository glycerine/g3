package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringTheoryFixture(t *testing.T, m *ast.ASTManager) (*SMTStringTheory, *SMTInternalizerState) {
	t.Helper()
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTStringTheory(family, nil)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	arithmeticFamily, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	arithmetic, err := NewSMTTheoryArith(arithmeticFamily, true)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(arithmetic))
	astTestMust(t, state.RegisterTheory(theory))
	return theory, state
}

func TestSMTStringTheoryTypedAttachmentAndAlphabet(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	value, err := m.MkString("abc")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-attachment-x"), sort)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	contains, err := m.MkSequenceContains(x, value)
	astTestMust(t, err)
	regex, err := m.MkSequenceToRegex(value)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{length, contains, regex} {
		astTestMust(t, state.Internalize(expression, false))
	}
	for _, expression := range []ast.ASTHandle{x, value} {
		if !theory.IsAttachedToVariable(state.context.enodes.get(expression)) {
			t.Fatal("String term did not acquire a string-theory variable")
		}
	}
	for _, expression := range []ast.ASTHandle{length, contains, regex} {
		if theory.IsAttachedToVariable(state.context.enodes.get(expression)) {
			t.Fatal("scalar/Boolean/regex term acquired a string-theory variable")
		}
	}
	variable, found := state.BoolVar(contains)
	if !found {
		t.Fatal("relation callback variable missing")
	}
	data, err := state.BooleanData(variable)
	astTestMust(t, err)
	if !data.IsTheoryAtom() || !data.IsENode() {
		t.Fatal("Boolean relation lost source callback/enode ownership")
	}
	if len(theory.charsetLookup) != 255 || theory.charset[0] != 'a' || theory.charset[26] != 'A' || theory.charset[52] != '0' || theory.charset[255] != 0 {
		t.Fatal("source character search order was not preserved")
	}
	for symbol := 1; symbol < 256; symbol++ {
		index, found := theory.charsetLookup[byte(symbol)]
		if !found || theory.charset[index] != byte(symbol) {
			t.Fatal("source alphabet omitted or duplicated a byte", symbol)
		}
	}
	if _, found := theory.charsetLookup[0]; found {
		t.Fatal("NUL entered the source 255-character search alphabet")
	}
	astTestMust(t, theory.setUpAxioms(x))
	if status, err := theory.FinalCheck(); err != nil || status != SMTFinalCheckContinue {
		t.Fatal("unassigned String variable did not continue into tester assignment", status, err)
	}
}

func TestSMTStringTheoryPrivateClassesAndScopeRollback(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	value, err := m.MkString("root")
	astTestMust(t, err)
	sort, err := m.GetSort(value)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-private-class-x"), sort)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(x, false))
	astTestMust(t, state.Internalize(value, false))
	xNode, valueNode := state.context.enodes.get(x), state.context.enodes.get(value)
	for repeat := 0; repeat < 3; repeat++ {
		astTestMust(t, state.PushScope())
		astTestMust(t, theory.NewEquality(xNode.TheoryVar(theory.FamilyID()), valueNode.TheoryVar(theory.FamilyID())))
		if xNode.Root() == valueNode.Root() {
			t.Fatal("fixture accidentally relied on a core egraph merge")
		}
		got, concrete, err := theory.eqcStringValue(x)
		astTestMust(t, err)
		if !concrete || got != value {
			t.Fatal("model lookup ignored the private string-theory class")
		}
		child, err := theory.stringLiteral("cached-child")
		astTestMust(t, err)
		astTestMust(t, state.Internalize(child, false))
		before := theory.NumVariables()
		astTestMust(t, state.PopScopes(1))
		if theory.NumVariables() >= before || theory.find.GetNumVars() != uint32(theory.NumVariables()) {
			t.Fatal("string variables and private union-find diverged on pop")
		}
		_, concrete, err = theory.eqcStringValue(x)
		astTestMust(t, err)
		if concrete {
			t.Fatal("private class equality survived child scope")
		}
		again, err := theory.stringLiteral("cached-child")
		astTestMust(t, err)
		if child != again {
			t.Fatal("source lifetime string cache was incorrectly scoped")
		}
		for _, pending := range theory.basicTodo {
			if pending.InternalScopeLevel() > state.scopeLevel {
				t.Fatal("basic axiom queue retained a child enode")
			}
		}
	}
}

func TestSMTStringTheoryBasicLengthZeroEquivalence(t *testing.T) {
	for _, lengthFirst := range []bool{true, false} {
		m := astTestManager(t)
		theory, state := stringTheoryFixture(t, m)
		empty, err := m.MkString("")
		astTestMust(t, err)
		sort, err := m.GetSort(empty)
		astTestMust(t, err)
		x, err := m.MkConst(astTestSymbol(t, m, "str-empty-length-x"), sort)
		astTestMust(t, err)
		astTestMust(t, state.Internalize(x, false))
		astTestMust(t, theory.instantiateBasicStringAxioms(state.context.enodes.get(x)))
		length, err := m.MkSequenceLength(x)
		astTestMust(t, err)
		zero, err := m.ArithmeticIntegerNumeralText("0")
		astTestMust(t, err)
		lengthZero, err := m.MkEq(length, zero)
		astTestMust(t, err)
		isEmpty, err := m.MkEq(x, empty)
		astTestMust(t, err)
		first, second := lengthZero, isEmpty
		if !lengthFirst {
			first, second = second, first
		}
		literal, err := state.Literal(first)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		conclusion, err := state.Literal(second)
		astTestMust(t, err)
		assignment, err := state.Assignment(conclusion)
		astTestMust(t, err)
		if assignment != lbool.True || state.Inconsistent() {
			t.Fatal("native empty/length-zero equivalence did not propagate both directions", lengthFirst, assignment)
		}
	}
}

func TestSMTStringTheoryModelFactoryConcatAndFallback(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state := stringTheoryFixture(t, m)
	generator, err := NewSMTModelGenerator(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, generator.Close()) })
	astTestMust(t, generator.SetContext(state.context))
	astTestMust(t, generator.InitializeModel(config))
	astTestMust(t, theory.InitializeModel(generator))
	left, err := m.MkString("ab")
	astTestMust(t, err)
	right, err := m.MkString("cd")
	astTestMust(t, err)
	sort, err := m.GetSort(left)
	astTestMust(t, err)
	defaultValue, err := generator.SomeValue(sort)
	astTestMust(t, err)
	if text, _, err := m.SequenceStringValue(defaultValue); err != nil || text != "some value" {
		t.Fatal("string theory did not install its native model factory", text, err)
	}
	x, err := m.MkConst(astTestSymbol(t, m, "str-model-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "str-model-y"), sort)
	astTestMust(t, err)
	concat, err := m.MkSequenceConcat(x, right)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{concat, left, y} {
		astTestMust(t, state.Internalize(expression, false))
	}
	xNode, leftNode := state.context.enodes.get(x), state.context.enodes.get(left)
	astTestMust(t, theory.NewEquality(xNode.TheoryVar(theory.FamilyID()), leftNode.TheoryVar(theory.FamilyID())))
	for _, test := range []struct {
		expression ast.ASTHandle
		want       string
	}{{concat, "abcd"}, {y, "**UNUSED**0"}, {y, "**UNUSED**1"}} {
		procedure, err := theory.ModelValue(state.context.enodes.get(test.expression), generator)
		astTestMust(t, err)
		value, err := procedure.MakeValue(generator, nil)
		astTestMust(t, err)
		text, constant, err := m.SequenceStringValue(value)
		astTestMust(t, err)
		if !constant || text != test.want {
			t.Fatal("native string model value", text, test.want)
		}
	}
}

func TestSMTStringTheoryLiteralCacheParameterAndFreshOwner(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	theory, state := stringTheoryFixture(t, m)
	theory.parameterOwner = parameters
	first, err := theory.stringLiteral("persistent-cache")
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	again, err := theory.stringLiteral("persistent-cache")
	astTestMust(t, err)
	astTestMust(t, state.PopScopes(1))
	if first != again || theory.cacheAccesses != 2 || len(theory.strings) != 1 {
		t.Fatal("source lifetime cache or access count mismatch")
	}
	key := "str.string_constant_cache"
	astTestMust(t, config.Context.SetBoolName(config.Params, &key, false))
	astTestMust(t, parameters.Update(config.Params))
	_, err = theory.stringLiteral("uncached-value")
	astTestMust(t, err)
	if theory.cacheAccesses != 2 || len(theory.strings) != 1 {
		t.Fatal("borrowed parameter update did not disable the cache")
	}
	fresh, err := theory.Fresh()
	astTestMust(t, err)
	defer func() { astTestMust(t, fresh.Close()) }()
	if fresh.parameterOwner != parameters || fresh.cacheAccesses != 0 || fresh.unusedID != 0 || len(fresh.strings) != 0 || fresh.find.GetNumVars() != 0 {
		t.Fatal("source fresh owner copied mutable search/cache state")
	}
}

func TestSMTStringTheoryConcatAxiomsAndSyntacticEvaluation(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	left, err := m.MkString("ab")
	astTestMust(t, err)
	right, err := m.MkString("cd")
	astTestMust(t, err)
	concat, err := m.MkSequenceConcat(left, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(concat, false))
	astTestMust(t, theory.instantiateConcatAxiom(state.context.enodes.get(concat)))
	astTestMust(t, theory.tryEvalConcat(state.context.enodes.get(concat)))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	got, concrete, err := theory.eqcStringValue(concat)
	astTestMust(t, err)
	text, _, err := m.SequenceStringValue(got)
	astTestMust(t, err)
	if !concrete || text != "abcd" {
		t.Fatal("constant concatenation was not evaluated", text)
	}
	sort, err := m.GetSort(left)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-eval-symbolic-x"), sort)
	astTestMust(t, err)
	symbolic, err := m.MkSequenceConcat(x, right)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(symbolic, false))
	xNode, leftNode := state.context.enodes.get(x), state.context.enodes.get(left)
	astTestMust(t, theory.NewEquality(xNode.TheoryVar(theory.FamilyID()), leftNode.TheoryVar(theory.FamilyID())))
	before := len(theory.refs)
	astTestMust(t, theory.tryEvalConcat(state.context.enodes.get(symbolic)))
	if len(theory.refs) != before {
		t.Fatal("syntactic evaluation exposed an axiom from a model-only private equality")
	}
	_, resolved, err := theory.modelStringValue(symbolic)
	astTestMust(t, err)
	if !resolved {
		t.Fatal("model helper lost its stronger private-class evaluation")
	}
}

func TestSMTStringTheoryEqualityLengthAxiomKeepsPremise(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	empty, err := m.MkString("")
	astTestMust(t, err)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "str-guarded-length-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "str-guarded-length-y"), sort)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(x, false))
	astTestMust(t, state.Internalize(y, false))
	astTestMust(t, theory.instantiateStringEqualityLengthAxiom(state.context.enodes.get(x), state.context.enodes.get(y)))
	leftLength, err := m.MkSequenceLength(x)
	astTestMust(t, err)
	rightLength, err := m.MkSequenceLength(y)
	astTestMust(t, err)
	lengthEquality, err := m.MkEq(leftLength, rightLength)
	astTestMust(t, err)
	conclusion, err := state.Literal(lengthEquality)
	astTestMust(t, err)
	if value, err := state.Assignment(conclusion); err != nil || value != lbool.Undef {
		t.Fatal("length equality became unconditional", value, err)
	}
	equality, err := m.MkEq(x, y)
	astTestMust(t, err)
	premise, err := state.Literal(equality)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AssignLiteral(conclusion.Negated(), NewSMTBooleanAxiom()))
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if value, err := state.Assignment(premise); err != nil || value != lbool.False || state.Inconsistent() {
		t.Fatal("unequal lengths did not refuse the string equality", value, err)
	}
	astTestMust(t, state.PopScopes(1))
	if value, err := state.Assignment(premise); err != nil || value != lbool.Undef {
		t.Fatal("guard refusal survived the child scope", value, err)
	}
}

func TestSMTStringTheoryConstantLengthCountsBytes(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	value, err := m.MkString(string([]byte{0, 255, 'a', 'b'}))
	astTestMust(t, err)
	astTestMust(t, state.Internalize(value, false))
	if !theory.CanPropagate() {
		t.Fatal("string constant was not queued for eager basic axioms")
	}
	astTestMust(t, theory.Propagate())
	if theory.CanPropagate() {
		t.Fatal("basic constant queue did not drain")
	}
	length, err := m.MkSequenceLength(value)
	astTestMust(t, err)
	four, err := m.ArithmeticIntegerNumeralText("4")
	astTestMust(t, err)
	equality, err := m.MkEq(length, four)
	astTestMust(t, err)
	literal, err := state.Literal(equality)
	astTestMust(t, err)
	if assignment, err := state.Assignment(literal); err != nil || assignment != lbool.True {
		t.Fatal("native constant-length unit did not count the full byte string", assignment, err)
	}
}
