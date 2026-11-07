package smtcore

import (
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func stringLibraryFixture(t *testing.T) (*ast.ASTManager, *SMTStringTheory, *SMTInternalizerState, *rewriter.TheoryRewriter) {
	t.Helper()
	m, config := smtModelGeneratorFixture(t)
	// ctx.get_rewriter() is the asserted_formulas rewriter, configured by
	// set_eliminate_and (arith_ineq_lhs, som, ...).
	params, err := config.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, config.Context.Del(&params)) })
	for _, name := range []string{"arith_ineq_lhs", "sort_sums", "rewrite_patterns", "gcd_rounding", "expand_select_store", "bv_sort_ac", "som"} {
		astTestMust(t, config.Context.SetBoolName(&params, &name, true))
	}
	name := "elim_and"
	astTestMust(t, config.Context.SetBoolName(&params, &name, false))
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, &params)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, rw.Close()) })
	theory, state := stringTheoryFixture(t, m)
	state.theoryRewriter = rw
	return m, theory, state, rw
}

func stringLibraryInteger(t *testing.T, m *ast.ASTManager, state *SMTInternalizerState, name string) ast.ASTHandle {
	t.Helper()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	variable, err := m.MkConst(astTestSymbol(t, m, name), sort)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(variable, false))
	return variable
}

func stringLibraryWitness(t *testing.T, theory *SMTStringTheory, name string) ast.ASTHandle {
	t.Helper()
	m := theory.state.manager
	want := astTestSymbol(t, m, name)
	for _, node := range theory.state.context.enodes.all() {
		expression := node.owner
		declaration, err := m.GetDecl(expression)
		astTestMust(t, err)
		actual, err := m.GetASTName(declaration)
		astTestMust(t, err)
		if actual == want {
			return expression
		}
	}
	t.Fatal("missing native library witness", name)
	return ast.ASTHandle{}
}

func stringLibraryAxiomsTruth(t *testing.T, m *ast.ASTManager, rw *rewriter.TheoryRewriter, axioms []ast.ASTHandle, values map[ast.ASTHandle]ast.ASTHandle) bool {
	t.Helper()
	var roots []ast.ASTRef
	defer func() {
		for i := range roots {
			astTestMust(t, m.DelRef(&roots[i]))
		}
	}()
	// Each one-clause replacement owns its bindings only during that call.
	// Keep every destination alive across the entire multi-clause check.
	for _, value := range values {
		ref, err := m.NewRef(value)
		astTestMust(t, err)
		roots = append(roots, ref)
	}
	truth := true
	for _, axiom := range axioms {
		truth = stringAxiomTruth(t, m, rw, axiom, values) && truth
	}
	return truth
}

func TestSMTStringCharAtAxiomConcreteBoundsAndReduction(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "char-at-word")
	index := stringLibraryInteger(t, m, state, "char-at-index")
	expression, err := m.MkSequenceAt(word, index)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.instantiateCharAt(state.context.enodes.get(expression)))
	variables := []ast.ASTHandle{stringLibraryWitness(t, theory, "ts0!tmp0"), stringLibraryWitness(t, theory, "ts1!tmp1"), stringLibraryWitness(t, theory, "ts2!tmp2")}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	for _, text := range []string{"", "a", "abc", "\x00\xff"} {
		for _, position := range []int{-1, 0, 1, 2, 3} {
			prefix, selected, tail := "", "", ""
			if position >= 0 && position < len(text) {
				prefix, selected, tail = text[:position], text[position:position+1], text[position+1:]
			}
			for _, output := range []string{selected, "#wrong#"} {
				numeral, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(position))
				astTestMust(t, err)
				values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), index: numeral, expression: astTestString(t, m, output), variables[0]: astTestString(t, m, prefix), variables[1]: astTestString(t, m, selected), variables[2]: astTestString(t, m, tail)}
				if got := stringAxiomTruth(t, m, rw, axiom, values); got != (output == selected) {
					t.Fatal("character-at decomposition lost a boundary or its result reduction", text, position, output, got)
				}
			}
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateCharAt(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("duplicate character-at setup repeated its axiom/witnesses")
	}
}

func TestSMTStringSubstringAxiomsConcreteTruncationAndInvalidBounds(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "substring-word")
	position := stringLibraryInteger(t, m, state, "substring-position")
	count := stringLibraryInteger(t, m, state, "substring-count")
	expression, err := m.MkSequenceExtract(word, position, count)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.instantiateSubstring(state.context.enodes.get(expression)))
	variables := make([]ast.ASTHandle, 5)
	for i := range variables {
		variables[i] = stringLibraryWitness(t, theory, "t"+strconv.Itoa(i)+"!tmp"+strconv.Itoa(i))
	}
	var axioms []ast.ASTHandle
	for _, ref := range theory.refs[len(theory.refs)-3:] {
		axiom, err := m.RefGet(ref)
		astTestMust(t, err)
		axioms = append(axioms, axiom)
	}
	for _, text := range []string{"", "a", "abc", "\x00\xff"} {
		for _, start := range []int{-1, 0, 1, 2, 3} {
			for _, size := range []int{-1, 0, 1, 2, 5} {
				parts := [5]string{}
				selected := ""
				if start >= 0 && start < len(text) && size >= 0 {
					end := start + size
					if end >= len(text) {
						parts[0], parts[1], selected = text[:start], text[start:], text[start:]
					} else {
						parts[2], parts[3], parts[4] = text[:start], text[start:end], text[end:]
						selected = text[start:end]
					}
				}
				for _, output := range []string{selected, "#wrong#"} {
					p, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(start))
					astTestMust(t, err)
					astTestPin(t, m, p)
					c, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(size))
					astTestMust(t, err)
					astTestPin(t, m, c)
					values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), position: p, count: c, expression: astTestString(t, m, output)}
					for i, variable := range variables {
						values[variable] = astTestString(t, m, parts[i])
					}
					truth := stringLibraryAxiomsTruth(t, m, rw, axioms, values)
					if truth != (output == selected) {
						t.Fatal("substring cases lost invalid/zero-length/truncation semantics", text, start, size, output, truth)
					}
				}
			}
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateSubstring(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("duplicate substring setup repeated its cases/witnesses")
	}
}

func TestSMTStringReplaceAxiomsFirstOccurrenceAndNoMatch(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "replace-word")
	needle := stringContainmentWord(t, theory, "replace-needle")
	replacement := stringContainmentWord(t, theory, "replace-replacement")
	expression, err := m.MkSequenceReplace(word, needle, replacement)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(expression, false))
	astTestMust(t, theory.instantiateReplace(state.context.enodes.get(expression)))
	variables := []ast.ASTHandle{
		stringLibraryWitness(t, theory, "x1!tmp0"), stringLibraryWitness(t, theory, "x2!tmp1"),
		stringLibraryWitness(t, theory, "i1!tmp2"), stringLibraryWitness(t, theory, "result!tmp3"),
		stringLibraryWitness(t, theory, "x3!tmp4"), stringLibraryWitness(t, theory, "x4!tmp5"),
	}
	var axioms []ast.ASTHandle
	for _, ref := range theory.refs[len(theory.refs)-2:] {
		axiom, err := m.RefGet(ref)
		astTestMust(t, err)
		axioms = append(axioms, axiom)
	}
	for _, text := range []string{"", "abc", "ababa", "\x00a\x00"} {
		for _, find := range []string{"b", "ab", "\x00"} {
			for _, replace := range []string{"R", "", "\xff"} {
				index := strings.Index(text, find)
				selected := text
				prefix, tail, before, after := "", "", "", ""
				if index >= 0 {
					prefix, tail = text[:index], text[index+len(find):]
					endBefore := index + len(find) - 1
					before, after = text[:endBefore], text[endBefore:]
					selected = prefix + replace + tail
				}
				for _, output := range []string{selected, "#wrong#"} {
					numeral, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(index))
					astTestMust(t, err)
					astTestPin(t, m, numeral)
					values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), needle: astTestString(t, m, find), replacement: astTestString(t, m, replace), expression: astTestString(t, m, output), variables[0]: astTestString(t, m, prefix), variables[1]: astTestString(t, m, tail), variables[2]: numeral, variables[3]: astTestString(t, m, selected), variables[4]: astTestString(t, m, before), variables[5]: astTestString(t, m, after)}
					truth := stringLibraryAxiomsTruth(t, m, rw, axioms, values)
					if truth != (output == selected) {
						t.Fatal("replace decomposition lost first occurrence, no-match or result reduction", text, find, replace, output, truth)
					}
				}
			}
		}
	}
	// A later occurrence satisfies the two decompositions and result
	// reduction but must fail the native no-earlier-match constraint.
	later, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, "ababa"), needle: astTestString(t, m, "ab"), replacement: astTestString(t, m, "R"), expression: astTestString(t, m, "abRa"), variables[0]: astTestString(t, m, "ab"), variables[1]: astTestString(t, m, "a"), variables[2]: later, variables[3]: astTestString(t, m, "abRa"), variables[4]: astTestString(t, m, "aba"), variables[5]: astTestString(t, m, "ba")}
	if stringLibraryAxiomsTruth(t, m, rw, axioms, values) {
		t.Fatal("replacement accepted a later occurrence despite an earlier match")
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.instantiateReplace(state.context.enodes.get(expression)))
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("duplicate replacement setup repeated axioms/witnesses")
	}
}

func TestSMTStringExtractionAndReplaceLibraryQueue(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringContainmentWord(t, theory, "library-queue-word")
	needle := stringContainmentWord(t, theory, "library-queue-needle")
	replacement := stringContainmentWord(t, theory, "library-queue-replacement")
	position := stringLibraryInteger(t, m, state, "library-queue-position")
	count := stringLibraryInteger(t, m, state, "library-queue-count")
	at, err := m.MkSequenceAt(word, position)
	astTestMust(t, err)
	extract, err := m.MkSequenceExtract(word, position, count)
	astTestMust(t, err)
	replace, err := m.MkSequenceReplace(word, needle, replacement)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{at, extract, replace} {
		astTestMust(t, state.Internalize(expression, false))
		astTestMust(t, theory.setUpAxioms(expression))
	}
	astTestMust(t, theory.Propagate())
	if theory.CanPropagate() || len(theory.contains) != 2 {
		t.Fatal("library queue did not complete recursive replacement-containment setup", theory.CanPropagate(), len(theory.contains))
	}
	for _, expression := range []ast.ASTHandle{at, extract, replace} {
		if _, processed := theory.axiomatized[expression]; !processed {
			t.Fatal("library operation bypassed its native axiom producer")
		}
	}
	before, fresh := len(theory.refs), theory.freshID
	astTestMust(t, theory.Propagate())
	if len(theory.refs) != before || theory.freshID != fresh {
		t.Fatal("empty library worklist repeated setup")
	}
}
