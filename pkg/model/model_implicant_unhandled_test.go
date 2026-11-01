package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestModelImplicantUnhandledSourceTermsCleanUpAndRetry(t *testing.T) {
	for _, cone := range []bool{false, true} {
		for _, kind := range []string{"arithmetic", "basic", "implication-witness", "basic-collection"} {
			scripts := []string{"T", "CT", "CCT"}
			if kind == "implication-witness" {
				scripts = []string{"T", "C"}
			}
			if kind == "basic-collection" {
				scripts = []string{"CCCCT"}
			}
			for _, script := range scripts {
				t.Run(fmt.Sprintf("cone=%t/%s/%s", cone, kind, script), func(t *testing.T) {
					m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
					output, input := modelImplicantVerificationFixture(t, m, script)
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
					yes, no := own(m.MkTrue()), own(m.MkFalse())
					var formula ast.ASTHandle
					text := ""
					line := 0
					switch kind {
					case "arithmetic":
						seven := own(m.ArithmeticIntegerNumeralText("7"))
						family, err := m.ArithmeticFamilyID()
						arrayTableMust(t, err)
						absolute := own(m.MkFamilyApp(family, 17, nil, []ast.ASTHandle{seven}))
						formula = own(m.MkEq(absolute, seven))
						text = "(abs 7)"
						line = 432
					case "basic":
						formula = own(m.MkXor(no, yes))
						text = "(xor false true)"
						line = 808
					case "basic-collection":
						formula = own(m.MkInterp(yes))
						text = "(interp true)"
						line = 808
					case "implication-witness":
						boolean := own(m.MkBoolSort())
						symbol, err := m.Symbols.Intern("unsupported-p")
						arrayTableMust(t, err)
						p := own(m.MkConst(symbol, boolean))
						formula = own(m.MkImplies(p, p))
						text = "(=> unsupported-p unsupported-p)"
						line = 251
					}
					oldLevel, err := diagnostic.ProcessVerbosityLevel()
					arrayTableMust(t, err)
					oldStream, err := diagnostic.BorrowProcessVerboseStream()
					arrayTableMust(t, err)
					stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
					arrayTableMust(t, err)
					defer func() {
						arrayTableMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
						arrayTableMust(t, diagnostic.SetProcessVerboseStream(oldStream))
						arrayTableMust(t, stream.Close())
					}()
					arrayTableMust(t, diagnostic.SetProcessVerbosityLevel(0))
					arrayTableMust(t, diagnostic.SetProcessVerboseStream(stream))
					minimize := func(root ast.ASTHandle) ([]ast.ASTRef, error) {
						if cone {
							return implicant.MinimizeModel([]ast.ASTHandle{root}, md)
						}
						return implicant.MinimizeLiterals([]ast.ASTHandle{root}, md)
					}
					before := m.Stats()
					result, err := minimize(formula)
					continued := ast.ASTSourceDebug && kind == "implication-witness" && script == "C"
					if continued {
						arrayTableMust(t, err)
					} else {
						if err == nil {
							t.Fatal("unsupported source term did not fail")
						}
						if ast.ASTSourceDebug {
							var source z3exception.DefaultException
							if !errors.As(err, &source) {
								t.Fatal("source throw was replaced", err)
							}
						} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
							t.Fatal("source release guard was replaced", err)
						}
					}
					if len(result) != 0 {
						t.Fatal("unsupported source path published literals", len(result))
					}
					arrayTableMust(t, CloseImplicantResults(m, &result))
					messages := 1
					if ast.ASTSourceDebug && kind == "arithmetic" && script == "CCT" {
						messages = 2
					}
					if ast.ASTSourceDebug && kind == "basic-collection" {
						messages = 2
					}

					message, err := stream.String()
					arrayTableMust(t, err)
					if expected := strings.Repeat("Term not handled "+text+"\n", messages); message != expected {
						t.Fatalf("wrong source failure diagnostics\ngot %q\nwant %q", message, expected)
					}
					bytes, err := os.ReadFile(output.Name())
					arrayTableMust(t, err)
					state, err := input.State()
					arrayTableMust(t, err)
					position := uint64(0)
					if ast.ASTSourceDebug {
						position = uint64(len(script))
						source := fmt.Sprintf("Line: %d\nUNREACHABLE CODE WAS REACHED.\n", line)
						primaryActions := messages
						if kind == "basic-collection" {
							primaryActions = 1
						}
						if strings.Count(string(bytes), source) != primaryActions {
							t.Fatal("wrong unsupported source action", string(bytes), source)
						}
						if script != "T" && kind != "implication-witness" && !strings.Contains(string(bytes), "Line: 852\n!is_unknown(curr)\n") {
							t.Fatal("missing post-evaluation assertion", string(bytes))
						}
						if (script == "CCT" && kind == "basic" || kind == "basic-collection") && !strings.Contains(string(bytes), "Line: 858\n!is_unknown(form)\n") {
							t.Fatal("missing root evaluation assertion", string(bytes))
						}
						if kind == "basic-collection" && (!strings.Contains(string(bytes), "Line: 164\nis_true(e) || is_false(e)\n") || !strings.Contains(string(bytes), "Line: 260\nUNREACHABLE CODE WAS REACHED.\n")) {
							t.Fatal("missing unsupported collection action", string(bytes))
						}
						if strings.Contains(string(bytes), "Failed to verify:") {
							t.Fatal("exception was converted into false VERIFY", string(bytes))
						}
					} else if len(bytes) != 0 {
						t.Fatal("release failure read debug action bindings", string(bytes))
					}
					if state.Position != position {
						t.Fatal("wrong source action count", state.Position, position)
					}
					check := func() {
						t.Helper()
						after := m.Stats()
						if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
							t.Fatal("failed collection leaked ownership", after, before)
						}
						if implicant.model != nil || implicant.evaluator != nil || len(implicant.refs) != 0 || len(implicant.states) != 0 || len(implicant.pendingArrays) != 0 || len(implicant.pendingEvaluators) != 0 {
							t.Fatal("failed collection left owner state")
						}
					}
					check()
					result, err = minimize(yes)
					arrayTableMust(t, err)
					if len(result) != 0 {
						t.Fatal("retry collected a true constant", len(result))
					}
					arrayTableMust(t, CloseImplicantResults(m, &result))
					check()
					count, err := md.Core().NumConstants()
					arrayTableMust(t, err)
					if count != 0 {
						t.Fatal("failure path completed a partial model", count)
					}
					retriedInput, err := input.State()
					arrayTableMust(t, err)
					if retriedInput.Position != position {
						t.Fatal("valid retry invoked failure actions")
					}
				})
			}
		}
	}
}
