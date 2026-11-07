package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func stringTesterNewEquality(t *testing.T, theory *SMTStringTheory, left, right ast.ASTHandle) {
	t.Helper()
	state := theory.state
	astTestMust(t, state.Internalize(left, false))
	astTestMust(t, state.Internalize(right, false))
	astTestMust(t, theory.NewEquality(state.context.enodes.get(left).TheoryVar(theory.FamilyID()), state.context.enodes.get(right).TheoryVar(theory.FamilyID())))
}

func TestSMTStringTesterCallbacksBeforeMergeBothOrientations(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			m, theory, _, _ := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "callback-order")
			root, err := theory.generateStringLengthValueOptions(word, ast.ASTHandle{}, "")
			astTestMust(t, err)
			astTestMust(t, m.DelRef(&root))
			first := theory.testers.lengthIndicators[word][0]
			more := astTestString(t, m, "more")
			left, right := first, more
			if reverse {
				left, right = right, left
			}
			stringTesterNewEquality(t, theory, left, right)
			if len(theory.testers.lengthIndicators[word]) != 2 {
				t.Fatal("premerge more callback did not advance length window")
			}
			value, assigned, err := theory.eqcStringValue(first)
			astTestMust(t, err)
			if !assigned || value != more {
				t.Fatal("callback equality not merged afterward")
			}
			second := theory.testers.lengthIndicators[word][1]
			choice := astTestString(t, m, "3")
			left, right = second, choice
			if reverse {
				left, right = right, left
			}
			stringTesterNewEquality(t, theory, left, right)
			entries := theory.testers.valueIndicators[word][3]
			if len(entries) != 1 {
				t.Fatal("concrete length callback did not start value testing")
			}
			firstValue := entries[0].variable
			left, right = firstValue, more
			if reverse {
				left, right = right, left
			}
			stringTesterNewEquality(t, theory, left, right)
			entries = theory.testers.valueIndicators[word][3]
			if len(entries) != 2 || theory.testers.valueRanges[entries[1].variable][0] != 63 {
				t.Fatal("premerge value callback did not append next batch")
			}
		})
	}
}

func TestSMTStringTesterCallbacksUseActualBooleanEqualityPropagation(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	word := stringGroundingWord(t, theory, "callback-core")
	root, err := theory.generateStringLengthValueOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, theory.assertStringTesterOptions(root))
	length := theory.testers.lengthIndicators[word][0]
	stringDependencyEquality(t, theory, length, astTestString(t, m, "1"), true)
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if state.inconsistent || len(theory.testers.valueIndicators[word][1]) != 1 {
		t.Fatal("core equality propagation did not invoke concrete length callback")
	}
	value := theory.testers.valueIndicators[word][1][0].variable
	stringDependencyEquality(t, theory, value, astTestString(t, m, "more"), true)
	_, err = state.PropagateCore()
	astTestMust(t, err)
	if state.inconsistent || len(theory.testers.valueIndicators[word][1]) != 2 {
		t.Fatal("core equality propagation did not invoke more value callback")
	}
}

func TestSMTStringTesterAttemptRolesAndSortGate(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "callback-roles")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	before := len(theory.refs)
	handled, err := theory.attemptStringFreeVariable(length, astTestString(t, m, "1"))
	astTestMust(t, err)
	if !handled || len(theory.refs) != before {
		t.Fatal("length role without reverse subject was not consumed without generation")
	}
	value, err := theory.freshStringValueTester(word, 1, 0)
	astTestMust(t, err)
	handled, err = theory.attemptStringFreeVariable(value, astTestString(t, m, "0"))
	astTestMust(t, err)
	if !handled {
		t.Fatal("concrete value callback not consumed")
	}
	handled, err = theory.attemptStringFreeVariable(value, word)
	astTestMust(t, err)
	if handled {
		t.Fatal("nonliteral value counterpart consumed")
	}
	unroll, err := theory.freshStringVariable("unroll-tester")
	astTestMust(t, err)
	theory.unrollTestVariables[unroll] = struct{}{}
	handled, err = theory.attemptStringFreeVariable(unroll, word)
	astTestMust(t, err)
	if !handled {
		t.Fatal("unroll role requires no literal counterpart")
	}
	// The source sort gate precedes role handling and all String axioms.
	integer, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	theory.lengthTestVariables[integer] = struct{}{}
	before = len(theory.refs)
	astTestMust(t, theory.handleStringEquality(integer, astTestString(t, m, "more")))
	if len(theory.refs) != before {
		t.Fatal("non-String sort entered tester/String equality processing")
	}
}

func TestSMTStringTesterBinaryCallbackAsymmetriesAndGuards(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	key := "str.use_binary_search"
	astTestMust(t, context.SetBoolName(&params, &key, true))
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	word := stringGroundingWord(t, theory, "callback-binary")
	root, err := theory.generateStringLengthValueOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	length := theory.testers.binaryStacks[word][0]
	stringTesterNewEquality(t, theory, length, astTestString(t, m, "32"))
	if len(theory.testers.valueIndicators[word]) != 0 {
		t.Fatal("binary length callback invented absent sequential reverse entry")
	}
	// Actual binary generation after the assigned length creates a value
	// tester; its later callback uses the active binary stack.
	root, err = theory.generateStringLengthValueOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	value := theory.testers.valueIndicators[word][32][0].variable
	stringTesterNewEquality(t, theory, value, astTestString(t, m, "more"))
	if len(theory.testers.valueIndicators[word][32]) != 2 {
		t.Fatal("binary more value callback did not use top assigned length")
	}
	other := stringGroundingWord(t, theory, "callback-binary-guards")
	root, err = theory.generateStringLengthValueOptions(other, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	unknown := theory.testers.binaryStacks[other][0]
	manual, err := theory.freshStringValueTester(other, 1, 0)
	astTestMust(t, err)
	theory.testers.valueSubjects[manual] = other
	before := theory.freshID
	astTestMust(t, theory.moreStringValueTests(manual, "more"))
	if theory.freshID != before {
		t.Fatal("unknown binary length warning generated a tester")
	}
	more := astTestString(t, m, "more")
	astTestMust(t, state.Internalize(more, false))
	stringDependencyPrivateMerge(t, theory, unknown, more)
	if err := theory.moreStringValueTests(manual, "more"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("nonconcrete binary top bypassed source guard", err)
	}
	delete(theory.testers.binaryStacks, other)
	if err := theory.moreStringValueTests(manual, "more"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("missing binary stack accepted", err)
	}
	left, right := state.context.enodes.get(manual).TheoryVar(theory.FamilyID()), state.context.enodes.get(more).TheoryVar(theory.FamilyID())
	if err := theory.NewEquality(left, right); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("failed callback did not propagate its guard", err)
	}
	if theory.find.Find(uint32(left)) == theory.find.Find(uint32(right)) {
		t.Fatal("failed callback merged private classes")
	}
}

func TestSMTStringTesterValueCallbackUsesInactiveAssignedLength(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "callback-inactive-length")
	root, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.lengthIndicators[word][0]
	root, err = theory.generateStringSequentialOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	second := theory.testers.lengthIndicators[word][1]
	for _, entry := range []struct {
		tester ast.ASTHandle
		text   string
	}{{first, "1"}, {second, "2"}} {
		literal := astTestString(t, m, entry.text)
		astTestMust(t, state.Internalize(literal, false))
		stringDependencyPrivateMerge(t, theory, entry.tester, literal)
	}
	delete(theory.internalVariables, first)
	root, err = theory.generateStringFreeValueOptions(word, first, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	value := theory.testers.valueIndicators[word][1][0].variable
	astTestMust(t, theory.moreStringValueTests(value, "more"))
	if len(theory.testers.valueIndicators[word][1]) != 2 || len(theory.testers.valueIndicators[word][2]) != 0 {
		t.Fatal("value callback filtered inactive assigned length or selected later entry")
	}
}
