package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestModelImplicantTheoryExpressionsUseSourceModelEvalPolicy(t *testing.T) {
	for _, name := range []string{"defined", "missing", "limited", "allocation"} {
		t.Run(name, func(t *testing.T) {
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
			no := own(m.MkFalse())
			symbol, err := m.Symbols.Intern("theory-policy-f")
			arrayTableMust(t, err)
			function := own(m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
			if name != "missing" {
				bound := own(m.MkVar(0, boolean))
				body := own(m.MkNot(bound))
				graph, err := NewFunctionInterpretation(m, 1)
				arrayTableMust(t, err)
				arrayTableMust(t, graph.SetElse(body))
				arrayTableMust(t, md.Core().RegisterFunction(function, graph))
			}
			expression := own(m.MkApp(function, []ast.ASTHandle{no}))
			params, err := implicant.services.Context.NewValue()
			arrayTableMust(t, err)
			defer func() {
				arrayTableMust(t, implicant.reset())
				implicant.services.Params = nil
				arrayTableMust(t, implicant.services.Context.Del(&params))
			}()
			completion, steps := "completion", "max_steps"
			arrayTableMust(t, implicant.services.Context.SetBoolName(&params, &completion, true))
			if name == "limited" {
				arrayTableMust(t, implicant.services.Context.SetUIntName(&params, &steps, 0))
			}
			implicant.services.Params = &params
			arrayTableMust(t, implicant.setupModel(md))
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
			arrayTableMust(t, diagnostic.SetProcessVerbosityLevel(3))
			arrayTableMust(t, diagnostic.SetProcessVerboseStream(stream))
			before := m.Stats()
			if name == "allocation" {
				m.Budget().FailAfter(0)
				_, err = implicant.checkModel([]ast.ASTHandle{expression})
				m.Budget().FailAfter(-1)
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("construction failure was swallowed", err)
				}
				if implicant.state(expression) != implicantUnknown {
					t.Fatal("construction failure published a state")
				}
			}
			tags.Enable("pdr")
			if name == "limited" {
				tags.Enable("model_evaluator")
			}
			attempts := 1
			if name == "limited" {
				attempts = 2
			}
			expected := implicantTrue
			if name == "missing" || name == "limited" {
				expected = implicantX
			}
			for n := 0; n < attempts; n++ {
				delete(implicant.states, expression)
				satisfied, err := implicant.checkModel([]ast.ASTHandle{expression})
				arrayTableMust(t, err)
				if satisfied != (expected == implicantTrue) || implicant.state(expression) != expected {
					t.Fatal("wrong theory-expression evaluation", satisfied, implicant.state(expression), expected)
				}
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("theory model evaluation leaked ownership", after, before)
				}
			}
			count, err := md.Core().NumDeclarations()
			arrayTableMust(t, err)
			expectedFunctions := uint32(1)
			if name == "missing" {
				expectedFunctions = 0
			}
			if count != expectedFunctions {
				t.Fatal("theory evaluation completed a missing function", count, expectedFunctions)
			}
			if name == "limited" {
				verbose, err := stream.String()
				arrayTableMust(t, err)
				if got := strings.Count(verbose, "Not evaluated (theory-policy-f false) := null\n"); got != 2 {
					t.Fatal("missing null assignment verbosity", verbose)
				}
				if tags.ShouldTrace("pdr") && strings.Count(output.String(), "Variable is not tracked: (theory-policy-f false) := null\n") != 2 {
					t.Fatal("missing null assignment trace", output.String())
				}
				if tags.ShouldTrace("model_evaluator") && strings.Count(output.String(), "[model_evaluator] eval src/model/model.cpp:88 ---------\n"+messages.MaxSteps+"\n") != 2 {
					t.Fatal("typed failures were not independently caught", output.String())
				}
			}
		})
	}
}
