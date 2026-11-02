//go:build !g3debug

package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The source assertion is erased in non-debug builds; pending clauses do not
// prevent scope reset, including the zero-scope case.
func TestPB2BVPopWithPendingClausesInNativeBuild(t *testing.T) {
	m, r := pb2BVFixture(t)
	astTestMust(t, r.Push())
	astTestMust(t, r.context.Clause(0, ast.ASTLiteralVectorSpan{}))
	astTestMust(t, r.Pop(0))
	astTestMust(t, r.Pop(1))
	clauses, err := r.FlushSideConstraints()
	astTestMust(t, err)
	if len(clauses) != 1 {
		t.Fatal("pop changed pending side constraints", len(clauses))
	}
	for _, clause := range clauses {
		astTestMust(t, m.DecRef(clause))
	}
	if err := r.Pop(1); err != ast.ErrASTDomain {
		t.Fatal("invalid scope did not return a safe error", err)
	}
}
