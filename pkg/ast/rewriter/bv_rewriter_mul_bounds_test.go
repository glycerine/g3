package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp mk_bvsmul_no_overflow (upstream
// z3 4.17.0 algorithm, backported to the fork) and mk_bvumul_no_overflow.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The folds match upstream z3 and the patched fork, including the 4.7.1
// bugs they fix: #xa * #x3 is a signed underflow, not an overflow, and at
// width 1 the bit pattern 1 is -1, so (-1)*(-1) overflows (no "1" shortcut).
// Same cases as test_vectors/bv_smul_noovfl_fold.smt2.
func TestBVRewriterMultiplyBoundFoldsLikeUpstream(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, err := NewBVRewriter(m, c, g, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	for _, tc := range []struct {
		a, b, width uint32
		mode        BVMultiplyBound
		want        bool
	}{
		{1, 1, 1, BVSignedNoOverflow, false},
		{1, 0, 1, BVSignedNoOverflow, true},
		{0xa, 0x3, 4, BVSignedNoOverflow, true},
		{0xa, 0xa, 4, BVSignedNoOverflow, false},
		{0x3, 0x3, 4, BVSignedNoOverflow, false},
		{0x7, 0x2, 4, BVSignedNoOverflow, false},
		{0x8, 0xf, 4, BVSignedNoOverflow, false},
		{0x3, 0x2, 4, BVSignedNoOverflow, true},
		{0xa, 0x3, 4, BVSignedNoUnderflow, false},
		{1, 1, 1, BVSignedNoUnderflow, true},
		{0x8, 0x1, 4, BVSignedNoUnderflow, true},
		{0xc, 0x2, 4, BVSignedNoUnderflow, true},
		{0xc, 0x3, 4, BVSignedNoUnderflow, false},
		{128, 2, 8, BVSignedNoUnderflow, false},
		{255, 2, 8, BVSignedNoUnderflow, true},
		{128, 2, 8, BVUnsignedNoOverflow, false},
		{1, 1, 1, BVUnsignedNoOverflow, true},
	} {
		args := ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, tc.a, tc.width), bvAddNum(t, m, tc.b, tc.width)})
		status, e := p.RewriteMultiplyBound(args, out, tc.mode)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if status != RewriteDone || m.IsTrue(got) != tc.want {
			t.Fatalf("%+v: status=%v true=%v", tc, status, m.IsTrue(got))
		}
	}
}
