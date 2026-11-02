package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArrayCoordinatorPreservesTransferredChildOnRefusedClose(t *testing.T) {
	active, callbacks := false, 0
	var transfer error
	var child *legacyArraySelectProjection
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
		if active {
			callbacks++
			transfer = closeTransferredLegacyArraySelect(child, failure)
		}
		return failure
	})
	m := f.r.manager
	formula := f.constant("array-transfer-formula", f.boolean)
	auxiliary := f.constant("array-transfer-auxiliary", f.integer)
	formulaRef, err := m.NewRef(formula)
	fmMust(t, err)
	auxiliaryRef, err := m.NewRef(auxiliary)
	fmMust(t, err)
	child = &legacyArraySelectProjection{manager: m, formula: formulaRef, variables: []ast.ASTRef{auxiliaryRef}}
	parentFormula, err := m.CopyRef(formulaRef)
	fmMust(t, err)
	parentAuxiliary, err := m.CopyRef(auxiliaryRef)
	fmMust(t, err)
	parent := &legacyArrayProjectionResult{manager: m, formula: parentFormula, variables: []ast.ASTRef{parentAuxiliary}}

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
	if !errors.As(transfer, &construction) || !errors.Is(transfer, ast.ErrASTPending) {
		t.Fatal("transfer did not publish retryable child", transfer)
	}
	reported := transfer.Error()
	if child.manager != m || child.formula != formulaRef || len(child.variables) != 1 || child.variables[0] != auxiliaryRef {
		t.Fatal("refused close changed child ownership")
	}
	actualFormula, err := m.RefGet(parent.formula)
	fmMust(t, err)
	actualAuxiliary, err := m.RefGet(parent.variables[0])
	fmMust(t, err)
	if actualFormula != formula || len(parent.variables) != 1 || actualAuxiliary != auxiliary {
		t.Fatal("transferred parent outputs changed", actualFormula, actualAuxiliary)
	}
	fmMust(t, construction.Cleanup())
	fmMust(t, construction.Cleanup())
	if transfer.Error() != reported || !errors.Is(transfer, ast.ErrASTPending) || construction.Unwrap() == nil {
		t.Fatal("successful retry changed the reported cleanup refusal", transfer, construction.Unwrap())
	}
	if child.manager != nil {
		t.Fatal("child cleanup retry incomplete")
	}
	actualFormula, err = m.RefGet(parent.formula)
	fmMust(t, err)
	if actualFormula != formula {
		t.Fatal("child retry invalidated parent copy")
	}
	fmMust(t, parent.Close())
}
