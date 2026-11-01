package model

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: model_implicant.cpp:382-415, rational.h:176-195, mpz.h:468-498.
func TestModelImplicantEuclideanIntegerDivision(t *testing.T) {
	cases := []struct{ a, b, quotient, modulus, remainder string }{
		{"7", "3", "2", "1", "1"},
		{"-7", "3", "-3", "2", "2"},
		{"7", "-3", "-2", "1", "-1"},
		{"-7", "-3", "3", "2", "-2"},
		{"6", "-3", "-2", "0", "0"},
		{"-6", "-3", "2", "0", "0"},
		{"0", "-3", "0", "0", "0"},
		{"-10000000000000000000000000000000000000001", "-3", "3333333333333333333333333333333333333334", "1", "-1"},
		{"7", "0", "", "", ""},
	}
	for _, c := range cases {
		for _, op := range []int32{11, 12, 13} {
			t.Run(fmt.Sprintf("%s/%s/op=%d", c.a, c.b, op), func(t *testing.T) {
				m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
				var pins []ast.ASTRef
				defer func() {
					for n := range pins {
						arrayTableMust(t, m.DelRef(&pins[n]))
					}
				}()
				own := func(h ast.ASTHandle, err error) ast.ASTHandle {
					t.Helper()
					arrayTableMust(t, err)
					r, err := m.NewRef(h)
					arrayTableMust(t, err)
					pins = append(pins, r)
					return h
				}
				a := own(m.ArithmeticIntegerNumeralText(c.a))
				b := own(m.ArithmeticIntegerNumeralText(c.b))
				family, err := m.ArithmeticFamilyID()
				arrayTableMust(t, err)
				expression := own(m.MkFamilyApp(family, op, nil, []ast.ASTHandle{a, b}))
				expected := c.quotient
				if op == 12 {
					expected = c.remainder
				} else if op == 13 {
					expected = c.modulus
				}
				text := expected
				if text == "" {
					text = "0"
				}
				numeral := own(m.ArithmeticIntegerNumeralText(text))
				formula := own(m.MkEq(expression, numeral))
				before := m.Stats()
				arrayTableMust(t, implicant.setupModel(md))
				satisfied, err := implicant.checkModel([]ast.ASTHandle{formula})
				arrayTableMust(t, err)
				if expected == "" {
					if satisfied || !implicant.isX(expression) || !implicant.isX(formula) {
						t.Fatal("zero divisor did not propagate unknown")
					}
				} else {
					want, _ := new(big.Rat).SetString(expected)
					got := implicant.numbers[expression]
					if got == nil || got.Cmp(want) != 0 || !satisfied || !implicant.isTrue(formula) {
						t.Fatal("incorrect source integer arithmetic", got, expected, satisfied)
					}
					// Exercise the same arithmetic through the public literal collector.
					arrayTableMust(t, implicant.reset())
					result, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
					arrayTableMust(t, err)
					if len(result) != 1 {
						t.Fatal("arithmetic equality was not selected", len(result))
					}
					h, err := m.RefGet(result[0])
					arrayTableMust(t, err)
					if h != formula {
						t.Fatal("collector changed the satisfied equality")
					}
					arrayTableMust(t, CloseImplicantResults(m, &result))
				}
				arrayTableMust(t, implicant.reset())
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("integer arithmetic leaked references", after, before)
				}
			})
		}
	}
}
