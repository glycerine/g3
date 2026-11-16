package smtcore

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringLiteralUnrollCompatibleCommonPeriods(t *testing.T) {
	for _, test := range []struct {
		cores  []string
		common string
	}{
		{[]string{"ab", "abab"}, "abab"},
		{[]string{"ab", "ababab"}, "ababab"},
		{[]string{"a", "aa", "aaa"}, "aaaaaa"},
		{[]string{"\x00\xff", "\x00\xff\x00\xff"}, "\x00\xff\x00\xff"},
	} {
		t.Run(test.common, func(t *testing.T) {
			m, theory, _, rw := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "literal-unroll-period")
			count, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			var unrolls []ast.ASTHandle
			for _, core := range test.cores {
				unrolls = append(unrolls, stringUnrollTerm(t, theory, stringRegexLiteral(t, m, core), count))
			}
			root, err := theory.generateStringLiteralUnrollAssignment(word, unrolls)
			astTestMust(t, err)
			formula, err := m.RefGet(root)
			astTestMust(t, err)
			history := theory.testers.unrollHistories[word][0]
			if len(history.testers) != 1 {
				t.Fatal("compatible periods did not enter actual conditional tester")
			}
			tester := history.testers[0]
			length, err := m.MkSequenceLength(word)
			astTestMust(t, err)
			for _, text := range []string{"", test.common, test.common + test.common, "wrong"} {
				value := astTestString(t, m, text)
				number, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(len(text)))
				astTestMust(t, err)
				values := map[ast.ASTHandle]ast.ASTHandle{word: value, length: number, tester: astTestString(t, m, "1")}
				for _, unroll := range unrolls {
					values[unroll] = value
				}
				if got := stringAxiomTruth(t, m, rw, formula, values); got != (text == test.common) {
					t.Fatal("least common period", test.cores, text, got)
				}
			}
			astTestMust(t, m.DelRef(&root))
		})
	}
}

func TestSMTStringLiteralUnrollIncompatibleGuardAndOwnership(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "literal-unroll-incompatible")
	count, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	u1 := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "ab"), count)
	u2 := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "aba"), count)
	fresh := theory.freshID
	root, err := theory.generateStringLiteralUnrollAssignment(word, []ast.ASTHandle{u2, u1})
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	if theory.freshID != fresh || len(theory.testers.unrollHistories[word]) != 0 {
		t.Fatal("incompatible periods invented a tester")
	}
	for _, test := range []struct {
		text, other string
		want        bool
	}{{"", "", true}, {"ab", "ab", false}, {"ab", "different", true}} {
		values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, test.text), u1: astTestString(t, m, test.text), u2: astTestString(t, m, test.other)}
		if got := stringAxiomTruth(t, m, rw, formula, values); got != test.want {
			t.Fatal("incompatible period implication", test, got)
		}
	}
	for _, ref := range theory.refs {
		term, err := m.RefGet(ref)
		astTestMust(t, err)
		if term == formula {
			t.Fatal("incompatible implication gained an extra direct lifetime root")
		}
	}
	astTestMust(t, m.DelRef(&root))
}

func TestSMTStringSimpleUnrollCollectorPreservesToRegexClassification(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "simple-unroll-class")
	symbolic := stringGroundingWord(t, theory, "symbolic-unroll-core")
	count, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	symbolicRegex, err := m.MkSequenceToRegex(symbolic)
	astTestMust(t, err)
	literalRegex := stringRegexLiteral(t, m, "ab")
	star, err := m.MkRegexStar(literalRegex)
	astTestMust(t, err)
	u1 := stringUnrollTerm(t, theory, literalRegex, count)
	u2 := stringUnrollTerm(t, theory, symbolicRegex, count)
	u3 := stringUnrollTerm(t, theory, star, count)
	u4 := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, ""), count)
	constant := astTestString(t, m, "class-value")
	astTestMust(t, state.Internalize(constant, false))
	astTestMust(t, state.PushScope())
	for _, member := range []ast.ASTHandle{u1, u2, u3, u4, constant} {
		stringDependencyPrivateMerge(t, theory, word, member)
	}
	value, unrolls, err := theory.stringClassSimpleUnrolls(word)
	astTestMust(t, err)
	want := []ast.ASTHandle{u1, u2, u4}
	stringSortHandles(theory, want)
	if value != constant || !reflect.DeepEqual(unrolls, want) {
		t.Fatal("simple collection added a literal/nonempty filter or retained a star core", value, unrolls, want)
	}
	astTestMust(t, state.PopScopes(1))
	value, unrolls, err = theory.stringClassSimpleUnrolls(word)
	astTestMust(t, err)
	if !value.IsNull() || len(unrolls) != 0 {
		t.Fatal("private-class scope rollback not observed")
	}
}

func TestSMTStringLiteralUnrollUndefinedProducerGuardsAndLCM(t *testing.T) {
	for _, test := range []struct{ a, b, want int32 }{{1, 2, 2}, {2, 4, 4}, {4, 6, 12}, {12, 18, 36}, {0, 5, 0}, {0, 0, 0}} {
		got, err := stringUnrollLCM(test.a, test.b)
		astTestMust(t, err)
		if got != test.want {
			t.Fatal("source LCM", test, got)
		}
	}
	if _, err := stringUnrollLCM(46341, 46342); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("undefined native signed LCM overflow accepted", err)
	}
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "literal-unroll-invalid")
	count, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	symbolic, err := m.MkSequenceToRegex(word)
	astTestMust(t, err)
	for _, unrolls := range [][]ast.ASTHandle{nil, {stringUnrollTerm(t, theory, stringRegexLiteral(t, m, ""), count)}, {stringUnrollTerm(t, theory, symbolic, count)}} {
		if _, err := theory.generateStringLiteralUnrollAssignment(word, unrolls); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("native empty/symbolic division state was fabricated", err)
		}
	}
	if len(theory.testers.unrollHistories[word]) != 0 {
		t.Fatal("invalid period fabricated a tester history")
	}
}
