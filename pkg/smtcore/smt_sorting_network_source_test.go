package smtcore

// Source: src/test/sorting_network.cpp:184-324,326-330,436-440.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestPsortRegisteredComparisonsUseSMTKernel(t *testing.T) {
	// The source test_sorting5 drives each relation through smt::kernel. The
	// AST-package campaigns check every bound semantically; this integration
	// test also exercises the Go kernel's source push/assert/check/pop sequence.
	for n := uint32(3); n < 20; n += 2 {
		for k := uint32(1); k < n; k++ {
			for _, tc := range []struct {
				name string
				mode int
			}{{"le", 1}, {"eq", 2}, {"ge", 0}} {
				t.Run(fmt.Sprintf("n=%d/k=%d/%s", n, k, tc.name), func(t *testing.T) {
					m := astTestManager(t)
					context, err := ast.NewPsortASTContext(m)
					astTestMust(t, err)
					runtime, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.BudgetField})
					astTestMust(t, err)
					network, err := ast.NewPsortASTNetwork(context, runtime, m.BudgetField, nil)
					astTestMust(t, err)
					input, err := ast.NewASTLiteralVector(m.BudgetField)
					astTestMust(t, err)
					inputs := make([]ast.ASTHandle, 0, n)
					for i := uint32(0); i < n; i++ {
						h, err := context.Fresh()
						astTestMust(t, err)
						inputs = append(inputs, h)
						astTestMust(t, input.Push(h))
					}
					span, err := input.Span()
					astTestMust(t, err)
					var result ast.ASTHandle
					switch tc.mode {
					case 0:
						result, err = network.Ge(false, k, n, span)
					case 1:
						result, err = network.Le(false, k, n, span)
					case 2:
						result, err = network.Eq(true, k, n, span)
					}
					astTestMust(t, err)
					kernel, err := NewSMTKernel(m, nil)
					astTestMust(t, err)
					t.Cleanup(func() {
						if !kernel.closed {
							astTestMust(t, kernel.Close())
						}
					})
					astTestMust(t, kernel.Push())
					astTestMust(t, kernel.Assert(result, ast.ASTHandle{}))
					clauses, err := context.TakeClauses()
					astTestMust(t, err)
					for _, clause := range clauses {
						astTestMust(t, kernel.Assert(clause, ast.ASTHandle{}))
					}
					check := func(want lbool.LBool, where string) {
						t.Helper()
						got, err := kernel.Check()
						astTestMust(t, err)
						if got != want {
							t.Fatalf("%s check = %v, want %v", where, got, want)
						}
					}
					check(lbool.True, "base")
					if tc.mode == 0 {
						astTestMust(t, kernel.Push())
						for i := uint32(0); i < n-k; i++ {
							notInput, err := m.MkNot(inputs[i])
							astTestMust(t, err)
							astTestMust(t, kernel.Assert(notInput, ast.ASTHandle{}))
						}
						check(lbool.True, "source false prefix")
						notBoundary, err := m.MkNot(inputs[n-k])
						astTestMust(t, err)
						astTestMust(t, kernel.Assert(notBoundary, ast.ASTHandle{}))
						check(lbool.False, "source false boundary")
						astTestMust(t, kernel.Pop(1))
					} else {
						for i := uint32(0); i < k; i++ {
							astTestMust(t, kernel.Assert(inputs[i], ast.ASTHandle{}))
						}
						check(lbool.True, "source true prefix")
						astTestMust(t, kernel.Assert(inputs[k], ast.ASTHandle{}))
						check(lbool.False, "source true boundary")
						astTestMust(t, kernel.Pop(1))
					}
					astTestMust(t, kernel.Close())
					for _, clause := range clauses {
						astTestMust(t, m.DecRef(clause))
					}
					astTestMust(t, network.Close())
					astTestMust(t, input.Close())
					astTestMust(t, runtime.Close())
					astTestMust(t, context.Close())
				})
			}
		}
	}
}
