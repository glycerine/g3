package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestLegacyProjectionParentCleanupContinuation(t *testing.T) {
	for _, kind := range []string{"array", "array-progressed", "combined", "combined-progressed"} {
		t.Run(kind, func(t *testing.T) {
			active, callbacks := false, 0
			var closeOwner func() error
			var first, second error
			f := newFMFixtureTransport(t, false, func(failure memory.Failure) error {
				if active {
					callbacks++
					first = closeOwner()
					second = closeOwner()
				}
				return failure
			})
			m := f.r.manager
			md, err := model.NewModelCore(m)
			fmMust(t, err)
			defer func() { fmMust(t, md.Close()) }()
			root := f.constant("pending-projection-parent", f.integer)
			var array *legacyArrayProjection
			var combined *LegacyProjection
			if kind == "array" || kind == "array-progressed" {
				array, err = newLegacyArrayProjection(m, f.r.config)
				fmMust(t, err)
				call, err := array.syntax.begin(md)
				fmMust(t, err)
				_, err = call.ref(root)
				fmMust(t, err)
				closeOwner = array.Close
			} else {
				combined, err = NewLegacyProjection(m, f.r.config)
				fmMust(t, err)
				call, err := combined.arithmetic.syntax.begin(md)
				fmMust(t, err)
				_, err = call.hold(root, nil)
				fmMust(t, err)
				closeOwner = combined.Close
			}
			// Stage a valid cleanup continuation by completing the preceding
			// child normally; the remaining pending call owns a real AST root.
			// This tests continuation state, not how an earlier Close failed.
			if kind == "array-progressed" {
				fmMust(t, array.theory.Close())
				array.theory = nil
			}
			if kind == "combined-progressed" {
				fmMust(t, combined.array.Close())
				combined.array = nil
			}
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
				t.Fatal("missing numerical refusal")
			}
			if !errors.Is(first, ast.ErrASTPending) || !errors.Is(second, ast.ErrASTPending) {
				t.Fatal("cleanup continuation changed refusal", first, second)
			}
			if array != nil {
				if array.syntax == nil || len(array.syntax.pending) != 1 || len(array.syntax.pending[0].refs) != 1 {
					t.Fatal("lost pending array owner")
				}
			}
			if combined != nil {
				if combined.arithmetic == nil || combined.arithmetic.syntax == nil || len(combined.arithmetic.syntax.pending) != 1 || len(combined.arithmetic.syntax.pending[0].refs) != 1 {
					t.Fatal("lost pending arithmetic owner")
				}
			}
			if combined != nil {
				truth, err := m.MkTrue()
				fmMust(t, err)
				for _, native := range []bool{false, true} {
					// A closing owner must reject before any allocation or
					// mutation of the still-owned pending child call.
					m.Budget().FailAfter(0)
					var result *LegacyProjectionResult
					if native {
						result, err = combined.ProjectNativeArithmetic(md, nil, truth, false, true)
					} else {
						result, err = combined.Project(md, nil, truth, false, true)
					}
					m.Budget().FailAfter(-1)
					if result != nil {
						fmMust(t, result.Close())
						t.Fatal("closing projection published a result")
					}
					if err != ast.ErrASTHandle {
						t.Fatal("closing projection entered query work", native, err)
					}
				}
			}
			var pendingRoot ast.ASTRef
			if array != nil {
				pendingRoot = array.syntax.pending[0].refs[0]
			} else {
				pendingRoot = combined.arithmetic.syntax.pending[0].refs[0]
			}
			actual, err := m.RefGet(pendingRoot)
			fmMust(t, err)
			if actual != root {
				t.Fatal("pending root identity changed")
			}
			fmMust(t, closeOwner())
			if array != nil && (array.syntax != nil || array.theory != nil) {
				t.Fatal("array cleanup incomplete")
			}
			if combined != nil && (combined.manager != nil || combined.array != nil || combined.arithmetic != nil) {
				t.Fatal("combined cleanup incomplete")
			}

		})
	}
}
