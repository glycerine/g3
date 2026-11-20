package smtcore

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringValueOptionTestByte(index int) byte {
	const first = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	if index < len(first) {
		return first[index]
	}
	if index < 126 {
		return byte(index - 94)
	}
	return byte(index + 1)
}

func TestSMTStringValueOptionsWindowsGuardsAndCoverage(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "value-options-word")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	theory.testers.valueIndicators[word] = make(map[int32][]smtStringScopedIndicator)
	var prior []smtStringScopedIndicator
	for tries := int32(0); tries < 8; tries++ {
		indicator, err := theory.freshStringValueTester(word, 1, tries)
		astTestMust(t, err)
		prior = append(prior, smtStringScopedIndicator{level: theory.level, variable: indicator})
		theory.testers.valueIndicators[word][1] = prior
		booleans := len(state.boolExpr)
		root, err := theory.generateStringValueOptions(word, length, indicator, "1", tries)
		astTestMust(t, err)
		if len(state.boolExpr) != booleans {
			t.Fatal("ordinary value generation internalized Boolean choices")
		}
		formula, err := m.RefGet(root)
		astTestMust(t, err)
		lower, upper := int(tries)*32, int(tries+1)*32
		if upper > 255 {
			upper = 255
		}
		if !reflect.DeepEqual(theory.testers.valueRanges[indicator], []int32{int32(upper - 1), 0}) {
			t.Fatal("range did not retain last generated encoding")
		}
		check := func(label, text string, want bool, wrongLength, wrongPrior bool) {
			t.Helper()
			lengthText := "1"
			if wrongLength {
				lengthText = "2"
			}
			values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), length: astTestString(t, m, lengthText), indicator: astTestString(t, m, label)}
			for i, old := range prior[:len(prior)-1] {
				value := "more"
				if wrongPrior && i == 0 {
					value = "0"
				}
				values[old.variable] = astTestString(t, m, value)
			}
			if got := stringAxiomTruth(t, m, rw, formula, values); got != want {
				t.Fatalf("window %d label %q word %q length/prior %v/%v got %v want %v", tries, label, text, wrongLength, wrongPrior, got, want)
			}
		}
		for index := lower; index < upper; index++ {
			label, text := strconv.Itoa(index), string([]byte{stringValueOptionTestByte(index)})
			check(label, text, true, false, false)
			check(label, "\x00", false, false, false)
			check("more", text, false, false, false)
		}
		check("more", "\x00", tries < 7, false, false)
		check("junk", "\x00", false, false, false)
		check("junk", "\x00", true, true, false)
		if tries > 0 {
			check("junk", "\x00", true, false, true)
		}
		astTestMust(t, m.DelRef(&root))
	}
	// Native value guards use all previous testers, without filtering their
	// lifetime membership or lexical scope. Preserve that asymmetry with len.
	astTestMust(t, state.PopScopes(1))
	delete(theory.internalVariables, prior[0].variable)
	root, err := theory.generateStringValueOptions(word, length, prior[1].variable, "1", 1)
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, "\x00"), length: astTestString(t, m, "1"), prior[1].variable: astTestString(t, m, "junk"), prior[0].variable: astTestString(t, m, "0")}
	if !stringAxiomTruth(t, m, rw, formula, values) {
		t.Fatal("out-of-scope prior guard was filtered")
	}
	astTestMust(t, m.DelRef(&root))
}

func TestSMTStringValueOptionsEmptyAndInvalidHistory(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "value-empty")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	indicator, err := theory.freshStringValueTester(word, 0, 0)
	astTestMust(t, err)
	theory.testers.valueIndicators[word] = map[int32][]smtStringScopedIndicator{0: {{variable: indicator}}}
	root, err := theory.generateStringValueOptions(word, length, indicator, "0", 0)
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	for _, test := range []struct {
		label, text string
		want        bool
	}{{"0", "", true}, {"0", "a", false}, {"more", "", false}, {"junk", "", false}} {
		if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{length: astTestString(t, m, "0"), indicator: astTestString(t, m, test.label), word: astTestString(t, m, test.text)}); got != test.want {
			t.Fatal("empty-word coverage", test, got)
		}
	}
	for _, ref := range theory.refs {
		term, err := m.RefGet(ref)
		astTestMust(t, err)
		if term == formula {
			t.Fatal("value formula gained an extra direct lifetime root")
		}
	}
	astTestMust(t, m.DelRef(&root))
	for _, test := range []struct {
		text  string
		tries int32
	}{{"-1", 0}, {"2147483647", 0}, {"2147483648", 0}, {"more", 0}, {"0", -1}, {"0", 1}, {"1", 1}, {"0", 1 << 30}} {
		if _, err := theory.generateStringValueOptions(word, length, indicator, test.text, test.tries); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("undefined source producer state accepted", test, err)
		}
	}
}

func TestSMTStringValueOptionsCachePhasesAndSelfGuard(t *testing.T) {
	m, theory, state, rw := stringRegexStageFixture(t)
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	for _, key := range []string{"str.fast_value_tester_cache", "str.aggressive_value_testing"} {
		key := key
		astTestMust(t, context.SetBoolName(&params, &key, true))
	}
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	state.caseSplitStrategy = 6
	astTestMust(t, state.ensureTheoryAwareBranching())
	priorities := len(state.theoryAwareBranching.priorities)
	word := stringGroundingWord(t, theory, "value-cache")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	indicator, err := theory.freshStringValueTester(word, 1, 0)
	astTestMust(t, err)
	root, err := theory.generateStringValueOptions(word, length, indicator, "1", 0)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(theory.testers.valueLiterals) != 32 {
		t.Fatal("value cache did not retain all generated words")
	}
	if len(state.theoryAwareBranching.priorities) != priorities {
		t.Fatal("value tester invented length-style branch priorities")
	}
	for _, label := range []string{"0", "31", "more"} {
		equal, err := state.mkEqualityAtom(indicator, astTestString(t, m, label))
		astTestMust(t, err)
		variable, ok := state.BoolVar(equal)
		if !ok {
			t.Fatal("aggressive choice not internalized")
		}
		data, err := state.BooleanData(variable)
		astTestMust(t, err)
		if !data.PhaseAvailable() || data.Phase() != (label != "more") || data.TryTrueFirst() || !state.isRelevant(equal) {
			t.Fatal("aggressive value phase changed", label)
		}
		assigned, err := state.Assignment(NewSMTLiteral(variable, false))
		astTestMust(t, err)
		if assigned != lbool.Undef {
			t.Fatal("phase hint assigned a value")
		}
	}
	cached := theory.testers.valueLiterals["a"]
	count, err := m.GetRefCount(cached)
	astTestMust(t, err)
	root, err = theory.generateStringValueOptions(word, length, indicator, "1", 0)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	after, err := m.GetRefCount(cached)
	astTestMust(t, err)
	if after != count || theory.testers.valueLiterals["a"] != cached {
		t.Fatal("cache hit acquired another root")
	}
	// Source skips a previous entry equal to the current tester. The prior
	// encoding still supplies the next window before the map entry is replaced.
	theory.testers.valueIndicators[word] = map[int32][]smtStringScopedIndicator{1: {{variable: indicator}}}
	root, err = theory.generateStringValueOptions(word, length, indicator, "1", 1)
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	if stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{length: astTestString(t, m, "1"), indicator: astTestString(t, m, "junk"), word: astTestString(t, m, "\x00")}) {
		t.Fatal("self guard made constraints vacuous")
	}
	astTestMust(t, m.DelRef(&root))
	fresh, err := theory.Fresh()
	astTestMust(t, err)
	defer func() { astTestMust(t, fresh.Close()) }()
	if len(fresh.testers.valueIndicators) != 0 || len(fresh.testers.valueRanges) != 0 || len(fresh.testers.valueLiterals) != 0 {
		t.Fatal("Fresh copied value tester caches/history")
	}
}

func TestSMTStringValueOptionsTwoByteCarryBoundary(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "value-two-byte")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	previous, err := theory.freshStringValueTester(word, 2, 6)
	astTestMust(t, err)
	indicator, err := theory.freshStringValueTester(word, 2, 7)
	astTestMust(t, err)
	// Seed the exact preceding batch endpoint, then cross base-255 carry at
	// ordinals 254 -> 255 while the 32-option batch remains incomplete.
	entries := make([]smtStringScopedIndicator, 7)
	for i := range entries {
		entries[i].variable = previous
	}
	theory.testers.valueIndicators[word] = map[int32][]smtStringScopedIndicator{2: entries}
	theory.testers.valueRanges[previous] = []int32{223, 0, 0}
	root, err := theory.generateStringValueOptions(word, length, indicator, "2", 7)
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	for _, test := range []struct {
		label, text string
		want        bool
	}{
		{"224", "a\xe1", true}, {"254", "a\xff", true}, {"255", "ba", true},
		{"255", "a\xff", false}, {"more", "bb", true}, {"more", "ba", false},
	} {
		values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, test.text), length: astTestString(t, m, "2"), indicator: astTestString(t, m, test.label), previous: astTestString(t, m, "more")}
		if got := stringAxiomTruth(t, m, rw, formula, values); got != test.want {
			t.Fatal("two-byte carry", test, got)
		}
	}
	if !reflect.DeepEqual(theory.testers.valueRanges[indicator], []int32{0, 1, 0}) || !reflect.DeepEqual(theory.testers.valueRanges[previous], []int32{223, 0, 0}) {
		t.Fatal("carry endpoint or previous range changed")
	}
	astTestMust(t, m.DelRef(&root))
}
