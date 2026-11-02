package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyArithmeticCoordinatorPreservesTransferredChildOnRefusedClose(t *testing.T) {
	active, callbacks := false, 0
	var transfer error
	var child *legacyRealProjection
	f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
		if active {
			callbacks++
			transfer = closeTransferredLegacyRealProjection(child, failure)
		}
		return failure
	})
	m := f.r.manager
	substitution := f.constant("arithmetic-transfer-substitution", f.real)
	divisibility := f.constant("arithmetic-transfer-divisibility", f.boolean)
	literal := f.constant("arithmetic-transfer-literal", f.boolean)
	replacement := f.constant("arithmetic-transfer-replacement", f.boolean)
	substitutionRef, err := m.NewRef(substitution)
	fmMust(t, err)
	divisibilityRef, err := m.NewRef(divisibility)
	fmMust(t, err)
	replacementRef, err := m.NewRef(replacement)
	fmMust(t, err)
	child = &legacyRealProjection{
		manager: m, substitution: substitutionRef, divisibility: divisibilityRef,
		replacements: []legacyLiteralReplacement{{literal: literal, replacement: replacementRef}},
	}
	parentFormula, err := m.NewRef(literal)
	fmMust(t, err)
	parent := &legacyFormulaProjection{manager: m, formula: parentFormula}

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
	if child.manager != m || child.substitution != substitutionRef || child.divisibility != divisibilityRef || len(child.replacements) != 1 || child.replacements[0].replacement != replacementRef {
		t.Fatal("refused close changed arithmetic child ownership")
	}
	for ref, want := range map[ast.ASTRef]ast.ASTHandle{
		child.substitution: substitution, child.divisibility: divisibility, child.replacements[0].replacement: replacement,
	} {
		actual, err := m.RefGet(ref)
		fmMust(t, err)
		if actual != want {
			t.Fatal("arithmetic child root changed", actual, want)
		}
	}
	actualParent, err := m.RefGet(parent.formula)
	fmMust(t, err)
	if actualParent != literal {
		t.Fatal("parent formula changed", actualParent, literal)
	}
	fmMust(t, construction.Cleanup())
	fmMust(t, construction.Cleanup())
	if transfer.Error() != reported || !errors.Is(transfer, ast.ErrASTPending) || construction.Unwrap() == nil {
		t.Fatal("successful retry changed reported refusal", transfer)
	}
	if child.manager != nil {
		t.Fatal("arithmetic child cleanup retry incomplete")
	}
	actualParent, err = m.RefGet(parent.formula)
	fmMust(t, err)
	if actualParent != literal {
		t.Fatal("child retry invalidated parent formula")
	}
	fmMust(t, parent.Close())
}
