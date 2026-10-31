package arith

// Source: src/tactic/arith/bv2real_rewriter.h:178-197;
// src/ast/rewriter/rewriter_def.h no-proof traversal output contract.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestBV2RealConfigurationProofResetAndSourcePolicies(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	equationMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	truth, err := f.m.MkTrue()
	equationMust(t, err)
	proof, err := f.m.MkProofAsserted(truth)
	equationMust(t, err)
	fixDLPin(t, f.m, proof)
	pr, out := bv2RealOut(t, f.m), bv2RealOut(t, f.m)
	s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
	for _, op := range []int32{6, 10} {
		input := fixDLApp(t, f.m, op, s, tv)
		decl, err := f.m.GetDecl(input)
		equationMust(t, err)
		if r.cfg.RewritePatterns() {
			t.Fatal("source configuration rewrites quantifier patterns")
		}
		flat, err := r.cfg.FlatAssoc(decl)
		equationMust(t, err)
		if flat {
			t.Fatal("source configuration flattens associative arguments")
		}
		equationMust(t, f.m.SetRef(pr, proof))
		equationMust(t, f.m.SetRef(out, s))
		status, err := r.cfg.ReduceApp(decl, ast.SliceArguments([]ast.ASTHandle{s, tv}), out, pr)
		equationMust(t, err)
		got, err := f.m.RefGet(pr)
		equationMust(t, err)
		if !got.IsNull() {
			t.Fatal("configuration did not clear proof before core reduction")
		}
		got, err = f.m.RefGet(out)
		equationMust(t, err)
		if op == 10 {
			if status != rewriter.RewriteFailed || got != s {
				t.Fatal("refusing core changed result", status, got)
			}
		} else if status != rewriter.RewriteDone || got == s {
			t.Fatal("successful configuration did not publish result", status, got)
		}
	}
}

func TestBV2RealTraversalProofDisabledAndUtilityBorrowed(t *testing.T) {
	r, u, f := bv2RealRewriteFixture(t)
	equationMust(t, f.m.ToggleProofMode(ast.ASTProofEnabled))
	s, tv := bv2RealSymbolicEncoding(t, u, "left", 4, 3), bv2RealSymbolicEncoding(t, u, "right", 6, 3)
	input := mustNLA2BVEq(t, f.m, s, tv)
	proof, err := f.m.MkProofAsserted(input)
	equationMust(t, err)
	pr, out := bv2RealOut(t, f.m), bv2RealOut(t, f.m)
	equationMust(t, f.m.SetRef(pr, proof))
	equationMust(t, r.RewriteWithProof(input, out, pr))
	after, err := f.m.RefGet(pr)
	equationMust(t, err)
	if after != proof {
		t.Fatal("no-proof traversal overwrote caller proof slot")
	}
	result, err := f.m.RefGet(out)
	equationMust(t, err)
	if result == input || result.IsNull() {
		t.Fatal("traversal did not execute the supported equality reducer")
	}
	equationMust(t, r.Reset())
	equationMust(t, r.Close())
	if max, err := u.MaxNumBits(); err != nil || max != 128 {
		t.Fatal("rewriter destroyed its borrowed utility", max, err)
	}
	// The caller result remains owned independently after traversal destruction.
	if _, err = f.m.GetKind(result); err != nil {
		t.Fatal("rewriter destroyed caller result", err)
	}
}

func TestBV2RealHeaderSignatureFieldsAndCanonicalEquality(t *testing.T) {
	u, f := bv2RealFixture(t, 64)
	out := bv2RealOut(t, f.m)
	keys := []struct {
		first, second uint32
		divisor, root int64
	}{
		{3, 5, 2, 2}, {4, 5, 2, 2}, {3, 6, 2, 2}, {5, 3, 2, 2}, {3, 5, 3, 2}, {3, 5, 2, 3},
	}
	var decls []ast.ASTHandle
	for _, key := range keys {
		a, b := f.bv(t, 1, key.first), f.bv(t, 2, key.second)
		equationMust(t, u.MkBV2RealC(a, b, big.NewRat(key.divisor, 1), big.NewRat(key.root, 1), out))
		h, err := f.m.RefGet(out)
		equationMust(t, err)
		decl, err := f.m.GetDecl(h)
		equationMust(t, err)
		for _, prior := range decls {
			if decl == prior {
				t.Fatal("signature equality omitted an ordered width or rational field", key)
			}
		}
		decls = append(decls, decl)
		// Different coefficient values and equal reduced rationals preserve identity.
		equationMust(t, u.MkBV2RealC(f.bv(t, 0, key.first), f.bv(t, 1, key.second), big.NewRat(2*key.divisor, 2), big.NewRat(3*key.root, 3), out))
		same, err := f.m.RefGet(out)
		equationMust(t, err)
		sameDecl, err := f.m.GetDecl(same)
		equationMust(t, err)
		if sameDecl != decl {
			t.Fatal("signature equality depends on coefficient values or unreduced rationals", key)
		}
	}
	aux, err := u.AuxDecls()
	equationMust(t, err)
	if len(aux) != 2+len(keys) {
		t.Fatal("source auxiliary declaration vector count", len(aux))
	}
}
