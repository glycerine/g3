package smtcore

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringBinaryCaseSplitTruthAndLiteralSideEffects(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "binary-cases")
	tester, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	for _, text := range []string{"-1", "0", "3", "18446744073709551617"} {
		mid, ok := new(big.Int).SetString(text, 10)
		if !ok {
			t.Fatal(text)
		}
		root, err := theory.generateStringBinaryCaseSplit(word, tester, &smtStringBinaryBounds{lower: smtQZero(), upper: smtQZero(), window: smtQZero(), midpoint: *theory.testRational(mid)})
		astTestMust(t, err)
		formula, err := m.RefGet(root)
		astTestMust(t, err)
		length, err := m.MkSequenceLength(word)
		astTestMust(t, err)
		for _, label := range []string{"less", "more", text, "junk"} {
			if label != "junk" {
				choice, err := state.mkEqualityAtom(tester, astTestString(t, m, label))
				astTestMust(t, err)
				if _, ok := state.BoolVar(choice); !ok {
					t.Fatal("unused literal collector lost Boolean internalization", label)
				}
			}
			for offset := int64(-2); offset <= 2; offset++ {
				n := new(big.Int).Add(mid, big.NewInt(offset))
				number, err := m.ArithmeticIntegerNumeralText(n.String())
				astTestMust(t, err)
				want := label == "less" && offset < 0 || label == "more" && offset > 0 || label == text && offset == 0
				if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{tester: astTestString(t, m, label), length: number}); got != want {
					t.Fatal("binary case split", text, label, offset, got, want)
				}
			}
		}
		for _, ref := range theory.refs {
			term, err := m.RefGet(ref)
			astTestMust(t, err)
			if term == formula {
				t.Fatal("case-split helper acquired driver lifetime root")
			}
		}
		astTestMust(t, m.DelRef(&root))
	}
	if _, err := theory.generateStringBinaryCaseSplit(word, tester, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("nil bounds accepted", err)
	}
}

func TestSMTStringBinaryWindowsExpansionAndImmutableBounds(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "binary-expand")
	root, err := theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.binaryStacks[word][0]
	initial := theory.testers.binaryBounds[first]
	for _, want := range []string{"48", "56", "60", "62", "63", "64", "96"} {
		stack := theory.testers.binaryStacks[word]
		last := stack[len(stack)-1]
		root, err = theory.generateStringBinaryLengthOptions(word, last, "more")
		astTestMust(t, err)
		formula, err := m.RefGet(root)
		astTestMust(t, err)
		owned := false
		for _, ref := range theory.refs {
			term, err := m.RefGet(ref)
			astTestMust(t, err)
			if term == formula {
				owned = true
			}
		}
		if !owned {
			t.Fatal("driver lost source lifetime formula root")
		}
		astTestMust(t, m.DelRef(&root))
		stack = theory.testers.binaryStacks[word]
		bounds := theory.testers.binaryBounds[stack[len(stack)-1]]
		if theory.testBig(&bounds.midpoint).String() != want {
			t.Fatal("binary midpoint", bounds.midpoint, want)
		}
		if want == "96" && (theory.testBig(&bounds.lower).String() != "65" || theory.testBig(&bounds.upper).String() != "128" || theory.testBig(&bounds.window).String() != "128") {
			t.Fatal("upper-window expansion changed", bounds)
		}
	}
	if initial.lower.Sign() != 0 || theory.testBig(&initial.midpoint).String() != "32" || theory.testBig(&initial.upper).String() != "64" || theory.testBig(&initial.window).String() != "64" {
		t.Fatal("child update mutated prior bounds")
	}
	if len(theory.testers.lengthIndicators[word]) != 0 || len(theory.testers.lengthSubjects) != 0 {
		t.Fatal("binary path invented sequential reverse/history registration")
	}
}

func TestSMTStringBinaryScopeRollbackAndPersistentTesterReuse(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "binary-scopes")
	astTestMust(t, state.PushScope())
	root, err := theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.binaryStacks[word][0]
	queued := 0
	for _, node := range theory.basicTodo {
		if node.Owner() == first {
			queued++
		}
	}
	if queued != 2 {
		t.Fatal("initial creation lost native extra refresh")
	}
	astTestMust(t, state.PushScope())
	root, err = theory.generateStringBinaryLengthOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	high := theory.testers.binaryHigh[first]
	astTestMust(t, state.PopScopes(1))
	if len(theory.testers.binaryStacks[word]) != 1 || theory.testers.binaryBounds[high] != nil || theory.testers.binaryHigh[first] != high {
		t.Fatal("scope rollback erased cache or retained active child bounds")
	}
	fresh := theory.freshID
	astTestMust(t, state.PushScope())
	root, err = theory.generateStringBinaryLengthOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || theory.testers.binaryStacks[word][1] != high {
		t.Fatal("cached high tester not reused")
	}
	astTestMust(t, state.PopScopes(1))
	astTestMust(t, state.PushScope())
	root, err = theory.generateStringBinaryLengthOptions(word, first, "less")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	low := theory.testers.binaryLow[first]
	if theory.testBig(&theory.testers.binaryBounds[low].midpoint).String() != "15" {
		t.Fatal("lower branch midpoint changed")
	}
	astTestMust(t, state.PopScopes(2))
	if len(theory.testers.binaryStacks[word]) != 0 || len(theory.testers.binaryBounds) != 0 || theory.testers.binaryStart[word] != first || theory.testers.binaryLow[first] != low {
		t.Fatal("nested rollback/cache lifetime changed")
	}
	fresh = theory.freshID
	root, err = theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || theory.testers.binaryStacks[word][0] != first {
		t.Fatal("initial tester cache not reused after empty stack")
	}
}

func TestSMTStringBinaryNegativeMidpointAndConcreteValue(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "binary-negative")
	root, err := theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	for _, want := range []string{"15", "7", "3", "1", "0", "-1"} {
		stack := theory.testers.binaryStacks[word]
		last := stack[len(stack)-1]
		root, err = theory.generateStringBinaryLengthOptions(word, last, "less")
		astTestMust(t, err)
		astTestMust(t, m.DelRef(&root))
		stack = theory.testers.binaryStacks[word]
		if got := theory.testBig(&theory.testers.binaryBounds[stack[len(stack)-1]].midpoint).String(); got != want {
			t.Fatal("negative floor midpoint", got, want)
		}
	}
	stack := theory.testers.binaryStacks[word]
	last := stack[len(stack)-1]
	root, err = theory.generateStringBinaryLengthOptions(word, last, "-1")
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(word)
	astTestMust(t, err)
	for _, text := range []string{"-1", "0", "1"} {
		number, err := m.ArithmeticIntegerNumeralText(text)
		astTestMust(t, err)
		if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{length: number}); got != (text != "-1") {
			t.Fatal("negative midpoint guard", text, got)
		}
	}
	astTestMust(t, m.DelRef(&root))
	if len(theory.testers.valueIndicators[word]) != 0 {
		t.Fatal("negative midpoint started value testing")
	}
	other := stringGroundingWord(t, theory, "binary-concrete")
	root, err = theory.generateStringBinaryLengthOptions(other, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	last = theory.testers.binaryStacks[other][0]
	root, err = theory.generateStringBinaryLengthOptions(other, last, "32")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(theory.testers.valueIndicators[other][32]) != 1 {
		t.Fatal("concrete callback did not start value enumeration")
	}
	// Assigned private values take precedence even over a mismatched callback.
	// Unlike sequential dispatch, native binary dispatch does not run the
	// regex-constrained-subterm refusal before starting value testing.
	stringRegexRegister(t, theory, other, stringRegexLiteral(t, m, "abc"))
	value := astTestString(t, m, "32")
	astTestMust(t, theory.state.Internalize(value, false))
	stringDependencyPrivateMerge(t, theory, last, value)
	root, err = theory.generateStringBinaryLengthOptions(other, ast.ASTHandle{}, "more")
	astTestMust(t, err)
	if root != (ast.ASTRef{}) {
		t.Fatal("assigned length should replay its existing value tester without a separate return")
	}
}

func TestSMTStringBinaryKnownBigLengthAndDispatchGuards(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	key := "str.use_binary_search"
	astTestMust(t, context.SetBoolName(&params, &key, true))
	key = "str.binary_search_start"
	astTestMust(t, context.SetUIntName(&params, &key, 3))
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	word := stringGroundingWord(t, theory, "binary-big")
	stringSetLength(t, m, state, word, "18446744073709551617")
	root, err := theory.generateStringLengthValueOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.binaryStacks[word][0]
	bounds := theory.testers.binaryBounds[first]
	if theory.testBig(&bounds.midpoint).String() != "18446744073709551617" || theory.testBig(&bounds.upper).String() != "36893488147419103234" {
		t.Fatal("known arbitrary-precision initial length lost")
	}
	decl, err := m.GetDecl(first)
	astTestMust(t, err)
	symbol, err := m.GetASTName(decl)
	astTestMust(t, err)
	name, _, err := m.Symbols.BareString(symbol)
	astTestMust(t, err)
	if name != "$$_len_binary-big_1_0!tmp0" {
		t.Fatal("known length applied before native initial name", name)
	}
	fresh := theory.freshID
	if _, err := theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "more"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unexpected callback reordering accepted", err)
	}
	if _, err := theory.generateStringBinaryLengthOptions(word, first, "more"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("out-of-int32 fresh midpoint accepted", err)
	}
	if theory.freshID != fresh || len(theory.testers.binaryStacks[word]) != 1 {
		t.Fatal("guard failure changed tester state")
	}
	params, err = context.NewValue()
	astTestMust(t, err)
	key = "str.use_binary_search"
	astTestMust(t, context.SetBoolName(&params, &key, false))
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	other := stringGroundingWord(t, theory, "sequential-dispatch")
	root, err = theory.generateStringLengthValueOptions(other, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(theory.testers.lengthIndicators[other]) != 1 || len(theory.testers.binaryStacks[other]) != 0 {
		t.Fatal("sequential dispatch changed")
	}
	freshOwner, err := theory.Fresh()
	astTestMust(t, err)
	defer func() { astTestMust(t, freshOwner.Close()) }()
	if len(freshOwner.testers.binaryStacks) != 0 || len(freshOwner.testers.binaryBounds) != 0 || len(freshOwner.testers.binaryStart) != 0 || len(freshOwner.testers.binaryTrail) != 0 {
		t.Fatal("Fresh copied binary search state")
	}
}

func TestSMTStringBinaryCachedSuccessorAllowsBigUpdatedWindow(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	word := stringGroundingWord(t, theory, "binary-cached-big")
	astTestMust(t, state.PushScope())
	root, err := theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.binaryStart[word]
	root, err = theory.generateStringBinaryLengthOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	high := theory.testers.binaryHigh[first]
	astTestMust(t, state.PopScopes(1))
	astTestMust(t, state.PushScope())
	stringSetLength(t, m, state, word, "18446744073709551617")
	fresh := theory.freshID
	root, err = theory.generateStringBinaryLengthOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	root, err = theory.generateStringBinaryLengthOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || theory.testers.binaryStacks[word][1] != high || theory.testBig(&theory.testers.binaryBounds[high].midpoint).String() != "27670116110564327426" {
		t.Fatal("cached tester incorrectly subjected to fresh int32 name conversion")
	}
	astTestMust(t, state.PopScopes(1))
}
