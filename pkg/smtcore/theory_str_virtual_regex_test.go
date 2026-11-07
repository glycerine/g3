package smtcore

import (
	"errors"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringUnrollConstructorDefinitionAndOwnership(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	bound := stringLibraryInteger(t, m, theory.state, "constructor-count")
	regex := stringRegexLiteral(t, m, "ab")
	term, err := theory.makeStringUnroll(regex, bound)
	astTestMust(t, err)
	formula, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	length, err := m.MkSequenceLength(term)
	astTestMust(t, err)
	roots := 0
	for _, ref := range theory.refs {
		value, err := m.RefGet(ref)
		astTestMust(t, err)
		if value == term {
			roots++
		}
	}
	if roots != 1 || theory.state.context.enodes.get(term) == nil {
		t.Fatal("unroll constructor lost explicit root or assertion internalization")
	}
	for count := -1; count <= 2; count++ {
		for _, text := range []string{"", "a"} {
			for size := -1; size <= 1; size++ {
				c, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(count))
				astTestMust(t, err)
				n, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(size))
				astTestMust(t, err)
				want := count >= 0 && size >= 0 && ((count == 0) == (text == ""))
				if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{term: astTestString(t, m, text), bound: c, length: n}); got != want {
					t.Fatal("unroll definition", count, text, size, got, want)
				}
			}
		}
	}
}

func TestSMTStringVirtualRegexLiteralUnionAndConcat(t *testing.T) {
	for _, kind := range []string{"literal", "union", "concat"} {
		t.Run(kind, func(t *testing.T) {
			m, theory, _, rw := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "virtual-word")
			left, right := stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "bb")
			regex := left
			var err error
			if kind == "union" {
				regex, err = m.MkRegexUnion(left, right)
			} else if kind == "concat" {
				regex, err = m.MkRegexConcat(left, right)
			}
			astTestMust(t, err)
			items := smtStringLengthPremises{theory: theory}
			defer func() { astTestMust(t, items.close()) }()
			astTestMust(t, theory.reduceStringVirtualRegexIn(word, regex, &items))
			wantItems, wantFresh := 2, uint32(0)
			if kind == "union" {
				wantItems, wantFresh = 7, 2
			} else if kind == "concat" {
				wantItems, wantFresh = 6, 2
			}
			if len(items.values) != wantItems || theory.freshID != wantFresh {
				t.Fatal("virtual reduction shape/fresh order", len(items.values), theory.freshID)
			}
			values := map[ast.ASTHandle]ast.ASTHandle{}
			if wantFresh > 0 {
				first := stringLibraryWitness(t, theory, "regex!tmp0")
				second := stringLibraryWitness(t, theory, "regex!tmp1")
				for _, term := range []ast.ASTHandle{first, second} {
					_, role := theory.regexVariables[term]
					_, internal := theory.internalVariables[term]
					if !role || internal {
						t.Fatal("virtual witness lost regex role")
					}
				}
				values[first], values[second] = astTestString(t, m, "a"), astTestString(t, m, "bb")
			}
			for _, text := range []string{"", "a", "bb", "abb", "bba", "wrong"} {
				values[word] = astTestString(t, m, text)
				want := kind == "literal" && text == "a" || kind == "union" && (text == "a" || text == "bb") || kind == "concat" && text == "abb"
				if got := stringLibraryAxiomsTruth(t, m, rw, items.values, values); got != want {
					t.Fatal("virtual regex word", kind, text, got, want)
				}
			}
		})
	}
}

func TestSMTStringVirtualRegexRangeAssertsDirectlyAndPreservesBytes(t *testing.T) {
	for _, bounds := range [][2]string{{"z", "x"}, {"\x00", "\x01"}, {"\xfe", "\xff"}} {
		t.Run(bounds[0], func(t *testing.T) {
			m, theory, _, rw := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "virtual-range")
			regex, err := m.MkRegexRange(astTestString(t, m, bounds[0]), astTestString(t, m, bounds[1]))
			astTestMust(t, err)
			items := smtStringLengthPremises{theory: theory}
			defer func() { astTestMust(t, items.close()) }()
			accesses, cache := theory.cacheAccesses, len(theory.strings)
			astTestMust(t, theory.reduceStringVirtualRegexIn(word, regex, &items))
			if len(items.values) != 0 || theory.cacheAccesses != accesses || len(theory.strings) != cache {
				t.Fatal("range was appended as a guarded premise or used theory String cache")
			}
			formula, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			lo, hi := bounds[0][0], bounds[1][0]
			if lo > hi {
				lo, hi = hi, lo
			}
			for ch := 0; ch < 256; ch++ {
				got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, string([]byte{byte(ch)}))})
				if got != (byte(ch) >= lo && byte(ch) <= hi) {
					t.Fatal("virtual range byte", bounds, ch, got)
				}
			}
			for _, text := range []string{"", "xx"} {
				if stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text)}) {
					t.Fatal("range accepted non-single-byte word", text)
				}
			}
		})
	}
}

func TestSMTStringVirtualRegexStarUsesActualUnrollConstructor(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "virtual-star")
	child := stringRegexLiteral(t, m, "ab")
	regex, err := m.MkRegexStar(child)
	astTestMust(t, err)
	items := smtStringLengthPremises{theory: theory}
	defer func() { astTestMust(t, items.close()) }()
	before := len(theory.refs)
	astTestMust(t, theory.reduceStringVirtualRegexIn(word, regex, &items))
	if len(items.values) != 2 || theory.freshID != 1 {
		t.Fatal("star reduction did not emit String/length equalities")
	}
	bound := stringLibraryWitness(t, theory, "unroll!tmp0")
	var unroll ast.ASTHandle
	for _, ref := range theory.refs[before:] {
		term, err := m.RefGet(ref)
		astTestMust(t, err)
		op, own, err := theory.operation(term)
		astTestMust(t, err)
		if own && op == 43 {
			unroll = term
		}
	}
	if unroll.IsNull() {
		t.Fatal("star reduction did not retain unroll term")
	}
	arg, err := m.GetArg(unroll, 0)
	astTestMust(t, err)
	if arg != child {
		t.Fatal("wrong unroll regex")
	}
	arg, err = m.GetArg(unroll, 1)
	astTestMust(t, err)
	if arg != bound {
		t.Fatal("wrong unroll bound")
	}
	if state := theory.state; state.context.enodes.get(unroll) == nil {
		t.Fatal("constructor axioms did not internalize unroll")
	}
}

func TestSMTStringVirtualRegexUnsupportedPreservesCompletedPrefix(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "virtual-error")
	left, err := m.MkRegexRange(astTestString(t, m, "a"), astTestString(t, m, "b"))
	astTestMust(t, err)
	right, err := m.MkRegexPlus(stringRegexLiteral(t, m, "c"))
	astTestMust(t, err)
	regex, err := m.MkRegexUnion(left, right)
	astTestMust(t, err)
	items := smtStringLengthPremises{theory: theory}
	defer func() { astTestMust(t, items.close()) }()
	before := len(theory.refs)
	if err := theory.reduceStringVirtualRegexIn(word, regex, &items); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unsupported native operator was silently expanded", err)
	}
	if len(items.values) != 3 || theory.freshID != 2 || len(theory.refs) <= before+2 {
		t.Fatal("error rolled back native completed union/range side effects")
	}
	formula, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	kind, err := m.GetDeclKind(formula)
	astTestMust(t, err)
	if kind != ast.ASTOpOr {
		t.Fatal("left range assertion missing before unsupported right child")
	}
}
