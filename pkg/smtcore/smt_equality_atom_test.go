package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTSearchEqualityArithmeticConventions(t *testing.T) {
	for _, backend := range []string{"unregistered", "dummy", "simplex", "difference", "dense", "utvpi"} {
		t.Run(backend, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			one, err := m.ArithmeticRealNumeralText("1")
			astTestMust(t, err)
			two, err := m.ArithmeticRealNumeralText("2")
			astTestMust(t, err)
			sort, err := m.GetSort(one)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "search-equality-x"), sort)
			astTestMust(t, err)
			y, err := m.MkConst(astTestSymbol(t, m, "search-equality-y"), sort)
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			var theory SMTTheoryInternalizer
			switch backend {
			case "dummy":
				theory, err = NewSMTDummyTheory(family, "arithmetic")
			case "simplex":
				theory, err = NewSMTTheoryArith(family, false)
			case "difference":
				theory, err = NewSMTRealDiffTheory(family)
			case "dense":
				theory, err = NewSMTDenseDiffTheory(family)
			case "utvpi":
				theory, err = NewSMTUTVPITheory(family)
			}
			astTestMust(t, err)
			if theory != nil {
				astTestMust(t, state.RegisterTheory(theory))
			}
			arithmetic := backend != "unregistered" && backend != "dummy"
			// Creation order above is one, two, x, y. The numeral precedes x
			// by AST ID, so numeral-right ordering differs from the default.
			numeralEquality, err := m.MkEq(one, x)
			astTestMust(t, err)
			reflexiveX, err := m.MkEq(x, x)
			astTestMust(t, err)
			reflexiveOne, err := m.MkEq(one, one)
			astTestMust(t, err)
			distinctNumerals, err := m.MkEq(one, two)
			astTestMust(t, err)
			if arithmetic {
				numeralEquality, err = m.MkEq(x, one)
				astTestMust(t, err)
				reflexiveX, err = m.MkTrue()
				astTestMust(t, err)
				reflexiveOne = reflexiveX
				distinctNumerals, err = m.MkFalse()
				astTestMust(t, err)
			}
			xy, err := m.MkEq(x, y)
			astTestMust(t, err)
			for _, example := range [][3]ast.ASTHandle{
				{x, one, numeralEquality}, {one, x, numeralEquality},
				{y, x, xy}, {x, y, xy}, {x, x, reflexiveX},
				{one, one, reflexiveOne}, {one, two, distinctNumerals}, {two, one, distinctNumerals},
			} {
				got, e := state.mkEqualityAtom(example[0], example[1])
				astTestMust(t, e)
				if got != example[2] {
					t.Fatalf("operands %v: equality atom %v, want %v", example[:2], got, example[2])
				}
			}
		})
	}
}
