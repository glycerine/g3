package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2706-2800.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVRewriterMultiplyBoundsOriginalBranches(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	cases := []struct {
		mode BVMultiplyBound
		a, b uint32
		want bool
	}{
		{BVUnsignedNoOverflow, 16, 15, true}, {BVUnsignedNoOverflow, 16, 16, false},
		{BVSignedNoOverflow, 8, 15, true}, {BVSignedNoOverflow, 8, 16, false},
		{BVSignedNoUnderflow, 128, 2, false}, {BVSignedNoUnderflow, 255, 2, true}, {BVSignedNoUnderflow, 255, 255, true},
	}
	for _, tc := range cases {
		st, e := p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, tc.a, 8), bvAddNum(t, m, tc.b, 8)}), out, tc.mode)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if st != RewriteDone || m.IsTrue(got) != tc.want {
			t.Fatal(tc, st, got)
		}
	}
	x := bvMergeVars(t, m, 1)[0]
	st, e := p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 1, 8), x}), out, BVSignedNoUnderflow)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteDone || !m.IsTrue(got) {
		t.Fatal("one fast path", st)
	}
	st, e = p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{x, x}), out, BVUnsignedNoOverflow)
	astTestMust(t, e)
	now, e := m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteFailed || now != got {
		t.Fatal("FAILED output", st)
	}
}

func TestBVRewriterSignedMultiplyBoundNumerals(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, err := NewBVRewriter(m, c, g, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	out, err := m.NewRef(ast.ASTHandle{})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	for width := uint32(1); width <= 4; width++ {
		limit := uint32(1) << width
		sign := int64(limit / 2)
		for a := uint32(0); a < limit; a++ {
			for b := uint32(0); b < limit; b++ {
				left, right := int64(a), int64(b)
				if left >= sign {
					left -= int64(limit)
				}
				if right >= sign {
					right -= int64(limit)
				}
				for _, tc := range []struct {
					mode BVMultiplyBound
					want bool
				}{
					{BVSignedNoOverflow, left*right < sign},
					{BVSignedNoUnderflow, left*right >= -sign},
				} {
					st, err := p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, a, width), bvAddNum(t, m, b, width)}), out, tc.mode)
					astTestMust(t, err)
					got, err := m.RefGet(out)
					astTestMust(t, err)
					if st != RewriteDone || m.IsTrue(got) != tc.want {
						t.Fatalf("width %d: %d * %d mode %d: got %v want %v", width, left, right, tc.mode, got, tc.want)
					}
				}
			}
		}
	}
	sort, err := m.BitVectorSort(1)
	astTestMust(t, err)
	x, err := m.MkVar(0, sort)
	astTestMust(t, err)
	for _, mode := range []BVMultiplyBound{BVSignedNoOverflow, BVSignedNoUnderflow} {
		st, err := p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 1, 1), x}), out, mode)
		astTestMust(t, err)
		if st != RewriteFailed {
			t.Fatal("one-bit -1 with unknown operand must not fold", mode, st)
		}
	}
}

func TestBVRewriterMultiplyBoundsStoredWidthAndColdPublication(t *testing.T) {
	for _, mode := range []BVMultiplyBound{BVSignedNoOverflow, BVUnsignedNoOverflow, BVSignedNoUnderflow} {
		complete, published := false, false
		for ordinal := int64(0); ordinal < 512 && !complete; ordinal++ {
			t.Run(fmt.Sprintf("%d/%d", mode, ordinal), func(t *testing.T) {
				c, g, _ := bvPolynomialParamsFixture(t)
				m, _ := bvAddFixture(t)
				p, e := NewBVRewriter(m, c, g, nil)
				astTestMust(t, e)
				a, b := bvAddNum(t, m, 128, 8), bvAddNum(t, m, 2, 8)
				sort, e := m.BitVectorSort(8)
				astTestMust(t, e)
				older := make([]ast.ASTHandle, 32)
				for i := range older {
					older[i], e = m.MkVar(uint32(i+100), sort)
					astTestMust(t, e)
				}
				old, e := m.MkBitVectorOr(older)
				astTestMust(t, e)
				out, e := m.NewRef(old)
				astTestMust(t, e)
				defer func() { astTestMust(t, m.DelRef(&out)) }()
				m.Budget().FailAfter(ordinal)
				st, body := p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{a, b}), out, mode)
				m.Budget().FailAfter(-1)
				if body == nil {
					complete = true
					nativeResult, nativeErr := m.RefGet(out)
					astTestMust(t, nativeErr)
					if nativeResult != old {
						published = true
					}
					if st != RewriteDone {
						t.Fatal(st)
					}
				} else {
					if !numericHoistRefusal(body) {
						t.Fatal(body)
					}
					got, e := m.RefGet(out)
					astTestMust(t, e)
					if got != old {
						published = true
						if st != RewriteDone {
							t.Fatal("published status", st)
						}
					}
					if _, e = p.RewriteMultiplyBound(ast.AppArguments{}, out, mode); e != body {
						t.Fatal("stopped replay", e, body)
					}
				}
				cause := p.cause
				astTestMust(t, p.Close())
				astTestMust(t, p.Close())
				if p.cause != cause {
					t.Fatal("cleanup changed history")
				}
			})
		}
		if !complete || !published {
			t.Fatal("bound campaign incomplete", mode, complete, published)
		}
	}
	// The second successful numeral read overwrites the first stored width.
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteMultiplyBound(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 4, 8), bvAddNum(t, m, 3, 4)}), out, BVSignedNoOverflow)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteDone || !m.IsFalse(got) {
		t.Fatal("rhs width must select limit 8", st)
	}
}
