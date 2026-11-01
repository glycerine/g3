package model

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestModelImplicantMinimizationSourceTrace(t *testing.T) {
	for _, cone := range []bool{false, true} {
		for _, shape := range []string{"empty", "positive", "negative"} {
			for _, mode := range []string{"disabled", "other-tag", "enabled"} {
				t.Run(fmt.Sprintf("cone=%v/%s/%s", cone, shape, mode), func(t *testing.T) {
					m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
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
					boolean := own(m.MkBoolSort())
					yes, no := own(m.MkTrue()), own(m.MkFalse())
					symbol, err := m.Symbols.Intern("minimize-trace-p")
					arrayTableMust(t, err)
					decl := own(m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{}))
					variable := own(m.MkApp(decl, nil))
					value := yes
					formula, formulaText := variable, "minimize-trace-p"
					if shape == "negative" {
						value = no
						formula = own(m.MkNot(variable))
						formulaText = "(not minimize-trace-p)"
					}
					arrayTableMust(t, md.Core().RegisterConstant(decl, value))
					var formulas []ast.ASTHandle
					if shape != "empty" {
						formulas = []ast.ASTHandle{formula, formula}
					}
					tag, other, function, prefix := "pdr", "pdr_verbose", "minimize_literals", "minimized model:\n"
					first, last := 121, 154
					if cone {
						tag, other, function, prefix = other, tag, "minimize_model", "pruned model:\n"
						first, last = 99, 105
					}
					if mode == "enabled" {
						tags.Enable(tag)
					} else if mode == "other-tag" {
						tags.Enable(other)
					}
					before := m.Stats()
					var result []ast.ASTRef
					if cone {
						result, err = implicant.MinimizeModel(formulas, md)
					} else {
						result, err = implicant.MinimizeLiterals(formulas, md)
					}
					arrayTableMust(t, err)
					if implicant.model != nil || implicant.evaluator != nil || len(implicant.refs) != 0 {
						t.Fatal("minimization did not reset its temporary state")
					}
					count := 1
					if shape == "empty" {
						count = 0
					}
					if len(result) != count {
						t.Fatal("duplicate or missing selected result", len(result), count)
					}
					if tags.ShouldTrace(tag) {
						header := func(line int, body string) string {
							return fmt.Sprintf("-------- [%s] %s src/model/model_implicant.cpp:%d ---------\n%s------------------------------------------------\n", tag, function, line, body)
						}
						input, body := "formulas:\n", prefix
						if shape != "empty" {
							input += formulaText + "\n" + formulaText + "\n"
							resultText := formulaText
							if cone {
								scalar := "true"
								if shape == "negative" {
									scalar = "false"
								}
								resultText = "(= minimize-trace-p " + scalar + ")"
							}
							body += resultText + "\n"
						}
						expected := header(first, input) + header(last, body)
						if output.String() != expected {
							t.Fatalf("trace mismatch\ngot %q\nwant %q", output.String(), expected)
						}
					} else if output.Len() != 0 {
						t.Fatal("disabled minimization trace produced output", output.String())
					}
					arrayTableMust(t, CloseImplicantResults(m, &result))
					after := m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
						t.Fatal("minimization diagnostics leaked ownership", after, before)
					}
				})
			}
		}
	}
}
