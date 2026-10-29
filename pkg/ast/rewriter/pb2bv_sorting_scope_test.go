package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp sorting auxiliaries/scopes.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVSortingProfileFreshAndClauseScope(t *testing.T) {
	m, r := pb2BVFixture(t)
	pb2BVSetProfile(t, r, "pb.solver", "sorting")
	astTestMust(t, r.Push())
	h := pb2BVWeightedFixture(t, m)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	fresh, e := r.FreshConstants()
	astTestMust(t, e)
	clauses, e := r.FlushSideConstraints()
	astTestMust(t, e)
	if len(clauses) == 0 {
		t.Fatal("sorting profile omitted owned network graph", len(fresh), len(clauses))
	}
	for _, clause := range clauses {
		astTestMust(t, m.DecRef(clause))
	}
	astTestMust(t, r.Pop(1))
	fresh, e = r.FreshConstants()
	astTestMust(t, e)
	if len(fresh) != 0 {
		t.Fatal("sorting fresh declarations survived source pop", len(fresh))
	}
}
