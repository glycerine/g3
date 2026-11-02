package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyProjectionOuterTransfersPreserveRefusedChildren(t *testing.T) {
	cases := []struct {
		name, kind string
		success    bool
	}{
		{name: "array/operation-error", kind: "array"},
		{name: "array/operation-success", kind: "array", success: true},
		{name: "arithmetic/operation-error", kind: "arithmetic"},
		{name: "arithmetic/operation-success", kind: "arithmetic", success: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			active, callbacks := false, 0
			var transfer error
			var closeChild func(error) error
			f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
				if active {
					callbacks++
					cause := error(failure)
					if test.success {
						cause = nil
					}
					transfer = closeChild(cause)
				}
				return failure
			})
			m := f.r.manager
			formula := f.constant("outer-transfer-formula-"+test.name, f.boolean)
			variable := f.constant("outer-transfer-variable-"+test.name, f.integer)
			formulaRef, err := m.NewRef(formula)
			fmMust(t, err)
			variableRef, err := m.NewRef(variable)
			fmMust(t, err)
			var childOpen func() bool
			if test.kind == "array" {
				child := &legacyArrayProjectionResult{manager: m, formula: formulaRef, variables: []ast.ASTRef{variableRef}}
				closeChild = func(cause error) error { return closeTransferredLegacyOuterArray(child, cause) }
				childOpen = func() bool {
					return child.manager == m && child.formula == formulaRef && len(child.variables) == 1 && child.variables[0] == variableRef
				}
			} else {
				child := &legacyFormulaProjection{manager: m, formula: formulaRef, variables: []ast.ASTRef{variableRef}}
				closeChild = func(cause error) error { return closeTransferredLegacyOuterArithmetic(child, cause) }
				childOpen = func() bool {
					return child.manager == m && child.formula == formulaRef && len(child.variables) == 1 && child.variables[0] == variableRef
				}
			}
			// These are the independent owners created by LegacyProjection.project
			// before it retires the round result.
			current, err := m.CopyRef(formulaRef)
			fmMust(t, err)
			ownedVariable, err := m.CopyRef(variableRef)
			fmMust(t, err)

			value, err := m.NumeralService().NewValue()
			fmMust(t, err)
			defer func() { fmMust(t, m.NumeralService().Del(&value)) }()
			fmMust(t, m.NumeralService().SetString(&value, "18446744073709551617/18446744073709551619"))
			fmMust(t, f.runtime.SetMaxCount(uint64(f.runtime.State().GlobalCount)+1))
			var invocation ast.ASTArithmeticNumeralInvocation
			active = true
			_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			active = false
			fmMust(t, f.runtime.SetMaxCount(0))
			fmMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
			if callbacks == 0 || body == nil {
				t.Fatal("missing native refusal")
			}
			var construction memory.ConstructionFailure
			if !errors.As(transfer, &construction) || !errors.Is(transfer, ast.ErrASTPending) || !childOpen() {
				t.Fatal("outer transfer lost retryable child", transfer)
			}
			if construction.OriginalCause() == nil || construction.CleanupCause() == nil || !errors.Is(construction.CleanupCause(), ast.ErrASTPending) {
				t.Fatal("outer transfer lost stable construction causes", construction.OriginalCause(), construction.CleanupCause())
			}
			if test.success && !errors.Is(construction.OriginalCause(), ast.ErrASTPending) {
				t.Fatal("successful transfer did not promote cleanup to original cause", construction.OriginalCause())
			}
			reported := transfer.Error()
			for ref, want := range map[ast.ASTRef]ast.ASTHandle{formulaRef: formula, variableRef: variable, current: formula, ownedVariable: variable} {
				actual, err := m.RefGet(ref)
				fmMust(t, err)
				if actual != want {
					t.Fatal("outer transfer root changed", actual, want)
				}
			}
			fmMust(t, construction.Cleanup())
			fmMust(t, construction.Cleanup())
			if childOpen() || transfer.Error() != reported || !errors.Is(transfer, ast.ErrASTPending) || construction.Unwrap() == nil {
				t.Fatal("outer retry changed receipt state", transfer)
			}
			actual, err := m.RefGet(current)
			fmMust(t, err)
			if actual != formula {
				t.Fatal("child retry invalidated current formula")
			}
			actual, err = m.RefGet(ownedVariable)
			fmMust(t, err)
			if actual != variable {
				t.Fatal("child retry invalidated copied variable")
			}
			fmMust(t, m.DelRef(&ownedVariable))
			fmMust(t, m.DelRef(&current))
		})
	}
}
