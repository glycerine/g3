package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestModelImplicantUnhandledSourceTermsCleanUpAndRetry(t *testing.T) {
	for _, cone := range []bool{false, true} {
		for _, kind := range []string{"arithmetic", "basic", "implication-witness"} {
			t.Run(fmt.Sprintf("cone=%t/%s", cone, kind), func(t *testing.T) {
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
				yes, no := own(m.MkTrue()), own(m.MkFalse())
				var formula ast.ASTHandle
				text := ""
				switch kind {
				case "arithmetic":
					seven := own(m.ArithmeticIntegerNumeralText("7"))
					family, err := m.ArithmeticFamilyID()
					arrayTableMust(t, err)
					absolute := own(m.MkFamilyApp(family, 17, nil, []ast.ASTHandle{seven}))
					formula = own(m.MkEq(absolute, seven))
					text = "(abs 7)"
				case "basic":
					formula = own(m.MkXor(no, yes))
					text = "(xor false true)"
				case "implication-witness":
					boolean := own(m.MkBoolSort())
					symbol, err := m.Symbols.Intern("unsupported-p")
					arrayTableMust(t, err)
					p := own(m.MkConst(symbol, boolean))
					formula = own(m.MkImplies(p, p))
					text = "(=> unsupported-p unsupported-p)"
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
				if !errors.Is(err, ast.ErrASTDomain) || len(result) != 0 {
					t.Fatal("unhandled source term did not fail cleanly", err, len(result))
				}
				message, err := stream.String()
				arrayTableMust(t, err)
				if expected := "Term not handled " + text + "\n"; message != expected {
					t.Fatalf("wrong source failure diagnostic\ngot %q\nwant %q", message, expected)
				}
				check := func() {
					t.Helper()
					after := m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
						t.Fatal("failed collection leaked ownership", after, before)
					}
					if implicant.model != nil || implicant.evaluator != nil || len(implicant.refs) != 0 || len(implicant.states) != 0 {
						t.Fatal("failed collection left model state")
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
					t.Fatal("failed collection completed the partial model", count)
				}
			})
		}
	}
}
