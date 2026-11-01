package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestModelImplicantCheckModelSourceDiagnostics(t *testing.T) {
	for _, state := range []string{"true", "false", "unknown", "unknown-stream-failure"} {
		for _, level := range []uint32{34, 35} {
			t.Run(fmt.Sprintf("%s/%d", state, level), func(t *testing.T) {
				m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
				truth := state
				if state == "unknown-stream-failure" {
					truth = "unknown"
					tags = diagnostic.NewTraceTagsWithWriter(modelImplicantRefusingTraceWriter{})
					implicant.services.Trace = tags
				}
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
				boolean := own(m.MkBoolSort())
				yes, no := own(m.MkTrue()), own(m.MkFalse())
				sym, err := m.Symbols.Intern("check-witness")
				arrayTableMust(t, err)
				witness := own(m.MkConst(sym, boolean))
				decl, err := m.GetDecl(witness)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, yes))
				var formula ast.ASTHandle
				text, assign := "(not false)", "assigned falsefalse\nassigned (not false)true\n"
				switch truth {
				case "true":
					formula = own(m.MkNot(no))
				case "false":
					formula = own(m.MkNot(yes))
					text = "(not true)"
					assign = "assigned truetrue\nassigned (not true)false\n"
				case "unknown":
					sym, err = m.Symbols.Intern("check-unknown")
					arrayTableMust(t, err)
					formula = own(m.MkConst(sym, boolean))
					text = "check-unknown"
					assign = "assigned check-unknownunknown\n"
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
				arrayTableMust(t, diagnostic.SetProcessVerbosityLevel(level))
				arrayTableMust(t, diagnostic.SetProcessVerboseStream(stream))
				tags.Enable("pdr")
				tags.Enable("pdr_verbose")
				before := m.Stats()
				arrayTableMust(t, implicant.setupModel(md))
				satisfied, err := implicant.checkModel([]ast.ASTHandle{formula})
				if truth == "false" {
					if !errors.Is(err, ast.ErrASTDomain) {
						t.Fatal("false formula lost source failure", err)
					}
				} else {
					arrayTableMust(t, err)
				}
				if satisfied != (truth == "true") {
					t.Fatal("incorrect check_model result", satisfied, truth)
				}
				expected := ""
				if truth == "unknown" {
					expected = "Not evaluated check-unknown := check-unknown\n"
				}
				if level == 35 {
					expected += assign
				}
				if truth == "false" {
					expected += "formula false in model: " + text + "\n"
				}
				if truth == "unknown" {
					expected += "formula undetermined in model: " + text + "\n"
				}
				verbose, err := stream.String()
				arrayTableMust(t, err)
				if verbose != expected {
					t.Fatalf("verbose mismatch\ngot %q\nwant %q", verbose, expected)
				}
				if tags.ShouldTrace("pdr_verbose") && state != "unknown-stream-failure" {
					record := "[pdr_verbose] check_model src/model/model_implicant.cpp:859 ---------\nformula is " + state + "\n" + text + "\n"
					if strings.Count(output.String(), record) != 1 {
						t.Fatal("missing root state trace", output.String())
					}
				}
				if tags.ShouldTrace("pdr") && state != "unknown-stream-failure" {
					dump := strings.Index(output.String(), "[pdr] check_model src/model/model_implicant.cpp:868")
					if truth == "unknown" {
						if dump < 0 || !strings.Contains(output.String()[dump:], "check-witness") {
							t.Fatal("missing attached model dump", output.String())
						}
					} else if dump >= 0 {
						t.Fatal("known formula dumped its model", output.String())
					}
				}
				count, err := md.Core().NumConstants()
				arrayTableMust(t, err)
				if count != 1 {
					t.Fatal("diagnostics completed an unknown constant", count)
				}
				arrayTableMust(t, implicant.reset())
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("check_model diagnostics leaked ownership", after, before)
				}
			})
		}
	}
}

type modelImplicantRefusingTraceWriter struct{}

func (modelImplicantRefusingTraceWriter) Write([]byte) (int, error) {
	return 0, errors.New("test trace stream refusal")
}
