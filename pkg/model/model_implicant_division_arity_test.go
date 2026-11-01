package model

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source: ast.cpp:2188-2192 expands left-associative division to binary
// applications, satisfying model_implicant.cpp:373,383 on well-sorted inputs.
func TestModelImplicantDivisionUsesSourceBinaryConstruction(t *testing.T) {
	cases := []struct {
		args          []string
		integer, real string
	}{
		{[]string{"9", "2", "3"}, "1", "3/2"},
		{[]string{"-7", "-3", "2"}, "1", "7/6"},
		{[]string{"7", "-3", "2"}, "-1", "-7/6"},
		{[]string{"100", "3", "2", "2"}, "8", "25/3"},
		{[]string{"7", "0", "2"}, "", ""},
	}
	for _, real := range []bool{false, true} {
		for _, assertions := range []bool{true, false} {
			for _, c := range cases {
				t.Run(fmt.Sprintf("real=%t/assertions=%t/%v", real, assertions, c.args), func(t *testing.T) {
					m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
					_, input := modelImplicantVerificationFixture(t, m, "T")
					arrayTableMust(t, diagnostic.BorrowProcessDebugContext().EnableAssertions(assertions))
					var pins []ast.ASTRef
					defer func() {
						for n := range pins {
							arrayTableMust(t, m.DelRef(&pins[n]))
						}
					}()
					own := func(h ast.ASTHandle, err error) ast.ASTHandle {
						t.Helper()
						arrayTableMust(t, err)
						ref, err := m.NewRef(h)
						arrayTableMust(t, err)
						pins = append(pins, ref)
						return h
					}
					numeral := func(text string) ast.ASTHandle {
						if real {
							return own(m.ArithmeticRealNumeralText(text))
						}
						return own(m.ArithmeticIntegerNumeralText(text))
					}
					args := make([]ast.ASTHandle, len(c.args))
					for n, text := range c.args {
						args[n] = numeral(text)
					}
					bound := numeral("1000")
					family, err := m.ArithmeticFamilyID()
					arrayTableMust(t, err)
					op, want := int32(11), c.integer
					if real {
						op, want = 10, c.real
					}
					expression := own(m.MkFamilyApp(family, op, nil, args))
					cursor := expression
					for n := len(args) - 1; n >= 1; n-- {
						count, err := m.GetNumArgs(cursor)
						arrayTableMust(t, err)
						if count != 2 {
							t.Fatal("division did not use binary source construction", count)
						}
						kind, err := m.GetDeclKind(cursor)
						arrayTableMust(t, err)
						if kind != op {
							t.Fatal("constructor changed division operator", kind, op)
						}
						right, err := m.GetArg(cursor, 1)
						arrayTableMust(t, err)
						if right != args[n] {
							t.Fatal("wrong left-associative operand order", n, right, args[n])
						}
						cursor, err = m.GetArg(cursor, 0)
						arrayTableMust(t, err)
					}
					if cursor != args[0] {
						t.Fatal("leftmost operand changed")
					}
					formula := own(m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{expression, bound}))
					before := m.Stats()
					arrayTableMust(t, implicant.setupModel(md))
					satisfied, err := implicant.checkModel([]ast.ASTHandle{formula})
					arrayTableMust(t, err)
					if want == "" {
						if satisfied || !implicant.isX(expression) || !implicant.isX(formula) {
							t.Fatal("inner zero divisor did not propagate unknown")
						}
					} else {
						expected, _ := new(big.Rat).SetString(want)
						got := implicant.numbers[expression]
						if !satisfied || got == nil || got.Cmp(expected) != 0 {
							t.Fatal("nested source division produced wrong value", satisfied, got, expected)
						}
					}
					arrayTableMust(t, implicant.reset())
					if want != "" {
						result, err := implicant.MinimizeLiterals([]ast.ASTHandle{formula}, md)
						arrayTableMust(t, err)
						if len(result) != 1 {
							t.Fatal("nested arithmetic literal was not collected", len(result))
						}
						h, err := m.RefGet(result[0])
						arrayTableMust(t, err)
						if h != formula {
							t.Fatal("collector changed nested arithmetic")
						}
						arrayTableMust(t, CloseImplicantResults(m, &result))
					}
					state, err := input.State()
					arrayTableMust(t, err)
					if state.Position != 0 {
						t.Fatal("valid constructed division reached a source assertion", state)
					}
					after := m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
						t.Fatal("nested division leaked references", after, before)
					}
				})
			}
		}
	}
}
