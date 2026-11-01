package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestModelImplicantArrayEqualityUsesSourceModelEvalPolicy(t *testing.T) {
	for _, name := range []string{"unknown-default", "unknown-row", "limited", "allocation"} {
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
			zero, one := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("1"))
			integer, err := m.GetSort(zero)
			arrayTableMust(t, err)
			sort := own(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), boolean))
			constant := func(name string) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				arrayTableMust(t, err)
				return own(m.MkConst(symbol, boolean))
			}
			u, v := constant("array-policy-u"), constant("array-policy-v")
			family, err := m.ArrayFamilyID()
			arrayTableMust(t, err)
			base := own(m.MkConstArray(sort, no))
			left, right := base, base
			expected := implicantX
			switch name {
			case "unknown-default":
				left = own(m.MkConstArray(sort, u))
				right = own(m.MkConstArray(sort, v))
			case "unknown-row":
				left = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, u}))
				right = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, v}))
			case "limited", "allocation":
				left = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, zero, own(m.MkNot(u))}))
				right = own(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, one, own(m.MkNot(v))}))
				if name == "limited" {
					expected = implicantTrue
				}
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
			}
			implicant.services.Params = &params
			arrayTableMust(t, implicant.setupModel(md))
			before := m.Stats()
			if name == "allocation" {
				m.Budget().FailAfter(0)
				err = implicant.evalArrayEq(equality, left, right)
				m.Budget().FailAfter(-1)
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("allocation failure was swallowed as a model exception", err)
				}
				if implicant.state(equality) != implicantUnknown {
					t.Fatal("failed construction published an equality state")
				}
			} else {
				tags.Enable("pdr")
				tags.Enable("model_evaluator")
			}
			attempts := 1
			if name == "limited" {
				attempts = 2
			}
			for n := 0; n < attempts; n++ {
				delete(implicant.states, equality)
				arrayTableMust(t, implicant.evalArrayEq(equality, left, right))
				// Source ignores model::eval's Boolean return here: two caught failures
				// leave two null expr_refs, whose identity makes this comparison true.
				if got := implicant.state(equality); got != expected {
					t.Fatal("wrong source array evaluation result", got, expected)
				}
				after := m.Stats()
				if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes {
					t.Fatal("array model evaluation leaked ownership", after, before)
				}
			}
			count, err := md.Core().NumConstants()
			arrayTableMust(t, err)
			if count != 0 {
				t.Fatal("array equality completed missing Boolean interpretations", count)
			}
			if name == "limited" && tags.ShouldTrace("model_evaluator") {
				caught := "-------- [model_evaluator] eval src/model/model.cpp:88 ---------\n" + messages.MaxSteps + "\n------------------------------------------------\n"
				if got := strings.Count(output.String(), caught); got != 4 {
					t.Fatal("operand failures were not caught independently", got, output.String())
				}
			}
		})
	}
}
