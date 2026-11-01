package model

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestModelImplicantAssignmentClassificationAndSourceDiagnostics(t *testing.T) {
	for _, name := range []string{"true", "false", "numeral", "model-value", "untracked"} {
		for _, level := range []uint32{2, 3} {
			t.Run(fmt.Sprintf("%s/verbosity=%d", name, level), func(t *testing.T) {
				m, md, implicant, output, tags := modelImplicantDiagnosticFixture(t)
				var pins []ast.ASTRef
				defer func() {
					for j := range pins {
						arrayTableMust(t, m.DelRef(&pins[j]))
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
				value := own(m.MkTrue())
				expected := implicantTrue
				switch name {
				case "false":
					value = own(m.MkFalse())
					expected = implicantFalse
				case "numeral":
					value = own(m.ArithmeticIntegerNumeralText("7"))
					expected = implicantConcrete
				case "model-value":
					symbol, err := m.Symbols.Intern("AssignmentSort")
					arrayTableMust(t, err)
					sort := own(m.MkUninterpretedSort(symbol, nil))
					value = own(m.MkModelValue(1, sort))
					expected = implicantConcrete
				case "untracked":
					symbol, err := m.Symbols.Intern("assignment-value")
					arrayTableMust(t, err)
					value = own(m.MkConst(symbol, boolean))
					expected = implicantX
				}
				sort, err := m.GetSort(value)
				arrayTableMust(t, err)
				symbol, err := m.Symbols.Intern("assignment-variable")
				arrayTableMust(t, err)
				variable := own(m.MkConst(symbol, sort))
				decl, err := m.GetDecl(variable)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, value))
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
				before := m.Stats().LiveReferences
				arrayTableMust(t, implicant.setupModel(md))
				if got := implicant.state(variable); got != expected {
					t.Fatal("wrong model value classification", got, expected)
				}
				if name == "numeral" && (implicant.numbers[variable] == nil || implicant.numbers[variable].RatString() != "7") {
					t.Fatal("numeral assignment was not tracked exactly")
				}
				if name == "model-value" && implicant.values[variable] != value {
					t.Fatal("concrete model value was not tracked")
				}
				verbose, err := stream.String()
				arrayTableMust(t, err)
				expectedVerbose, expectedTrace := "", ""
				if name == "untracked" {
					if level >= 3 {
						expectedVerbose = "Not evaluated assignment-variable := assignment-value\n"
					}
					if tags.ShouldTrace("pdr") {
						expectedTrace = "-------- [pdr] assign_value src/model/model_implicant.cpp:66 ---------\nVariable is not tracked: assignment-variable := assignment-value\n------------------------------------------------\n"
					}
				}
				if verbose != expectedVerbose || output.String() != expectedTrace {
					t.Fatalf("wrong assignment diagnostics\nverbose %q want %q\ntrace %q want %q", verbose, expectedVerbose, output.String(), expectedTrace)
				}
				arrayTableMust(t, implicant.reset())
				if got := m.Stats().LiveReferences; got != before {
					t.Fatal("assignment/printing leaked reference slots", got, before)
				}
			})
		}
	}
}
