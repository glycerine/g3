package model

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestModelImplicantConcreteValueEqualityAndITEInheritance(t *testing.T) {
	for _, name := range []string{"equal", "different", "condition-true", "condition-false", "unknown-same-branch", "unknown-equal-values", "unknown-different-values"} {
		t.Run(name, func(t *testing.T) {
			m, md, implicant, _, _ := modelImplicantDiagnosticFixture(t)
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
			symbol, err := m.Symbols.Intern("ConcreteValueSort")
			arrayTableMust(t, err)
			sort := own(m.MkUninterpretedSort(symbol, nil))
			boolean := own(m.MkBoolSort())
			first, second := own(m.MkModelValue(0, sort)), own(m.MkModelValue(1, sort))
			constant := func(name string, sort ast.ASTHandle) ast.ASTHandle {
				symbol, err := m.Symbols.Intern(name)
				arrayTableMust(t, err)
				return own(m.MkConst(symbol, sort))
			}
			a, b, condition := constant("concrete-a", sort), constant("concrete-b", sort), constant("concrete-condition", boolean)
			same := name == "equal" || name == "unknown-equal-values"
			secondValue := second
			if same {
				secondValue = first
			}
			for _, entry := range []struct{ term, value ast.ASTHandle }{{a, first}, {b, secondValue}} {
				decl, err := m.GetDecl(entry.term)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, entry.value))
			}
			if name == "condition-true" || name == "condition-false" {
				value := own(m.MkTrue())
				if name == "condition-false" {
					value = own(m.MkFalse())
				}
				decl, err := m.GetDecl(condition)
				arrayTableMust(t, err)
				arrayTableMust(t, md.Core().RegisterConstant(decl, value))
			}
			expression := a
			root := own(m.MkEq(a, b))
			rootState, expressionState, expectedValue := implicantTrue, implicantConcrete, first
			switch name {
			case "equal":
			case "different":
				rootState = implicantFalse
			default:
				alternative := b
				if name == "unknown-same-branch" {
					alternative = a
				}
				expression = own(m.MkIte(condition, a, alternative))
				target := first
				if name == "condition-false" {
					target = second
					expectedValue = second
				}
				root = own(m.MkEq(expression, target))
				if name == "unknown-equal-values" {
					expressionState = implicantX
					expectedValue = ast.ASTHandle{}
				}
				if name == "unknown-different-values" {
					expressionState, rootState = implicantX, implicantX
					expectedValue = ast.ASTHandle{}
				}
			}
			// check_model consumes model-true formulas; preserve the false
			// equality as a child of its true negation.
			formula := root
			if rootState == implicantFalse {
				formula = own(m.MkNot(root))
			}
			modelCount, err := md.Core().NumConstants()
			arrayTableMust(t, err)
			before := m.Stats()
			arrayTableMust(t, implicant.setupModel(md))
			satisfied, err := implicant.checkModel([]ast.ASTHandle{formula})
			arrayTableMust(t, err)
			if satisfied != (rootState != implicantX) || implicant.state(root) != rootState {
				t.Fatal("wrong concrete equality decision", satisfied, implicant.state(root), rootState)
			}
			if implicant.state(expression) != expressionState {
				t.Fatal("wrong source ITE state", implicant.state(expression), expressionState)
			}
			if got := implicant.values[expression]; got != expectedValue {
				t.Fatal("wrong inherited concrete value", got, expectedValue)
			}
			// All source set_value writers either check is_value or inherit an
			// existing tracked value; exercise that invariant through live inputs.
			for term, value := range implicant.values {
				valid, err := m.IsValue(value)
				arrayTableMust(t, err)
				if !valid {
					t.Fatal("non-value entered tracked-value map", term, value)
				}
			}
			remaining, err := md.Core().NumConstants()
			arrayTableMust(t, err)
			if remaining != modelCount {
				t.Fatal("equality fallback completed an unknown condition")
			}
			arrayTableMust(t, implicant.reset())
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
				t.Fatal("concrete equality/ITE leaked ownership", after, before)
			}
		})
	}
}
