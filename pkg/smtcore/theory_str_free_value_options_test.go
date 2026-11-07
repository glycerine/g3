package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringFreeValueOptionsCreationReplayAndCallback(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "free-value-word")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	root, err := theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	if root == (ast.ASTRef{}) {
		t.Fatal("first tester did not return its constraint")
	}
	astTestMust(t, m.DelRef(&root))
	entries := theory.testers.valueIndicators[word][1]
	if len(entries) != 1 || entries[0].level != theory.level || theory.testers.valueSubjects[entries[0].variable] != word {
		t.Fatal("missing scoped history or reverse subject")
	}
	first := entries[0].variable
	before := len(theory.basicTodo)
	root, err = theory.generateStringFreeValueOptions(word, length, "1", first, "")
	astTestMust(t, err)
	if root != (ast.ASTRef{}) || len(theory.basicTodo) != before {
		t.Fatal("callback tester was replayed before callback stop")
	}
	root, err = theory.generateStringFreeValueOptions(word, length, "1", first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	entries = theory.testers.valueIndicators[word][1]
	if len(entries) != 2 || theory.testers.valueSubjects[entries[1].variable] != word || theory.testers.valueRanges[entries[1].variable][0] != 63 {
		t.Fatal("more callback did not append the next batch")
	}
	second := entries[1].variable
	fresh, before := theory.freshID, len(theory.basicTodo)
	root, err = theory.generateStringFreeValueOptions(word, length, "1", first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || len(theory.basicTodo) != before+1 || theory.basicTodo[len(theory.basicTodo)-1].Owner() != second {
		t.Fatal("existing successor not refreshed in place")
	}
	// A call without a callback reasserts each still-unassigned tester in
	// source order and returns no separate formula.
	var expected []ast.ASTHandle
	var pins []ast.ASTRef
	for i, entry := range entries {
		ref, err := theory.generateStringValueOptions(word, length, entry.variable, "1", int32(i))
		astTestMust(t, err)
		pins = append(pins, ref)
		formula, err := m.RefGet(ref)
		astTestMust(t, err)
		expected = append(expected, formula)
	}
	beforeRefs, before := len(theory.refs), len(theory.basicTodo)
	root, err = theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	if root != (ast.ASTRef{}) || len(theory.basicTodo) < before+2 {
		t.Fatal("unassigned tester replay missing")
	}
	var replay []ast.ASTHandle
	for _, ref := range theory.refs[beforeRefs:] {
		formula, err := m.RefGet(ref)
		astTestMust(t, err)
		if formula == expected[0] || formula == expected[1] {
			replay = append(replay, formula)
		}
	}
	if len(replay) != 2 || replay[0] != expected[0] || replay[1] != expected[1] {
		t.Fatal("replay assertion ordering changed", replay, expected)
	}
	for _, ref := range pins {
		ref := ref
		astTestMust(t, m.DelRef(&ref))
	}
}

func TestSMTStringFreeValueOptionsAssignedSkipAndScopeRefresh(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "free-value-scope")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	astTestMust(t, state.PushScope())
	root, err := theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.valueIndicators[word][1][0].variable
	astTestMust(t, state.PopScopes(1))
	if state.context.enodes.get(first) != nil {
		t.Fatal("test requires popped tester enode")
	}
	fresh := theory.freshID
	root, err = theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	if root != (ast.ASTRef{}) || theory.freshID != fresh || state.context.enodes.get(first) == nil {
		t.Fatal("persistent internal membership did not refresh popped tester")
	}
	zero := astTestString(t, m, "0")
	astTestMust(t, state.Internalize(zero, false))
	stringDependencyPrivateMerge(t, theory, first, zero)
	beforeRefs, beforeTodo := len(theory.refs), len(theory.basicTodo)
	root, err = theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	if root != (ast.ASTRef{}) || len(theory.refs) != beforeRefs || len(theory.basicTodo) != beforeTodo {
		t.Fatal("assigned tester was refreshed or reasserted")
	}
}

func TestSMTStringFreeValueOptionsInactiveHistoryAppendAndSkip(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "free-value-inactive")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	root, err := theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.valueIndicators[word][1][0].variable
	delete(theory.internalVariables, first)
	root, err = theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	entries := theory.testers.valueIndicators[word][1]
	if len(entries) != 2 || entries[0].variable != first || theory.testers.valueRanges[entries[1].variable][0] != 31 {
		t.Fatal("effectively empty history was cleared or started at wrong batch")
	}
	second := entries[1].variable
	// The index remains one despite the new tester's tries-zero name. Native
	// replay uses that physical index, so it now regenerates batch 32..63.
	before := len(theory.basicTodo)
	root, err = theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	if root != (ast.ASTRef{}) || theory.testers.valueRanges[second][0] != 63 || theory.basicTodo[before].Owner() != second {
		t.Fatal("inactive history was compacted or replayed")
	}
}

func TestSMTStringFreeValueOptionsInvalidCallbackPreservesSourceSideEffects(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "free-value-invalid")
	length, err := theory.freshStringLengthTester(word, 1)
	astTestMust(t, err)
	fresh := theory.freshID
	if _, err := theory.generateStringFreeValueOptions(word, length, "more", ast.ASTHandle{}, ""); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("invalid length accepted", err)
	}
	if theory.freshID != fresh {
		t.Fatal("invalid length consumed a tester")
	}
	root, err := theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	// A missing callback with 'more' reaches native i==testerTotal, appends
	// tries=total+1 and then reads that new tester's nonexistent last range.
	// Keep its registration side effects, but report the undefined vector
	// access as a domain error rather than silently correcting the index.
	_, err = theory.generateStringFreeValueOptions(word, length, "1", ast.ASTHandle{}, "more")
	if !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("missing callback producer state accepted", err)
	}
	entries := theory.testers.valueIndicators[word][1]
	if len(entries) != 2 || theory.testers.valueSubjects[entries[1].variable] != word {
		t.Fatal("source registration side effects lost on invalid callback")
	}
	if _, ok := theory.testers.valueRanges[entries[1].variable]; ok {
		t.Fatal("invalid callback fabricated a previous range")
	}
}
