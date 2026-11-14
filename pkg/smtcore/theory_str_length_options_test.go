package smtcore

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTStringLengthOptionsThreeWideWindowsAndInternalization(t *testing.T) {
	m, theory, state, rw := stringBackpropMixedFixture(t)
	word := stringGroundingWord(t, theory, "length-options-word")
	var previous []ast.ASTHandle
	for tries := int32(1); tries <= 3; tries++ {
		indicator, err := theory.freshStringVariable("length-indicator")
		astTestMust(t, err)
		theory.testers.lengthIndicators[word] = append(append([]ast.ASTHandle(nil), previous...), indicator)
		root, err := theory.generateStringLengthOptions(word, indicator, tries)
		astTestMust(t, err)
		formula, err := m.RefGet(root)
		astTestMust(t, err)
		length, err := m.MkSequenceLength(word)
		astTestMust(t, err)
		astTestPin(t, m, length)
		lower, upper := (tries-1)*3, tries*3
		for index := lower; index < upper; index++ {
			number, err := m.ArithmeticIntegerNumeralText(strconv.FormatInt(int64(index), 10))
			astTestMust(t, err)
			equal, err := state.mkEqualityAtom(length, number)
			astTestMust(t, err)
			if _, exists := state.BoolVar(equal); !exists {
				t.Fatal("unused case-split collection lost its live equality internalization", tries, index)
			}
		}
		for n := int32(0); n <= 10; n++ {
			for _, label := range []string{strconv.FormatInt(int64(lower), 10), strconv.FormatInt(int64(lower+1), 10), strconv.FormatInt(int64(lower+2), 10), "more", "junk"} {
				for _, priorMore := range []bool{false, true} {
					number, err := m.ArithmeticIntegerNumeralText(strconv.FormatInt(int64(n), 10))
					astTestMust(t, err)
					values := map[ast.ASTHandle]ast.ASTHandle{length: number, indicator: astTestString(t, m, label)}
					for _, prior := range previous {
						choice := "0"
						if priorMore {
							choice = "more"
						}
						values[prior] = astTestString(t, m, choice)
					}
					window := label == strconv.FormatInt(int64(n), 10) && n >= lower && n < upper || label == "more" && n >= upper
					want := len(previous) > 0 && !priorMore || window
					if got := stringAxiomTruth(t, m, rw, formula, values); got != want {
						t.Fatal("three-wide length formula changed", tries, n, label, priorMore, got, want)
					}
				}
			}
		}
		astTestMust(t, m.DelRef(&root))
		if tries == 1 {
			if _, err := m.GetKind(formula); err != nil {
				t.Fatal("first-window formula lost its native lifetime-trail root", err)
			}
		}
		previous = append(previous, indicator)
	}
}

func TestSMTStringLengthOptionsKnownBigLengthPrecedesWindowPolicy(t *testing.T) {
	m, theory, state, rw := stringRegexStageFixture(t)
	word := stringGroundingWord(t, theory, "length-options-known")
	indicator, err := theory.freshStringVariable("known-length-indicator")
	astTestMust(t, err)
	text := "18446744073709551617"
	length := stringSetLength(t, m, state, word, text)
	before := len(state.boolExpr)
	root, err := theory.generateStringLengthOptions(word, indicator, 0)
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	if len(state.boolExpr) != before || len(theory.testers.lengthLiterals) != 0 {
		t.Fatal("known-length branch generated window side effects")
	}
	for _, label := range []string{text, "more", "0"} {
		for _, same := range []bool{false, true} {
			number, ok := new(big.Int).SetString(text, 10)
			if !ok {
				t.Fatal("invalid test integer")
			}
			if !same {
				number.Add(number, big.NewInt(1))
			}
			value, err := m.ArithmeticIntegerNumeralText(number.String())
			astTestMust(t, err)
			if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{indicator: astTestString(t, m, label), length: value}); got != (label == text && same) {
				t.Fatal("known arbitrary-precision length option changed", label, same, got)
			}
		}
	}
	astTestMust(t, m.DelRef(&root))
	if _, err := m.GetKind(formula); err != nil {
		t.Fatal("known-length result lost its native lifetime-trail root", err)
	}
}

func TestSMTStringLengthOptionsPrioritiesAggressivePhasesAndLiteralCache(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	for _, key := range []string{"str.fast_length_tester_cache", "str.aggressive_length_testing"} {
		key := key
		astTestMust(t, context.SetBoolName(&params, &key, true))
	}
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	state.caseSplitStrategy = 6
	word := stringGroundingWord(t, theory, "length-options-priority")
	first, err := theory.freshStringVariable("priority-first")
	astTestMust(t, err)
	second, err := theory.freshStringVariable("priority-second")
	astTestMust(t, err)
	third, err := theory.freshStringVariable("priority-third")
	astTestMust(t, err)
	indicators := []ast.ASTHandle{first, second, third}
	theory.testers.lengthIndicators[word] = indicators
	for _, tries := range []int32{1, 2, 3} {
		current := indicators[tries-1]
		root, err := theory.generateStringLengthOptions(word, current, tries)
		astTestMust(t, err)
		astTestMust(t, m.DelRef(&root))
		for index := (tries - 1) * 3; index < tries*3; index++ {
			label := theory.testers.lengthLiterals[index]
			if label.IsNull() {
				t.Fatal("fast cache omitted a numeric String label", index)
			}
			equal, err := state.mkEqualityAtom(current, label)
			astTestMust(t, err)
			variable, exists := state.BoolVar(equal)
			if !exists {
				t.Fatal("choice was not internalized")
			}
			want := 0.2
			if index <= 5 {
				want = 0.3
			}
			if state.theoryAwareBranching.priorities[variable] != want {
				t.Fatal("small/large length priority changed", index)
			}
			data, err := state.BooleanData(variable)
			astTestMust(t, err)
			if !data.PhaseAvailable || !data.Phase || !state.isRelevant(equal) || data.TryTrueFirst() {
				t.Fatal("aggressive length choice was not relevant with a cached true phase")
			}
			if assigned, err := state.Assignment(NewSMTLiteral(variable, false)); err != nil || assigned != lbool.Undef {
				t.Fatal("phase hint assigned a literal", assigned, err)
			}
		}
		more := astTestString(t, m, "more")
		equal, err := state.mkEqualityAtom(current, more)
		astTestMust(t, err)
		variable, _ := state.BoolVar(equal)
		data, err := state.BooleanData(variable)
		astTestMust(t, err)
		if !data.PhaseAvailable || data.Phase || state.theoryAwareBranching.priorities[variable] != -0.1 {
			t.Fatal("more priority/false-phase hint changed")
		}
	}
	cached := theory.testers.lengthLiterals[0]
	count, err := m.GetRefCount(cached)
	astTestMust(t, err)
	root, err := theory.generateStringLengthOptions(word, first, 1)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.testers.lengthLiterals[0] != cached {
		t.Fatal("cache hit replaced its existing label")
	}
	after, err := m.GetRefCount(cached)
	astTestMust(t, err)
	if after != count {
		t.Fatal("cache hit added another direct lifetime root", count, after)
	}
}

func TestSMTStringLengthOptionsScopedGuardAndRetainedReturnOwnership(t *testing.T) {
	m, theory, state, rw := stringBackpropMixedFixture(t)
	word := stringGroundingWord(t, theory, "length-options-scope")
	astTestMust(t, state.PushScope())
	previous, err := theory.freshStringVariable("old-length-indicator")
	astTestMust(t, err)
	astTestMust(t, state.PopScopes(1))
	if _, active := theory.internalVariables[previous]; !active {
		t.Fatal("native lifetime internal-variable membership was unexpectedly cleared")
	}
	current, err := theory.freshStringVariable("current-length-indicator")
	astTestMust(t, err)
	theory.testers.lengthIndicators[word] = []ast.ASTHandle{previous, current}
	root, err := theory.generateStringLengthOptions(word, current, 2)
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	length, err := m.MkSequenceLength(word)
	astTestMust(t, err)
	astTestPin(t, m, length)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	for _, label := range []string{"0", "more"} {
		got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{previous: astTestString(t, m, label), current: astTestString(t, m, "junk"), length: zero})
		if got != (label != "more") {
			t.Fatal("popped but lifetime-registered indicator was omitted from the guard", label, got)
		}
	}
	astTestMust(t, m.DelRef(&root))
	if m.IsLiveHandle(formula) {
		t.Fatal("guarded later-window result was retained beyond its caller-owned root")
	}
	if root, err := theory.generateStringLengthOptions(word, current, 0); err == nil || root != (ast.ASTRef{}) {
		t.Fatal("invalid unknown-length window accepted")
	}
}
