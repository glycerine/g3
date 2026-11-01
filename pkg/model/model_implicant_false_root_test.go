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

func TestModelImplicantFalseRootUsesSourceUnreachable(t *testing.T) {
	for _, choice := range []string{"T", "C"} {
		for _, assertions := range []bool{true, false} {
			for _, laterUnknown := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/assertions=%t/later=%t", choice, assertions, laterUnknown), func(t *testing.T) {
					m, md, implicant, trace, tags := modelImplicantDiagnosticFixture(t)
					output, input := modelImplicantVerificationFixture(t, m, choice)
					arrayTableMust(t, diagnostic.BorrowProcessDebugContext().EnableAssertions(assertions))
					yes, err := m.MkTrue()
					arrayTableMust(t, err)
					root, err := m.MkNot(yes)
					arrayTableMust(t, err)
					rootPin, err := m.NewRef(root)
					arrayTableMust(t, err)
					defer func() { arrayTableMust(t, m.DelRef(&rootPin)) }()
					roots := []ast.ASTHandle{root}
					if laterUnknown {
						boolean, err := m.MkBoolSort()
						arrayTableMust(t, err)
						symbol, err := m.Symbols.Intern("false-root-later")
						arrayTableMust(t, err)
						unknown, err := m.MkConst(symbol, boolean)
						arrayTableMust(t, err)
						pin, err := m.NewRef(unknown)
						arrayTableMust(t, err)
						defer func() { arrayTableMust(t, m.DelRef(&pin)) }()
						roots = append(roots, unknown)
					}
					tags.Enable("pdr_verbose")
					tags.Enable("pdr")
					before := m.Stats()
					arrayTableMust(t, implicant.setupModel(md))
					satisfied, err := implicant.checkModel(roots)
					continued := ast.ASTSourceDebug && choice == "C"
					if continued {
						arrayTableMust(t, err)
						if satisfied == laterUnknown {
							t.Fatal("source continuation lost the later-root decision", satisfied, laterUnknown)
						}
					} else {
						if err == nil || satisfied {
							t.Fatal("false formula did not reach source failure", satisfied, err)
						}
						if ast.ASTSourceDebug {
							var source z3exception.DefaultException
							if !errors.As(err, &source) {
								t.Fatal("source throw action was replaced", err)
							}
						} else if !errors.Is(err, diagnostic.ErrDebugNativeUnreachable) {
							t.Fatal("release source guard was replaced", err)
						}
					}
					if !implicant.isFalse(root) {
						t.Fatal("failure action changed root truth")
					}
					bytes, err := os.ReadFile(output.Name())
					arrayTableMust(t, err)
					position := uint64(0)
					if ast.ASTSourceDebug {
						position = 1
						expected := "ASSERTION VIOLATION\nFile: src/model/model_implicant.cpp\nLine: 864\nUNREACHABLE CODE WAS REACHED.\n" + "(C)ontinue, (A)bort, (S)top, (T)hrow exception, Invoke (G)DB\n"
						if string(bytes) != expected {
							t.Fatalf("wrong source UNREACHABLE diagnostic\ngot %q\nwant %q", string(bytes), expected)
						}
					} else if len(bytes) != 0 {
						t.Fatal("release unreachable read debug bindings", string(bytes))
					}
					state, err := input.State()
					arrayTableMust(t, err)
					if state.Position != position {
						t.Fatal("wrong source action input count", state)
					}
					if tags.ShouldTrace("pdr_verbose") {
						falseRecord := "[pdr_verbose] check_model src/model/model_implicant.cpp:859 ---------\nformula is false\n(not true)\n"
						if strings.Count(trace.String(), falseRecord) != 1 {
							t.Fatal("missing false-root trace", trace.String())
						}
						expectedLater := 0
						if continued && laterUnknown {
							expectedLater = 1
						}
						if strings.Count(trace.String(), "formula is unknown\nfalse-root-later\n") != expectedLater {
							t.Fatal("later formula trace crossed source failure action", trace.String())
						}
					}
					arrayTableMust(t, implicant.reset())
					after := m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
						t.Fatal("false-root source action leaked ownership", after, before)
					}
					arrayTableMust(t, implicant.setupModel(md))
					satisfied, err = implicant.checkModel([]ast.ASTHandle{yes})
					arrayTableMust(t, err)
					if !satisfied {
						t.Fatal("valid retry did not recover")
					}
					arrayTableMust(t, implicant.reset())
					state, err = input.State()
					arrayTableMust(t, err)
					if state.Position != position {
						t.Fatal("valid retry invoked another failure action")
					}
					after = m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
						t.Fatal("false-root retry leaked ownership", after, before)
					}
				})
			}
		}
	}
}
