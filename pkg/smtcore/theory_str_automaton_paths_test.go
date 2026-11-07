package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func stringPathAutomaton(t *testing.T, m *ast.ASTManager, regex ast.ASTHandle) *rewriter.StringRegexAutomaton {
	t.Helper()
	owner, err := rewriter.NewStringRegexAutomaton(m, regex, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, owner.Close()) })
	graph, err := owner.Graph()
	astTestMust(t, err)
	graph.Compress()
	return owner
}

func TestSMTStringAutomatonPathPredicateTranslationAndGuards(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	character := stringGroundingWord(t, theory, "aut-predicate-character")
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	sort := pin(m.BitVectorSort(8))
	first, other := pin(m.MkVar(0, sort)), pin(m.MkVar(7, sort))
	for _, test := range []struct {
		condition ast.ASTHandle
		text      string
		variable  bool
	}{
		{pin(m.BitVectorNumeralUnsigned(0, 8)), "\x00", false},
		{pin(m.BitVectorNumeralUnsigned(255, 8)), "\xff", false},
		{pin(m.BitVectorNumeralUnsigned(42, 16)), "*", false},
		{first, "", true}, {other, "", true},
	} {
		root, err := theory.rewriteStringAutomatonConstraint(test.condition, character)
		astTestMust(t, err)
		value, err := m.RefGet(root)
		astTestMust(t, err)
		if test.variable {
			if value != character {
				t.Fatal("native variable substitution added an index restriction")
			}
		} else {
			text, constant, err := m.SequenceStringValue(value)
			astTestMust(t, err)
			if !constant || text != test.text {
				t.Fatal("BV character was not translated to exact String bytes", text, test.text)
			}
		}
		astTestMust(t, m.DelRef(&root))
	}
	a, b := pin(m.BitVectorNumeralUnsigned('a', 8)), pin(m.BitVectorNumeralUnsigned('b', 8))
	equalA, equalB := pin(m.MkEq(first, a)), pin(m.MkEq(other, b))
	condition := pin(m.MkAnd(equalA, pin(m.MkNot(equalB))))
	root, err := theory.rewriteStringAutomatonConstraint(condition, character)
	astTestMust(t, err)
	translated, err := m.RefGet(root)
	astTestMust(t, err)
	for _, text := range []string{"a", "b", "c", "\x00", "\xff"} {
		if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{translated}, map[ast.ASTHandle]ast.ASTHandle{character: astTestString(t, m, text)}); got != (text == "a") {
			t.Fatal("Boolean predicate declaration/changed equality sort was not preserved", text, got)
		}
	}
	astTestMust(t, m.DelRef(&root))
	integer := pin(m.ArithmeticIntegerNumeralText("3"))
	root, err = theory.rewriteStringAutomatonConstraint(integer, character)
	astTestMust(t, err)
	value, err := m.RefGet(root)
	astTestMust(t, err)
	if !value.IsNull() {
		t.Fatal("unknown non-Boolean path predicate invented a translation")
	}
	astTestMust(t, m.DelRef(&root))
	tooLarge := pin(m.BitVectorNumeralUnsigned(256, 16))
	if root, err := theory.rewriteStringAutomatonConstraint(tooLarge, character); err == nil || root != (ast.ASTRef{}) {
		t.Fatal("native byte range guard was bypassed")
	}
	bvOrder := pin(m.MkBitVectorULE(first, a))
	if root, err := theory.rewriteStringAutomatonConstraint(bvOrder, character); err == nil || root != (ast.ASTRef{}) {
		t.Fatal("unsupported BV declaration was silently replaced with a String comparison")
	}
}

func TestSMTStringAutomatonPathNextConditionMergeOrder(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	x := stringGroundingWord(t, theory, "aut-next-x")
	a, err := m.MkEq(x, astTestString(t, m, "a"))
	astTestMust(t, err)
	astTestPin(t, m, a)
	b, err := m.MkEq(x, astTestString(t, m, "b"))
	astTestMust(t, err)
	astTestPin(t, m, b)
	p := smtStringAutomatonPath{theory: theory, locals: smtStringLengthPremises{theory: theory}}
	defer func() { astTestMust(t, p.locals.close()) }()
	next := make(map[uint32]ast.ASTHandle)
	astTestMust(t, p.addNext(next, 5, a))
	astTestMust(t, p.addNext(next, 5, b))
	want, err := m.MkOr(b, a)
	astTestMust(t, err)
	if next[5] != want {
		t.Fatal("new path condition must precede the existing accumulator")
	}
	truth, err := m.MkTrue()
	astTestMust(t, err)
	astTestMust(t, p.addNext(next, 5, truth))
	if next[5] != truth {
		t.Fatal("new true path must replace the previous accumulator")
	}
	astTestMust(t, p.addNext(next, 5, a))
	want, err = m.MkOr(a, truth)
	astTestMust(t, err)
	if next[5] != want {
		t.Fatal("existing true accumulator was normalized beyond native add_next")
	}
}

func TestSMTStringRegexPathConcreteLanguagesAndDecomposition(t *testing.T) {
	for _, name := range []string{"union", "range-any", "reversed-range", "star", "complement", "intersection", "nul-high"} {
		t.Run(name, func(t *testing.T) {
			m, theory, _, rw := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "aut-path-word")
			pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
				astTestMust(t, err)
				astTestPin(t, m, value)
				return value
			}
			literal := func(text string) ast.ASTHandle { return stringRegexLiteral(t, m, text) }
			var regex ast.ASTHandle
			switch name {
			case "union":
				regex = pin(m.MkRegexUnion(literal("ab"), literal("ac")))
			case "range-any":
				rangeRE := pin(m.MkRegexRange(astTestString(t, m, "a"), astTestString(t, m, "c")))
				sort, err := m.GetSort(rangeRE)
				astTestMust(t, err)
				regex = pin(m.MkRegexConcat(rangeRE, pin(m.MkRegexFullChar(sort))))
			case "reversed-range":
				regex = pin(m.MkRegexRange(astTestString(t, m, "c"), astTestString(t, m, "a")))
			case "star":
				regex = pin(m.MkRegexStar(literal("ab")))
			case "complement":
				regex = pin(m.MkRegexComplement(literal("a")))
			case "intersection":
				regex = pin(m.MkRegexIntersect(pin(m.MkRegexUnion(literal("a"), literal("b"))), pin(m.MkRegexUnion(literal("b"), literal("c")))))
			default:
				regex = literal("\x00\xff")
			}
			owner := stringPathAutomaton(t, m, regex)
			out, err := m.NewRef(ast.ASTHandle{})
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			maximumLength := uint32(3)
			if name == "star" {
				maximumLength = 4
			}
			for length := uint32(1); length <= maximumLength; length++ {
				root, err := theory.generateStringRegexPathConstraints(word, owner, length, out)
				astTestMust(t, err)
				path, err := m.RefGet(root)
				astTestMust(t, err)
				constraints, err := m.RefGet(out)
				astTestMust(t, err)
				negativePath := pin(m.MkNotSimplified(path))
				negativeConstraints := pin(m.MkNotSimplified(constraints))
				words := []string{""}
				for i := uint32(0); i < length; i++ {
					var next []string
					for _, prefix := range words {
						for _, symbol := range []byte{'a', 'b', 'c', 0, 255} {
							next = append(next, prefix+string([]byte{symbol}))
						}
					}
					words = next
				}
				for _, text := range words {
					accepted := false
					switch name {
					case "union":
						accepted = text == "ab" || text == "ac"
					case "range-any":
						accepted = len(text) == 2 && text[0] >= 'a' && text[0] <= 'c'
					case "reversed-range":
						// Native path generation swaps reversed range endpoints.
						accepted = len(text) == 1 && text[0] >= 'a' && text[0] <= 'c'
					case "star":
						accepted = len(text)%2 == 0 && text == strings.Repeat("ab", len(text)/2)
					case "complement":
						accepted = text != "a"
					case "intersection":
						accepted = text == "b"
					default:
						accepted = text == "\x00\xff"
					}
					for _, output := range []string{text, "#wrong#"} {
						values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, output)}
						for i, character := range theory.stringChars[word][:length] {
							values[character] = astTestString(t, m, text[i:i+1])
						}
						wantPath, wantCharacters := path, constraints
						if !accepted {
							wantPath = negativePath
						}
						if output != text {
							wantCharacters = negativeConstraints
						}
						if !stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{wantPath, wantCharacters}, values) {
							t.Fatal("finite regex path or String decomposition disagrees with independent byte language", name, length, []byte(text), output, accepted)
						}
					}
				}
				astTestMust(t, m.DelRef(&root))
			}
		})
	}
}

func TestSMTStringRegexPathRejectsUnreachedZeroLength(t *testing.T) {
	for _, accepts := range []bool{false, true} {
		m, theory, _, _ := stringLibraryFixture(t)
		word := stringGroundingWord(t, theory, "aut-zero-word")
		text := "a"
		if accepts {
			text = ""
		}
		owner := stringPathAutomaton(t, m, stringRegexLiteral(t, m, text))
		marker, err := m.MkTrue()
		astTestMust(t, err)
		out, err := m.NewRef(marker)
		astTestMust(t, err)
		root, err := theory.generateStringRegexPathConstraints(word, owner, 0, out)
		if err == nil || root != (ast.ASTRef{}) {
			t.Fatal("nonzero retained specialization accepted an unreached zero-length input")
		}
		actual, err := m.RefGet(out)
		astTestMust(t, err)
		if actual != marker || len(theory.stringChars) != 0 || theory.freshID != 0 {
			t.Fatal("zero-length path changed character output/cache or created witnesses")
		}
		astTestMust(t, m.DelRef(&out))
	}
}

func TestSMTStringRegexPathUnitLengthConstraints(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "aut-unit-word")
	literal := stringRegexLiteral(t, m, "a")
	sort, err := m.GetSort(literal)
	astTestMust(t, err)
	regex, err := m.MkRegexFullChar(sort)
	astTestMust(t, err)
	astTestPin(t, m, regex)
	owner := stringPathAutomaton(t, m, regex)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	root, err := theory.generateStringRegexPathConstraints(word, owner, 1, out)
	astTestMust(t, err)
	path, err := m.RefGet(root)
	astTestMust(t, err)
	if !m.IsTrue(path) {
		t.Fatal("full-character predicate introduced a nontrivial path restriction")
	}
	constraints, err := m.RefGet(out)
	astTestMust(t, err)
	for _, character := range []string{"", "x", "xx", "\x00", "\xff"} {
		for _, value := range []string{character, "wrong"} {
			values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, value), theory.stringChars[word][0]: astTestString(t, m, character)}
			if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{constraints}, values); got != (len(character) == 1 && value == character) {
				t.Fatal("character witness lost its exact one-byte length or word equality", character, value, got)
			}
		}
	}
	astTestMust(t, m.DelRef(&root))
	astTestMust(t, m.DelRef(&out))
}

func TestSMTStringRegexPathWitnessCacheRefreshGrowthAndShorterPrefix(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "aut-cache-word")
	base := stringRegexLiteral(t, m, "a")
	sort, err := m.GetSort(base)
	astTestMust(t, err)
	regex, err := m.MkRegexFull(sort)
	astTestMust(t, err)
	astTestPin(t, m, regex)
	owner := stringPathAutomaton(t, m, regex)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	astTestMust(t, state.PushScope())
	root, err := theory.generateStringRegexPathConstraints(word, owner, 2, out)
	astTestMust(t, err)
	old := append([]ast.ASTHandle(nil), theory.stringChars[word]...)
	if len(old) != 2 || old[0] != stringLibraryWitness(t, theory, "ch0!tmp0") || old[1] != stringLibraryWitness(t, theory, "ch1!tmp1") {
		t.Fatal("path witness names/order changed")
	}
	astTestMust(t, m.DelRef(&root))
	astTestMust(t, m.SetRef(out, ast.ASTHandle{}))
	astTestMust(t, state.PopScopes(1))
	for _, character := range old {
		if state.context.enodes.get(character) != nil {
			t.Fatal("scoped character retained its old enode")
		}
	}
	root, err = theory.generateStringRegexPathConstraints(word, owner, 1, out)
	astTestMust(t, err)
	if theory.freshID != 2 || len(theory.stringChars[word]) != 2 || state.context.enodes.get(old[0]) == nil || state.context.enodes.get(old[1]) != nil {
		t.Fatal("shorter reuse did not refresh only the requested prefix")
	}
	astTestMust(t, m.DelRef(&root))
	root, err = theory.generateStringRegexPathConstraints(word, owner, 3, out)
	astTestMust(t, err)
	characters := theory.stringChars[word]
	if len(characters) != 3 || characters[0] != old[0] || characters[1] != old[1] || theory.freshID != 3 {
		t.Fatal("cached growth replaced existing witnesses")
	}
	count := 0
	for _, ref := range theory.refs {
		value, err := m.RefGet(ref)
		astTestMust(t, err)
		if value == characters[2] {
			count++
		}
	}
	if count != 2 {
		t.Fatal("existing-cache growth omitted native additional lifetime trail root", count)
	}
	constraints, err := m.RefGet(out)
	astTestMust(t, err)
	decomposition, err := m.GetArg(constraints, 1)
	astTestMust(t, err)
	concat, err := m.GetArg(decomposition, 1)
	astTestMust(t, err)
	left, right, matched, err := m.SequenceConcatParts(concat)
	astTestMust(t, err)
	if !matched || right != characters[2] {
		t.Fatal("path character concat was not left-associated")
	}
	first, second, matched, err := m.SequenceConcatParts(left)
	astTestMust(t, err)
	if !matched || first != characters[0] || second != characters[1] {
		t.Fatal("path character concat reordered its prefix")
	}
	for _, character := range characters {
		if state.context.enodes.get(character) == nil {
			t.Fatal("cached character was not refreshed")
		}
	}
	astTestMust(t, m.DelRef(&root))
}

func TestSMTStringRegexPathUnsupportedPredicateKeepsOutputAndWitnesses(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "aut-unsupported-word")
	rangeRE, err := m.MkRegexRange(astTestString(t, m, "a"), astTestString(t, m, "z"))
	astTestMust(t, err)
	astTestPin(t, m, rangeRE)
	regex, err := m.MkRegexIntersect(rangeRE, stringRegexLiteral(t, m, "m"))
	astTestMust(t, err)
	astTestPin(t, m, regex)
	owner := stringPathAutomaton(t, m, regex)
	marker, err := m.MkTrue()
	astTestMust(t, err)
	out, err := m.NewRef(marker)
	astTestMust(t, err)
	root, err := theory.generateStringRegexPathConstraints(word, owner, 1, out)
	if err == nil || root != (ast.ASTRef{}) {
		t.Fatal("unsupported translated BV range predicate invented an accepted path")
	}
	actual, err := m.RefGet(out)
	astTestMust(t, err)
	if actual != marker || len(theory.stringChars[word]) != 1 || theory.freshID != 1 {
		t.Fatal("failed predicate translation changed output or rolled back native witness creation")
	}
	supported := stringPathAutomaton(t, m, stringRegexLiteral(t, m, "m"))
	root, err = theory.generateStringRegexPathConstraints(word, supported, 1, out)
	astTestMust(t, err)
	if theory.freshID != 1 {
		t.Fatal("subsequent generation did not reuse the retained character witness")
	}
	astTestMust(t, m.DelRef(&root))
	astTestMust(t, m.DelRef(&out))
}

func TestSMTStringRegexPathNonValueCharacterAndOwnerGuards(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "aut-symbolic-char-word")
	sort, err := m.BitVectorSort(8)
	astTestMust(t, err)
	element, err := m.MkVar(3, sort)
	astTestMust(t, err)
	unit, err := m.MkSequenceUnit(element)
	astTestMust(t, err)
	regex, err := m.MkSequenceToRegex(unit)
	astTestMust(t, err)
	astTestPin(t, m, regex)
	owner := stringPathAutomaton(t, m, regex)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	if root, err := theory.generateStringRegexPathConstraints(word, nil, 1, out); err == nil || root != (ast.ASTRef{}) || theory.freshID != 0 {
		t.Fatal("nil automaton bypassed its owner guard or created witnesses")
	}
	root, err := theory.generateStringRegexPathConstraints(word, owner, 1, out)
	astTestMust(t, err)
	path, err := m.RefGet(root)
	astTestMust(t, err)
	if !m.IsFalse(path) {
		t.Fatal("non-value character label acquired an invented native path case")
	}
	constraints, err := m.RefGet(out)
	astTestMust(t, err)
	if constraints.IsNull() || len(theory.stringChars[word]) != 1 {
		t.Fatal("missing path skipped native character decomposition output")
	}
	astTestMust(t, owner.Close())
	if root, err := theory.generateStringRegexPathConstraints(word, owner, 1, out); err == nil || root != (ast.ASTRef{}) {
		t.Fatal("closed automaton exposed path-generation state")
	}
	astTestMust(t, m.DelRef(&root))
	astTestMust(t, m.DelRef(&out))
}
