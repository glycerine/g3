package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSMTAssertionScopeKeepsRefusedNativeRoots(t *testing.T) {
	for _, test := range []string{"inputs", "rewritten", "nested"} {
		t.Run(test, func(t *testing.T) {
			fixture := newSMTNativeCleanupFixture(t)
			m := fixture.manager
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			expression := smtRelevancyBool(t, m, "assertion-scope-expression")
			proof, err := m.MkProofAsserted(expression)
			astTestMust(t, err)
			var caller [2]ast.ASTRef
			for i, value := range []ast.ASTHandle{expression, proof} {
				caller[i], err = m.NewRef(value)
				astTestMust(t, err)
			}
			defer func() {
				for i := len(caller) - 1; i >= 0; i-- {
					astTestMust(t, m.DelRef(&caller[i]))
				}
			}()
			before := m.Stats()
			var callerCounts [2]uint32
			for i, value := range []ast.ASTHandle{expression, proof} {
				callerCounts[i], err = m.GetRefCount(value)
				astTestMust(t, err)
			}
			scope := &smtAssertionScope{manager: m}
			for i, value := range []ast.ASTHandle{expression, proof} {
				scope.inputs[i], err = m.NewRef(value)
				astTestMust(t, err)
				if test != "inputs" {
					astTestMust(t, m.IncRef(value))
					scope.rewritten[i] = value
				}
			}
			var nested *smtAssertionScope
			if test == "nested" {
				nested = &smtAssertionScope{manager: m}
				nested.inputs[0], err = m.NewRef(expression)
				astTestMust(t, err)
			}
			ownedInputs, ownedRewritten := scope.inputs, scope.rewritten
			counts := [2]uint32{}
			for i, value := range []ast.ASTHandle{expression, proof} {
				counts[i], err = m.GetRefCount(value)
				astTestMust(t, err)
			}
			operationCause := errors.New("assertion preprocessing stopped")
			var failure error
			fixture.refuse(t, func() {
				cause := operationCause
				if nested != nil {
					cause = nested.finish(cause)
				}
				failure = scope.finish(cause)
			})
			if !errors.Is(failure, operationCause) || !errors.Is(failure, ast.ErrASTPending) {
				t.Fatal("assertion cleanup lost body or native refusal", failure)
			}
			if scope.inputs != ownedInputs || scope.rewritten != ownedRewritten {
				t.Fatal("refused cleanup discarded an assertion root")
			}
			for i, value := range []ast.ASTHandle{expression, proof} {
				count, err := m.GetRefCount(value)
				astTestMust(t, err)
				if count != counts[i] {
					t.Fatal("native refusal changed caller or scoped ownership", i, counts[i], count)
				}
			}
			var receipt memory.ConstructionFailure
			if !errors.As(failure, &receipt) || receipt.RuntimeOwner() != fixture.runtime {
				t.Fatal("missing independent assertion cleanup receipt", failure)
			}
			original, exposed, message := receipt.OriginalCause(), receipt.Unwrap(), receipt.Error()
			astTestMust(t, receipt.Cleanup())
			astTestMust(t, receipt.Cleanup())
			if scope.inputs != ([2]ast.ASTRef{}) || scope.rewritten != ([2]ast.ASTHandle{}) || scope.nested != nil {
				t.Fatal("cleanup left lexical assertion roots")
			}
			if nested != nil && nested.inputs != ([2]ast.ASTRef{}) {
				t.Fatal("cleanup left the nested assertion input")
			}
			for i, value := range []ast.ASTHandle{expression, proof} {
				count, err := m.GetRefCount(value)
				astTestMust(t, err)
				if count != callerCounts[i] {
					t.Fatal("retry did not restore exact caller reference counts", i, callerCounts[i], count)
				}
			}
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveParameters != before.LiveParameters || after.LiveInfos != before.LiveInfos {
				t.Fatal("retry changed caller AST ownership", before, after)
			}
			if receipt.CleanupCause() != nil || receipt.OriginalCause() != original || receipt.Unwrap() != exposed || receipt.Error() != message {
				t.Fatal("cleanup changed the escaped assertion error")
			}
		})
	}
}
