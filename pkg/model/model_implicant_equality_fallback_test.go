package model

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestModelImplicantScalarEqualityFallbackUsesSourceModelEvalPolicy(t *testing.T) {
	for _, name := range []string{"true", "false", "unknown", "mixed", "limited"} {
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
			constant := func(name string) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				arrayTableMust(t, err)
				return own(m.MkConst(symbol, boolean))
			}
			left, right, u, v := constant("fallback-left"), constant("fallback-right"), constant("fallback-u"), constant("fallback-v")
			leftValue, rightValue := u, v
			expected, unknown := implicantX, "(= fallback-u fallback-v)"
			switch name {
			case "true":
				rightValue = u
				expected, unknown = implicantTrue, ""
			case "false":
				leftValue = own(m.MkNot(u))
				rightValue = u
				expected, unknown = implicantFalse, ""
			case "mixed":
				leftValue = own(m.MkTrue())
				unknown = "fallback-v"
			case "limited":
				leftValue = own(m.MkNot(u))
				rightValue = u
				unknown = "null"
			}
			for _, entry := range []struct{ variable, value ast.ASTHandle }{{left, leftValue}, {right, rightValue}} {
				decl, err := m.GetDecl(entry.variable)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, entry.value))
			}
			equality := own(m.MkEq(left, right))
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
				tags.Enable("model_evaluator")
			}
			implicant.services.Params = &params
			tags.Enable("pdr")
			arrayTableMust(t, implicant.setupModel(md))
			output.Reset()
			before := m.Stats()
			attempts := 1
			if name == "limited" {
				attempts = 2
			}
			for n := 0; n < attempts; n++ {
				delete(implicant.states, equality)
				arrayTableMust(t, implicant.evalEq(equality, left, right))
				if got := implicant.state(equality); got != expected {
					t.Fatal("wrong fallback state", got, expected)
				}
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
					t.Fatal("fallback leaked temporary equality/evaluator ownership", after, before)
				}
			}
			count, err := md.Core().NumConstants()
			arrayTableMust(t, err)
			if count != 2 {
				t.Fatal("fallback completed unknown model constants", count)
			}
			if unknown == "" && output.Len() != 0 {
				t.Fatal("decided equality emitted an unknown diagnostic", output.String())
			}
			if tags.ShouldTrace("pdr") && unknown != "" {
				message := "-------- [pdr] eval_eq src/model/model_implicant.cpp:635 ---------\ncannot evaluate: " + unknown + "\n------------------------------------------------\n"
				if got := strings.Count(output.String(), message); got != attempts {
					t.Fatal("missing source unknown diagnostic", got, attempts, output.String())
				}
			}
			if name == "limited" && tags.ShouldTrace("model_evaluator") {
				if got := strings.Count(output.String(), "-------- [model_evaluator] eval src/model/model.cpp:88 ---------\n"+messages.MaxSteps+"\n------------------------------------------------\n"); got != 2 {
					t.Fatal("source evaluator exception was not caught independently per call", got, output.String())
				}
			}
		})
	}
}
