package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mk_le mixed-radix refusal.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVSortingProfileNegativeDualFallsBackToBV(t *testing.T) {
	m, r := pb2BVFixture(t)
	pb2BVSetProfile(t, r, "pb.solver", "sorting")
	args := []ast.ASTHandle{goalTestAtom(t, m, "fallback-a"), goalTestAtom(t, m, "fallback-b")}
	values := pbTestRationals(t, m, "2", "3", "6")
	h, e := m.MkPseudoBooleanLE(values[:2], args, &values[2])
	astTestMust(t, e)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got == h {
		t.Fatal("negative mixed-radix dual failed to reach BV fallback")
	}
	stats, e := r.CollectStatistics()
	astTestMust(t, e)
	if stats.CompiledBV != 1 {
		t.Fatal("source weighted compile counter", stats)
	}
}
