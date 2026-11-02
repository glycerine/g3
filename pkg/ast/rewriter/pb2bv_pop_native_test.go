//go:build !g3debug

package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Pending clauses must be flushed before a scope reset in normal builds too.
func TestPB2BVPopWithPendingClausesInNativeBuild(t *testing.T) {
	m, r := pb2BVFixture(t)
	astTestMust(t, r.Push())
	astTestMust(t, r.context.Clause(0, ast.ASTLiteralVectorSpan{}))
	for _, scopes := range []uint32{0, 1} {
		if err := r.Pop(scopes); err != ast.ErrASTDomain {
			t.Fatal("pop accepted unflushed clauses", scopes, err)
		}
		if len(r.freshLimits) != 1 {
			t.Fatal("rejected pop changed scope", scopes)
		}
	}
	clauses, err := r.FlushSideConstraints()
	astTestMust(t, err)
	if len(clauses) != 1 {
		t.Fatal("pop changed pending side constraints", len(clauses))
	}
	for _, clause := range clauses {
		astTestMust(t, m.DecRef(clause))
	}
	astTestMust(t, r.Pop(1))
	if err := r.Pop(1); err != ast.ErrASTDomain {
		t.Fatal("invalid scope did not return a safe error", err)
	}
}
