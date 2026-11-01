package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestModelImplicantCollectionSourceVerificationChain(t *testing.T) {
	for _, cone := range []bool{false, true} {
		for _, script := range []string{"T", "CT", "CCT", "CCC", "assertions-off"} {
			t.Run(fmt.Sprintf("cone=%t/%s", cone, script), func(t *testing.T) {
				m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
				actions := script
				if script == "assertions-off" {
					actions = "CC"
				}
				output, input := modelImplicantVerificationFixture(t, m, actions)
				if script == "assertions-off" {
					arrayTableMust(t, diagnostic.BorrowProcessDebugContext().EnableAssertions(false))
				}
				boolean, err := m.MkBoolSort()
				arrayTableMust(t, err)
				symbol, err := m.Symbols.Intern("collection-unknown")
				arrayTableMust(t, err)
				formula, err := m.MkConst(symbol, boolean)
				arrayTableMust(t, err)
				pin, err := m.NewRef(formula)
				arrayTableMust(t, err)
				defer func() { arrayTableMust(t, m.DelRef(&pin)) }()
				negated, err := m.MkNot(formula)
				arrayTableMust(t, err)
				negativePin, err := m.NewRef(negated)
				arrayTableMust(t, err)
				defer func() { arrayTableMust(t, m.DelRef(&negativePin)) }()
				// A known root is selected first, so late assertions/verification
				// must release a partially constructed nonempty result vector.
				yes, err := m.MkTrue()
				arrayTableMust(t, err)
				knownSymbol, err := m.Symbols.Intern("collection-known")
				arrayTableMust(t, err)
				known, err := m.MkConst(knownSymbol, boolean)
				arrayTableMust(t, err)
				knownPin, err := m.NewRef(known)
				arrayTableMust(t, err)
				defer func() { arrayTableMust(t, m.DelRef(&knownPin)) }()
				knownDecl, err := m.GetDecl(known)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(knownDecl, yes))
				equality, err := m.MkEq(known, yes)
				arrayTableMust(t, err)
				equalityPin, err := m.NewRef(equality)
				arrayTableMust(t, err)
				defer func() { arrayTableMust(t, m.DelRef(&equalityPin)) }()
				before := m.Stats()
				var result []ast.ASTRef
				if cone {
					result, err = implicant.MinimizeModel([]ast.ASTHandle{formula, known}, md)
				} else {
					result, err = implicant.MinimizeLiterals([]ast.ASTHandle{formula, known}, md)
				}
				continued := ast.ASTSourceDebug && (script == "CCC" || script == "assertions-off")
				if continued {
					arrayTableMust(t, err)
					expected := []ast.ASTHandle{known, negated}
					if cone {
						expected = []ast.ASTHandle{equality}
					}
					if len(result) != len(expected) {
						t.Fatal("wrong continued result size", len(result), len(expected))
					}
					for n, ref := range result {
						h, err := m.RefGet(ref)
						arrayTableMust(t, err)
						if h != expected[n] {
							t.Fatal("continued source result has wrong order or polarity", n, h, expected[n])
						}
					}
					arrayTableMust(t, CloseImplicantResults(m, &result))
				} else {
					if err == nil || len(result) != 0 {
						t.Fatal("source verification did not fail without publishing output", err, len(result))
					}
					if ast.ASTSourceDebug {
						var source z3exception.DefaultException
						if !errors.As(err, &source) {
							t.Fatal("source debugger throw was replaced", err)
						}
					} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
						t.Fatal("release verification lost native domain guard", err)
					}
				}
				text, err := os.ReadFile(output.Name())
				arrayTableMust(t, err)
				if !strings.HasPrefix(string(text), "Failed to verify: check_model(formulas)\n") {
					t.Fatal("missing collection VERIFY diagnostic", string(text))
				}
				state, err := input.State()
				arrayTableMust(t, err)
				wantPosition := uint64(0)
				if ast.ASTSourceDebug {
					wantPosition = uint64(len(actions))
					if script == "assertions-off" && !cone {
						wantPosition = 1
					}
					if !strings.Contains(string(text), "Line: 273\n") {
						t.Fatal("wrong collection verification location", string(text))
					}
					if script != "T" && script != "assertions-off" && !strings.Contains(string(text), "Line: 164\nis_true(e) || is_false(e)\n") {
						t.Fatal("missing process_formula state assertion", string(text))
					}
					if script == "CCT" || script == "CCC" {
						last := "Line: 136\nis_true(e) || is_false(e)\n"
						if cone {
							last = "Line: 113\nUNREACHABLE CODE WAS REACHED.\n"
						}
						if !strings.Contains(string(text), last) {
							t.Fatal("wrong final source assertion/verification", string(text))
						}
					}
					if script == "assertions-off" && strings.Contains(string(text), "is_true(e) || is_false(e)") {
						t.Fatal("disabled source assertion evaluated", string(text))
					}
				}
				if state.Position != wantPosition {
					t.Fatal("wrong source debugger command count", state.Position, wantPosition, string(text))
				}
				if implicant.model != nil || implicant.evaluator != nil || len(implicant.refs) != 0 || len(implicant.pendingArrays) != 0 || len(implicant.pendingEvaluators) != 0 {
					t.Fatal("verification chain left owner state")
				}
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("collection verification leaked ownership", after, before)
				}
				count, err := md.Core().NumConstants()
				arrayTableMust(t, err)
				if count != 1 {
					t.Fatal("verification completed an unknown constant", count)
				}
				unknownDecl, err := m.GetDecl(formula)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(unknownDecl, yes))
				if cone {
					result, err = implicant.MinimizeModel([]ast.ASTHandle{formula, known}, md)
				} else {
					result, err = implicant.MinimizeLiterals([]ast.ASTHandle{formula, known}, md)
				}
				arrayTableMust(t, err)
				if len(result) != 2 {
					t.Fatal("valid-model retry lost selected constants", len(result))
				}
				for n, root := range []ast.ASTHandle{known, formula} {
					h, err := m.RefGet(result[n])
					arrayTableMust(t, err)
					if cone {
						left, err := m.GetArg(h, 0)
						arrayTableMust(t, err)
						right, err := m.GetArg(h, 1)
						arrayTableMust(t, err)
						if left != root || !m.IsTrue(right) {
							t.Fatal("wrong retry model equality", n, left, right)
						}
					} else if h != root {
						t.Fatal("wrong retry literal order", n, h, root)
					}
				}
				arrayTableMust(t, CloseImplicantResults(m, &result))
				after = m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("valid-model retry leaked ownership", after, before)
				}
				retriedInput, err := input.State()
				arrayTableMust(t, err)
				if retriedInput.Position != wantPosition {
					t.Fatal("valid-model retry reached a source failure action", retriedInput)
				}

			})
		}
	}
}
