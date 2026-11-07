package smtcore

import (
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringIndexLibraryQueue(t *testing.T) {
	for _, offset := range []string{"0", "2"} {
		t.Run(offset, func(t *testing.T) {
			m, theory, state, _ := stringLibraryFixture(t)
			word := stringContainmentWord(t, theory, "index-dispatch-word")
			needle := stringContainmentWord(t, theory, "index-dispatch-needle")
			start, err := m.ArithmeticIntegerNumeralText(offset)
			astTestMust(t, err)
			expression, err := m.MkSequenceIndex(word, needle, start)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(expression, false))
			astTestMust(t, theory.setUpAxioms(expression))
			astTestMust(t, theory.Propagate())
			if _, found := theory.axiomatized[expression]; !found || theory.CanPropagate() {
				t.Fatal("index library operation did not finish its initial axioms", found, theory.CanPropagate())
			}
			before, fresh := len(theory.refs), theory.freshID
			astTestMust(t, theory.Propagate())
			if len(theory.refs) != before || theory.freshID != fresh {
				t.Fatal("quiescent index worklist repeated witnesses or axioms")
			}
		})
	}
}

func TestSMTStringIndexFirstOccurrenceAndNoMatch(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "index-word")
	needle := stringContainmentWord(t, theory, "index-needle")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	expression, err := m.MkSequenceIndex(word, needle, zero)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.instantiateIndex(state.context.enodes.get(expression)))
	variables := []ast.ASTHandle{
		stringLibraryWitness(t, theory, "x1!tmp0"), stringLibraryWitness(t, theory, "x2!tmp1"),
		stringLibraryWitness(t, theory, "index!tmp2"),
		stringLibraryWitness(t, theory, "x3!tmp3"), stringLibraryWitness(t, theory, "x4!tmp4"),
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	if len(theory.delayedAssertions) != 0 {
		t.Fatal("simple index queued the native commented-out heuristic")
	}
	for _, text := range []string{"", "abc", "ababa", "\x00a\x00\xff"} {
		for _, find := range []string{"b", "ab", "\x00"} {
			index := strings.Index(text, find)
			prefix, tail, before, after := "", "", "", ""
			if index >= 0 {
				prefix, tail = text[:index], text[index+len(find):]
				end := index + len(find) - 1
				before, after = text[:end], text[end:]
			}
			for _, output := range []int{index, index + 1} {
				actual, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(output))
				astTestMust(t, err)
				astTestPin(t, m, actual)
				selected, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(index))
				astTestMust(t, err)
				values := map[ast.ASTHandle]ast.ASTHandle{
					word: astTestString(t, m, text), needle: astTestString(t, m, find), expression: actual,
					variables[0]: astTestString(t, m, prefix), variables[1]: astTestString(t, m, tail), variables[2]: selected,
					variables[3]: astTestString(t, m, before), variables[4]: astTestString(t, m, after),
				}
				if truth := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); truth != (output == index) {
					t.Fatal("simple index decomposition/result mismatch", text, find, output, index, truth)
				}
			}
		}
	}
	later, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	values := map[ast.ASTHandle]ast.ASTHandle{
		word: astTestString(t, m, "ababa"), needle: astTestString(t, m, "ab"), expression: later,
		variables[0]: astTestString(t, m, "ab"), variables[1]: astTestString(t, m, "a"), variables[2]: later,
		variables[3]: astTestString(t, m, "aba"), variables[4]: astTestString(t, m, "ba"),
	}
	if stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values) {
		t.Fatal("simple index accepted a later occurrence")
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateIndex(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("duplicate simple index repeated native witnesses/axioms")
	}
}

func TestSMTStringIndexExtendedBoundsAndTailRelativeResult(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "extended-index-word")
	needle := stringContainmentWord(t, theory, "extended-index-needle")
	start := stringLibraryInteger(t, m, state, "extended-index-start")
	expression, err := m.MkSequenceIndex(word, needle, start)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	before := len(theory.refs)
	astTestMust(t, theory.instantiateIndex(state.context.enodes.get(expression)))
	var axioms []ast.ASTHandle
	for _, ref := range theory.refs[before:] {
		value, err := m.RefGet(ref)
		astTestMust(t, err)
		family, err := m.GetFamilyID(value)
		astTestMust(t, err)
		kind, err := m.GetDeclKind(value)
		astTestMust(t, err)
		if family == m.BasicFamilyID() && kind == ast.ASTOpOr {
			axioms = append(axioms, value)
		}
	}
	if len(axioms) != 4 || len(theory.delayedAssertions) != 1 || theory.freshID != 2 {
		t.Fatal("extended index lost native cases, delayed heuristic or fresh order", len(axioms), len(theory.delayedAssertions), theory.freshID)
	}
	head := stringLibraryWitness(t, theory, "hd!tmp0")
	tail := stringLibraryWitness(t, theory, "tl!tmp1")
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	simple, err := m.MkSequenceIndex(word, needle, zero)
	astTestMust(t, err)
	tailIndex, err := m.MkSequenceIndex(tail, needle, zero)
	astTestMust(t, err)
	number := func(value int) ast.ASTHandle {
		valueAST, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(value))
		astTestMust(t, err)
		astTestPin(t, m, valueAST)
		return valueAST
	}
	for _, text := range []string{"", "abc", "ababa", "\x00a\x00\xff"} {
		for _, find := range []string{"b", "ab", "\x00"} {
			for _, position := range []int{-1, 0, 1, 2, 4, 5} {
				hd, tl, expected := "", "", -1
				if position == 0 {
					expected = strings.Index(text, find)
				} else if position > 0 && position < len(text) {
					hd, tl = text[:position], text[position:]
					// Retained source returns the tail's index directly.
					expected = strings.Index(tl, find)
				}
				for _, output := range []int{expected, expected + 1} {
					values := map[ast.ASTHandle]ast.ASTHandle{
						word: astTestString(t, m, text), needle: astTestString(t, m, find), start: number(position), expression: number(output),
						head: astTestString(t, m, hd), tail: astTestString(t, m, tl), simple: number(strings.Index(text, find)), tailIndex: number(strings.Index(tl, find)),
					}
					if truth := stringLibraryAxiomsTruth(t, m, rw, axioms, values); truth != (output == expected) {
						t.Fatal("extended index source cases lost bounds or tail-relative result", text, find, position, output, expected, truth)
					}
				}
			}
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateIndex(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh || len(theory.delayedAssertions) != 1 {
		t.Fatal("duplicate extended index repeated cases/heuristic")
	}
}

func TestSMTStringIndexQueueDefersHeuristicUntilSearch(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "index-queue-word")
	needle := stringContainmentWord(t, theory, "index-queue-needle")
	start := stringLibraryInteger(t, m, state, "index-queue-start")
	expression, err := m.MkSequenceIndex(word, needle, start)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.setUpAxioms(expression))
	astTestMust(t, theory.Propagate())
	if theory.CanPropagate() || len(theory.delayedAssertions) != 1 {
		t.Fatal("pre-search index propagation consumed or activated the deferred heuristic", theory.CanPropagate(), len(theory.delayedAssertions))
	}
	if _, found := theory.axiomatized[expression]; !found {
		t.Fatal("index library worklist skipped its producer")
	}
	// Both zero-start child index terms must have gone through the real
	// constructor/setup path and acquired their own first-occurrence axioms.
	count := 0
	for value := range theory.axiomatized {
		op, own, err := theory.operation(value)
		astTestMust(t, err)
		if own && op == 10 {
			count++
		}
	}
	if count != 3 {
		t.Fatal("extended index did not recursively axiomatize both zero-start children", count)
	}
	queueRoot := theory.delayedAssertions[0]
	heuristic, err := m.RefGet(queueRoot)
	astTestMust(t, err)
	before := len(theory.refs)
	astTestMust(t, theory.InitializeSearch())
	if !theory.searchStarted || !theory.CanPropagate() || len(theory.delayedAssertions) != 1 || len(theory.refs) != before {
		t.Fatal("search initialization failed to expose deferred work after its initial propagation")
	}
	astTestMust(t, theory.Propagate())
	if theory.CanPropagate() || len(theory.delayedAssertions) != 0 || len(theory.refs) != before+1 {
		t.Fatal("search propagation failed to publish the deferred equivalence once")
	}
	if _, err := m.RefGet(queueRoot); err == nil {
		t.Fatal("consumed deferred assertion retained its queue-owned AST reference")
	}
	actual, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	if actual != heuristic {
		t.Fatal("search propagation published a different heuristic")
	}
	astTestMust(t, theory.Propagate())
	if len(theory.refs) != before+1 {
		t.Fatal("empty worklist repeated the deferred equivalence")
	}
}

func TestSMTStringIndexDeferredOwnershipBeforeSearchClose(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "index-close-word")
	needle := stringContainmentWord(t, theory, "index-close-needle")
	start := stringLibraryInteger(t, m, state, "index-close-start")
	expression, err := m.MkSequenceIndex(word, needle, start)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.instantiateIndex(state.context.enodes.get(expression)))
	if len(theory.delayedAssertions) != 1 {
		t.Fatal("missing queue-owned pre-search heuristic")
	}
	root := theory.delayedAssertions[0]
	_, err = m.RefGet(root)
	astTestMust(t, err)
	astTestMust(t, theory.Close())
	if _, err := m.RefGet(root); err == nil || len(theory.delayedAssertions) != 0 {
		t.Fatal("close leaked an unconsumed deferred assertion root")
	}
}
