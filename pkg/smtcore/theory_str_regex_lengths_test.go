package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringRegexLengthLinearityAndFiniteSets(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	a, bb, ccc := stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "bb"), stringRegexLiteral(t, m, "ccc")
	empty := stringRegexLiteral(t, m, "")
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	all, char := pin(m.MkRegexFull(sort)), pin(m.MkRegexFullChar(sort))
	rangeRE := pin(m.MkRegexRange(astTestString(t, m, "z"), astTestString(t, m, "a")))
	union := pin(m.MkRegexUnion(a, bb))
	duplicate := pin(m.MkRegexUnion(bb, stringRegexLiteral(t, m, "xy")))
	other := pin(m.MkRegexUnion(bb, ccc))
	product := pin(m.MkRegexConcat(union, other))
	star := pin(m.MkRegexStar(union))
	plus := pin(m.MkRegexPlus(union))
	nested := pin(m.MkRegexStar(pin(m.MkRegexConcat(a, star))))
	siblings := pin(m.MkRegexConcat(star, plus))
	complement := pin(m.MkRegexComplement(a))
	option := pin(m.MkRegexOption(a))
	for _, test := range []struct {
		name    string
		regex   ast.ASTHandle
		linear  bool
		invalid bool
		lengths []int32
	}{
		{"literal", a, true, false, []int32{1}},
		{"empty-word", empty, true, false, []int32{0}},
		{"union", union, true, false, []int32{1, 2}},
		{"duplicate-length", duplicate, true, false, []int32{2}},
		{"pairwise-concat", product, true, false, []int32{3, 4, 5}},
		{"star", star, true, false, nil},
		{"plus", plus, true, false, nil},
		{"nested-star", nested, false, false, nil},
		{"sibling-stars", siblings, true, false, nil},
		{"failed-union-propagates", pin(m.MkRegexUnion(a, star)), true, false, nil},
		{"failed-concat-propagates", pin(m.MkRegexConcat(a, all)), true, false, nil},
		{"all", all, true, false, nil},
		{"char", char, true, false, []int32{1}},
		{"reversed-range-still-length-one", rangeRE, true, false, []int32{1}},
		{"complement", complement, false, false, nil},
		{"linearity-short-circuit", pin(m.MkRegexConcat(complement, option)), false, false, nil},
		// smtcore_13-F3: UNREACHABLE(); return false; in the release build.
		{"option-unknown-to-linearity", option, false, false, nil},
		{"inter-unknown-to-linearity", pin(m.MkRegexIntersect(a, bb)), false, false, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			linear, err := theory.checkRegexLengthLinearity(test.regex, false)
			if (err != nil) != test.invalid || linear != test.linear {
				t.Fatal("native regex length-linearity classification changed", linear, err)
			}
			lengths, err := theory.finiteRegexSubtermLengths(test.regex)
			astTestMust(t, err)
			if len(lengths) != len(test.lengths) {
				t.Fatal("finite subterm length set changed", lengths, test.lengths)
			}
			for _, length := range test.lengths {
				if _, present := lengths[length]; !present {
					t.Fatal("missing finite subterm length", length)
				}
			}
		})
	}
}

func TestSMTStringRegexLengthConcatWitnessOrderAndFormula(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	length := stringLibraryInteger(t, m, state, "regex-concat-length")
	inner := pin(m.MkRegexConcat(stringRegexLiteral(t, m, "bc"), stringRegexLiteral(t, m, "d")))
	regex := pin(m.MkRegexConcat(stringRegexLiteral(t, m, "a"), inner))
	var free []ast.ASTHandle
	root, err := theory.inferStringRegexLengths(length, regex, &free)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
	if len(free) != 4 {
		t.Fatal("nested concat did not create exactly two variables per concat", len(free))
	}
	for i, name := range []string{"rlen1!tmp0", "rlen2!tmp1", "rlen1!tmp2", "rlen2!tmp3"} {
		if free[i] != stringLibraryWitness(t, theory, name) {
			t.Fatal("regex concat witness creation order changed", i, name)
		}
	}
	axiom, err := m.RefGet(root)
	astTestMust(t, err)
	number := func(value int) ast.ASTHandle {
		result, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(value))
		astTestMust(t, err)
		astTestPin(t, m, result)
		return result
	}
	for total := 0; total <= 6; total++ {
		for left := 0; left <= 3; left++ {
			for right := 0; right <= 4; right++ {
				values := map[ast.ASTHandle]ast.ASTHandle{length: number(total), free[0]: number(left), free[1]: number(right), free[2]: number(2), free[3]: number(1)}
				want := total == left+right && left == 1 && right == 3
				if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); got != want {
					t.Fatal("nested concat length formula changed", total, left, right, got, want)
				}
			}
		}
	}
}

func TestSMTStringRegexLengthStarPlusNativeCombination(t *testing.T) {
	for _, plus := range []bool{false, true} {
		m, theory, state, rw := stringLibraryFixture(t)
		length := stringLibraryInteger(t, m, state, "regex-star-length")
		child, err := m.MkRegexUnion(stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "bb"))
		astTestMust(t, err)
		astTestPin(t, m, child)
		var regex ast.ASTHandle
		if plus {
			regex, err = m.MkRegexPlus(child)
		} else {
			regex, err = m.MkRegexStar(child)
		}
		astTestMust(t, err)
		astTestPin(t, m, regex)
		var free []ast.ASTHandle
		root, err := theory.inferStringRegexLengths(length, regex, &free)
		astTestMust(t, err)
		if len(free) != 2 || free[0] != stringLibraryWitness(t, theory, "rstar!tmp0") || free[1] != stringLibraryWitness(t, theory, "rstar!tmp1") {
			t.Fatal("star/plus did not create one counter per distinct length")
		}
		axiom, err := m.RefGet(root)
		astTestMust(t, err)
		number := func(value int) ast.ASTHandle {
			result, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(value))
			astTestMust(t, err)
			astTestPin(t, m, result)
			return result
		}
		for first := -1; first <= 2; first++ {
			for second := -1; second <= 2; second++ {
				want := first + 2*second
				if plus {
					want += 3 // Source adds the 1 and 2 base contributions separately.
				}
				for total := -3; total <= 10; total++ {
					values := map[ast.ASTHandle]ast.ASTHandle{length: number(total), free[0]: number(first), free[1]: number(second)}
					if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); got != (total == want) {
						t.Fatal("native star/plus combination or separately-owned nonnegativity constraints changed", plus, first, second, total, got, want)
					}
				}
			}
		}
		astTestMust(t, m.DelRef(&root))
	}
}

func TestSMTStringRegexLengthLeafUnionAndFallbackFormulas(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	length := stringLibraryInteger(t, m, state, "regex-leaf-length")
	a := stringRegexLiteral(t, m, "a")
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	all := pin(m.MkRegexFull(sort))
	for _, test := range []struct {
		name   string
		regex  ast.ASTHandle
		length int
	}{
		{"nul-byte-literal", stringRegexLiteral(t, m, "\x00\xff"), 2},
		{"empty-literal", stringRegexLiteral(t, m, ""), 0},
		{"union", pin(m.MkRegexUnion(a, stringRegexLiteral(t, m, "abc"))), -2},
		{"range", pin(m.MkRegexRange(astTestString(t, m, "a"), astTestString(t, m, "z"))), 1},
		{"full-char", pin(m.MkRegexFullChar(sort)), 1},
		{"full-seq", all, -1},
		{"complement", pin(m.MkRegexComplement(a)), -1},
		{"unknown-option", pin(m.MkRegexOption(a)), -1},
		{"star-unbounded-subterm", pin(m.MkRegexStar(all)), -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var free []ast.ASTHandle
			root, err := theory.inferStringRegexLengths(length, test.regex, &free)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&root)) }()
			if len(free) != 0 {
				t.Fatal("leaf/union/fallback created unnecessary free variables")
			}
			axiom, err := m.RefGet(root)
			astTestMust(t, err)
			for n := -1; n <= 4; n++ {
				numeral, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(n))
				astTestMust(t, err)
				want := n == test.length
				if test.length == -1 {
					want = n >= 0
				} else if test.length == -2 {
					want = n == 1 || n == 3
				}
				if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, map[ast.ASTHandle]ast.ASTHandle{length: numeral}); got != want {
					t.Fatal("native regex leaf/union/fallback formula changed", n, got, want)
				}
			}
		})
	}
}

func TestSMTStringRegexLengthGenerationPreconditionsAndPartialWitnesses(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	length := stringLibraryInteger(t, m, state, "regex-guard-length")
	word := stringGroundingWord(t, theory, "regex-symbolic-word")
	symbolic, err := m.MkSequenceToRegex(word)
	astTestMust(t, err)
	astTestPin(t, m, symbolic)
	if linear, err := theory.checkRegexLengthLinearity(symbolic, false); err != nil || !linear {
		t.Fatal("linearity added a constant-word guard absent from the source")
	}
	if _, err := theory.finiteRegexSubtermLengths(symbolic); err == nil {
		t.Fatal("finite length inference bypassed the native constant-word precondition")
	}
	valid := stringRegexLiteral(t, m, "a")
	if root, err := theory.inferStringRegexLengths(length, valid, nil); err == nil || root != (ast.ASTRef{}) {
		t.Fatal("nil free-variable output bypassed the Go ownership contract")
	}
	concat, err := m.MkRegexConcat(valid, symbolic)
	astTestMust(t, err)
	astTestPin(t, m, concat)
	var free []ast.ASTHandle
	if root, err := theory.inferStringRegexLengths(length, concat, &free); err == nil || root != (ast.ASTRef{}) || len(free) != 2 {
		t.Fatal("partial generation rolled back native fresh witnesses or returned an unowned formula", root, err, len(free))
	}
	for _, variable := range free {
		if _, err := m.GetSort(variable); err != nil {
			t.Fatal("partial generation released the theory-owned witness", err)
		}
	}
}

func TestSMTStringRegexLengthZeroOptionAndDeduplicatedCounters(t *testing.T) {
	for _, plus := range []bool{false, true} {
		m, theory, state, rw := stringLibraryFixture(t)
		length := stringLibraryInteger(t, m, state, "regex-zero-length")
		duplicate, err := m.MkRegexUnion(stringRegexLiteral(t, m, "ab"), stringRegexLiteral(t, m, "cd"))
		astTestMust(t, err)
		astTestPin(t, m, duplicate)
		child, err := m.MkRegexUnion(stringRegexLiteral(t, m, ""), duplicate)
		astTestMust(t, err)
		astTestPin(t, m, child)
		var regex ast.ASTHandle
		if plus {
			regex, err = m.MkRegexPlus(child)
		} else {
			regex, err = m.MkRegexStar(child)
		}
		astTestMust(t, err)
		astTestPin(t, m, regex)
		var free []ast.ASTHandle
		root, err := theory.inferStringRegexLengths(length, regex, &free)
		astTestMust(t, err)
		if len(free) != 2 {
			t.Fatal("zero length counter was omitted or duplicate lengths created extra counters", len(free))
		}
		axiom, err := m.RefGet(root)
		astTestMust(t, err)
		number := func(value int) ast.ASTHandle {
			result, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(value))
			astTestMust(t, err)
			astTestPin(t, m, result)
			return result
		}
		for _, zeroCounter := range []int{-7, 0, 4} {
			for _, nonzeroCounter := range []int{-1, 0, 2} {
				want := 2 * nonzeroCounter
				if plus {
					want += 2
				}
				for _, output := range []int{want, want + 1} {
					values := map[ast.ASTHandle]ast.ASTHandle{length: number(output), free[0]: number(zeroCounter), free[1]: number(nonzeroCounter)}
					if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, values); got != (output == want) {
						t.Fatal("zero coefficient or distinct-length counter generation changed", plus, zeroCounter, nonzeroCounter, output, got)
					}
				}
			}
		}
		astTestMust(t, m.DelRef(&root))
	}
}
