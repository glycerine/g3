package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringGroundingWord(t *testing.T, theory *SMTStringTheory, name string) ast.ASTHandle {
	t.Helper()
	word := stringContainmentWord(t, theory, name)
	astTestMust(t, theory.setUpAxioms(word))
	return word
}

func TestSMTStringGroundedPartialMatchingSourceBoundaries(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "ground-part-x")
	y := stringGroundingWord(t, theory, "ground-part-y")
	z := stringGroundingWord(t, theory, "ground-part-z")
	literal := func(text string) ast.ASTHandle {
		value := astTestString(t, m, text)
		astTestPin(t, m, value)
		return value
	}
	a, ab, bc, cd, b, c, empty := literal("a"), literal("ab"), literal("bc"), literal("cd"), literal("b"), literal("c"), literal("")
	for _, test := range []struct {
		name         string
		word, needle []ast.ASTHandle
		want         bool
	}{
		{"empty-word", nil, []ast.ASTHandle{empty}, false},
		{"empty-pattern-vector", []ast.ASTHandle{ab}, nil, false},
		{"longer-pattern-vector", []ast.ASTHandle{x}, []ast.ASTHandle{x, y}, false},
		{"single-literal-inside", []ast.ASTHandle{x, ab, y}, []ast.ASTHandle{b}, true},
		{"single-literal-absent", []ast.ASTHandle{x, ab, y}, []ast.ASTHandle{c}, false},
		{"empty-literal-present", []ast.ASTHandle{x, ab}, []ast.ASTHandle{empty}, true},
		{"empty-literal-unresolved", []ast.ASTHandle{x, y}, []ast.ASTHandle{empty}, false},
		{"single-variable-present", []ast.ASTHandle{ab, x}, []ast.ASTHandle{x}, true},
		{"single-variable-identity", []ast.ASTHandle{ab, x}, []ast.ASTHandle{y}, false},
		{"suffix-head-prefix-tail", []ast.ASTHandle{ab, x, cd}, []ast.ASTHandle{b, x, c}, true},
		{"head-mismatch", []ast.ASTHandle{ab, x, cd}, []ast.ASTHandle{a, x, c}, false},
		{"middle-identity-mismatch", []ast.ASTHandle{ab, y, cd}, []ast.ASTHandle{b, x, c}, false},
		{"tail-mismatch", []ast.ASTHandle{ab, x, cd}, []ast.ASTHandle{b, x, b}, false},
		{"scan-later-window", []ast.ASTHandle{y, ab, x, cd}, []ast.ASTHandle{b, x, c}, true},
		{"literal-head-variable-hay", []ast.ASTHandle{y, x, cd}, []ast.ASTHandle{b, x, c}, false},
		{"literal-tail-variable-hay", []ast.ASTHandle{ab, x, y}, []ast.ASTHandle{b, x, c}, false},
		{"variable-tail-identity", []ast.ASTHandle{ab, x, y}, []ast.ASTHandle{b, x, y}, true},
		{"variable-tail-mismatch", []ast.ASTHandle{ab, x, z}, []ast.ASTHandle{b, x, y}, false},
		// In the retained multi-part branch the variable head is unchecked.
		{"native-variable-head", []ast.ASTHandle{y, x, cd}, []ast.ASTHandle{z, x, c}, true},
		{"nul-high-byte", []ast.ASTHandle{literal("a\x00"), x, literal("\xffz")}, []ast.ASTHandle{literal("\x00"), x, literal("\xff")}, true},
		{"two-parts", []ast.ASTHandle{ab, bc}, []ast.ASTHandle{b, b}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := theory.partialInGroundedConcat(test.word, test.needle)
			astTestMust(t, err)
			if got != test.want {
				t.Fatal("native grounded partial-match rule changed", got, test.want)
			}
		})
	}
}

func TestSMTStringGroundingAliasesFoldLiteralsAndKeepPremises(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	g := newStringGrounding(theory)
	t.Cleanup(func() { astTestMust(t, g.close()) })
	x := stringGroundingWord(t, theory, "ground-x")
	y := stringGroundingWord(t, theory, "ground-y")
	xAlias := stringGroundingWord(t, theory, "ground-x-alias")
	yAlias := stringGroundingWord(t, theory, "ground-y-alias")
	z := stringGroundingWord(t, theory, "ground-z")
	a, bc := astTestString(t, m, "a\x00"), astTestString(t, m, "\xffb")
	astTestPin(t, m, a)
	astTestPin(t, m, bc)
	astTestMust(t, state.Internalize(a, false))
	astTestMust(t, state.Internalize(bc, false))
	concat, err := m.MkSequenceConcat(xAlias, yAlias)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(concat, false))
	g.variableAliases[xAlias], g.variableAliases[yAlias] = x, y
	g.variableValues[x], g.variableValues[y] = a, bc
	g.variableConcats[z] = map[ast.ASTHandle]int{concat: 1}
	astTestMust(t, g.ground(z))
	entries := g.grounded[z]
	if len(entries) != 1 || len(entries[0].parts) != 1 || len(entries[0].conditions) != 5 {
		t.Fatal("grounding lost folded literals or an alias/value/concat premise", entries)
	}
	text, literal, err := m.SequenceStringValue(entries[0].parts[0])
	astTestMust(t, err)
	if !literal || text != "a\x00\xffb" {
		t.Fatal("grounding did not merge adjacent constant bytes", text)
	}
	for _, pair := range [][2]ast.ASTHandle{{z, concat}, {xAlias, x}, {yAlias, y}, {x, a}, {y, bc}} {
		condition, err := stringEqAtom(m, pair[0], pair[1])
		astTestMust(t, err)
		found := false
		for _, actual := range entries[0].conditions {
			found = found || actual == condition
		}
		if !found {
			t.Fatal("grounding dropped an explicit dependency premise", pair)
		}
	}
	before := len(g.locals.refs)
	astTestMust(t, g.ground(z))
	if len(g.locals.refs) != before {
		t.Fatal("grounding repeated a memoized expansion")
	}
}

func TestSMTStringGroundingConstantShortcutFirstConcatAndUnrollRefusal(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	g := newStringGrounding(theory)
	t.Cleanup(func() { astTestMust(t, g.close()) })
	x := stringGroundingWord(t, theory, "ground-choice-x")
	y := stringGroundingWord(t, theory, "ground-choice-y")
	z := stringGroundingWord(t, theory, "ground-choice-z")
	first, err := m.MkSequenceConcat(x, y)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(first, false))
	second, err := m.MkSequenceConcat(y, x)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(second, false))
	choices := []ast.ASTHandle{first, second}
	stringSortHandles(choices)
	g.variableConcats[z] = map[ast.ASTHandle]int{second: 1, first: 1}
	astTestMust(t, g.ground(z))
	if len(g.grounded[z]) != 1 || len(g.grounded[z][0].conditions) != 1 {
		t.Fatal("variable grounding enumerated beyond its first native concat")
	}
	condition, err := stringEqAtom(m, z, choices[0])
	astTestMust(t, err)
	if g.grounded[z][0].conditions[0] != condition {
		t.Fatal("variable grounding chose map iteration order instead of stable handle order")
	}
	alias, err := m.MkSequenceConcat(z, y)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(alias, false))
	constant := astTestString(t, m, "fixed")
	astTestPin(t, m, constant)
	g.concatAliases[alias] = choices[1]
	g.concatValues[choices[1]] = constant
	astTestMust(t, g.ground(alias))
	if len(g.grounded[choices[1]]) != 1 || g.grounded[choices[1]][0].parts[0] != constant {
		t.Fatal("dealiased concat constant did not bypass child expansion")
	}
	bound, err := theory.freshUnrollBound()
	astTestMust(t, err)
	unroll := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "a"), bound)
	astTestMust(t, g.ground(unroll))
	if _, found := g.grounded[unroll]; found {
		t.Fatal("unroll was given an invented grounding")
	}
	withUnroll, err := m.MkSequenceConcat(unroll, x)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(withUnroll, false))
	astTestMust(t, g.ground(withUnroll))
	if entries, found := g.grounded[unroll]; !found || len(entries) != 0 || len(g.grounded[withUnroll]) != 0 {
		t.Fatal("unroll child did not preserve native empty-map insertion and absent product")
	}
}

func TestSMTStringComputeContainsGroundedGuardAndNoMatch(t *testing.T) {
	for _, needleText := range []string{"BC", "absent"} {
		t.Run(needleText, func(t *testing.T) {
			m, theory, state, rw := stringLibraryFixture(t)
			g := newStringGrounding(theory)
			t.Cleanup(func() { astTestMust(t, g.close()) })
			word := stringGroundingWord(t, theory, "ground-contains-word")
			needle := stringGroundingWord(t, theory, "ground-contains-needle")
			wordAlias := stringGroundingWord(t, theory, "ground-contains-word-alias")
			needleAlias := stringGroundingWord(t, theory, "ground-contains-needle-alias")
			x := stringGroundingWord(t, theory, "ground-contains-x")
			y := stringGroundingWord(t, theory, "ground-contains-y")
			relation := stringContainmentRegister(t, theory, word, needle)
			concat, err := m.MkSequenceConcat(x, y)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(concat, false))
			left, right, find := astTestString(t, m, "A"), astTestString(t, m, "BC"), astTestString(t, m, needleText)
			for _, value := range []ast.ASTHandle{left, right, find} {
				astTestPin(t, m, value)
				astTestMust(t, state.Internalize(value, false))
			}
			g.variableAliases[word], g.variableAliases[needle] = wordAlias, needleAlias
			g.variableValues[x], g.variableValues[y], g.variableValues[needleAlias] = left, right, find
			g.variableConcats[wordAlias] = map[ast.ASTHandle]int{concat: 1}
			before := len(theory.refs)
			astTestMust(t, g.computeContains())
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			var axioms []ast.ASTHandle
			for _, ref := range theory.refs[before:] {
				value, err := m.RefGet(ref)
				astTestMust(t, err)
				sort, err := m.GetSort(value)
				astTestMust(t, err)
				if sort == boolean {
					axioms = append(axioms, value)
				}
			}
			if needleText == "absent" {
				if len(axioms) != 0 {
					t.Fatal("nonmatching partial grounding introduced a negative containment assertion")
				}
				return
			}
			if len(axioms) != 1 {
				t.Fatal("grounded containment did not publish its actual guarded assertion")
			}
			axiom := axioms[0]
			var conditions []ast.ASTHandle
			for _, pair := range [][2]ast.ASTHandle{{word, wordAlias}, {needle, needleAlias}, {wordAlias, concat}, {x, left}, {y, right}, {needleAlias, find}} {
				condition, err := stringEqAtom(m, pair[0], pair[1])
				astTestMust(t, err)
				conditions = append(conditions, condition)
			}
			for mask := 0; mask < 1<<len(conditions); mask++ {
				for _, assigned := range []bool{false, true} {
					bit, err := m.MkBoolVal(assigned)
					astTestMust(t, err)
					values := map[ast.ASTHandle]ast.ASTHandle{relation: bit}
					for i, condition := range conditions {
						value, err := m.MkBoolVal(mask&(1<<i) != 0)
						astTestMust(t, err)
						values[condition] = value
					}
					want := mask != (1<<len(conditions))-1 || assigned
					if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); got != want {
						t.Fatal("grounded containment dropped an alias/value/decomposition guard", mask, assigned, got, want)
					}
				}
			}
		})
	}
}

func TestSMTStringComputeContainsUnconditionalIdentity(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	g := newStringGrounding(theory)
	t.Cleanup(func() { astTestMust(t, g.close()) })
	word := stringGroundingWord(t, theory, "ground-contains-identity")
	relation := stringContainmentRegister(t, theory, word, word)
	before := len(theory.refs)
	astTestMust(t, g.computeContains())
	if len(theory.refs) != before+1 {
		t.Fatal("unguarded grounded identity did not publish exactly one unit")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	if axiom != relation {
		t.Fatal("unguarded grounded identity introduced an invented premise")
	}
	literal, err := state.Literal(relation)
	astTestMust(t, err)
	value, err := state.Assignment(literal)
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("actual containment unit did not reach Boolean assignment", value)
	}
}
