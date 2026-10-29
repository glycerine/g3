package rewriter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Source: src/ast/rewriter/pb2bv_rewriter.cpp:401-477;
// src/ast/rewriter/pb2bv_rewriter.h:26-43.
func TestPB2BVCleanupFailureRetiresFreshAndPreservesProofAndParams(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	c, p := enum2BVParameters(t, m)
	key := astTestSymbol(t, m, "pb-cleanup-owner")
	astTestMust(t, c.SetUInt(p, key, 37))
	r, err := NewPB2BVRewriter(m, c, p)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })

	vars := make([]ast.ASTHandle, 7)
	for i := range vars {
		vars[i] = goalTestAtom(t, m, fmt.Sprintf("pb-cleanup-retire-%d", i))
	}
	input, err := m.MkPseudoBooleanAtMost(vars, 2)
	astTestMust(t, err)
	astTestPin(t, m, input)
	asserted, err := m.MkProofAsserted(vars[0])
	astTestMust(t, err)
	proof := astTestPin(t, m, asserted)
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)

	astTestMust(t, r.Rewrite(input, out, proof))
	actualProof, err := m.RefGet(proof)
	astTestMust(t, err)
	if actualProof != asserted {
		t.Fatal("no-proof traversal changed the caller proof", actualProof)
	}
	fresh, err := r.FreshConstants()
	astTestMust(t, err)
	if len(fresh) < 2 {
		t.Fatal("fixture did not reach the fresh declaration range")
	}
	// The rewritten result is caller-owned and would correctly keep its fresh
	// declarations alive. Retire it before checking the rewriter's own range.
	astTestMust(t, m.DelRef(&out))

	m.Budget().FailAfter(0)
	err = r.Cleanup()
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) || r.header != 0 {
		t.Fatal("cleanup did not stop after source member destruction", err, r.header)
	}
	for _, declaration := range fresh {
		if _, e := m.GetKind(declaration); !errors.Is(e, ast.ErrASTHandle) {
			t.Fatal("consumed fresh declaration remained in failed reconstruction", e)
		}
	}
	actualProof, err = m.RefGet(proof)
	astTestMust(t, err)
	if actualProof != asserted {
		t.Fatal("failed reconstruction acquired caller proof ownership", actualProof)
	}
	value, err := c.GetUInt(&r.cleanupParameters, key, 0)
	astTestMust(t, err)
	if value != 37 {
		t.Fatal("cleanup local did not outlive destroyed implementation", value)
	}

	astTestMust(t, r.Cleanup())
	if r.cleanupParameters != (uparams.ParamsRef{}) {
		t.Fatal("successful reconstruction retained temporary parameter owner")
	}
	value, err = c.GetUInt(&r.parameters, key, 0)
	astTestMust(t, err)
	if value != 37 {
		t.Fatal("reconstructed implementation lost constructor parameters", value)
	}
	if _, err = r.Manager(); err != nil {
		t.Fatal("reconstructed implementation is not usable", err)
	}
}
