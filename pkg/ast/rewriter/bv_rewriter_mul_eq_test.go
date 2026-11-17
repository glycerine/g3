package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2448-2508.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVRewriterMultiplicationEqualityOriginalBranches(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	xs := bvMergeVars(t, m, 2)
	three := bvAddNum(t, m, 3, 8)
	inverse := bvAddNum(t, m, 171, 8)
	lhs := bvMergeMul(t, m, three, xs[0])
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	sum, e := m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments([]ast.ASTHandle{bvMergeMul(t, m, three, xs[1]), bvAddNum(t, m, 2, 8)}))
	astTestMust(t, e)
	astTestPin(t, m, sum)
	cases := []struct {
		lhs, rhs, target ast.ASTHandle
		status           RewriteStatus
	}{
		{lhs, bvAddNum(t, m, 6, 8), bvAddNum(t, m, 2, 8), Rewrite1},
		{lhs, bvMergeMul(t, m, three, xs[1]), xs[1], Rewrite1},
		{lhs, bvMergeMul(t, m, bvAddNum(t, m, 5, 8), xs[1]), bvMergeMul(t, m, bvAddNum(t, m, 87, 8), xs[1]), Rewrite1},
		{lhs, sum, bvMergeMul(t, m, inverse, sum), Rewrite2},
	}
	for _, tc := range cases {
		st, e := p.RewriteMultiplicationEquality(tc.lhs, tc.rhs, out)
		astTestMust(t, e)
		want, e := m.MkEq(xs[0], tc.target)
		astTestMust(t, e)
		astTestPin(t, m, want)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if st != tc.status || got != want {
			t.Fatal(st, tc.status, got, want)
		}
	}
}

func TestBVRewriterMultiplicationEqualitySearchAndFailedGuards(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	xs := bvMergeVars(t, m, 3)
	six := bvAddNum(t, m, 6, 8)
	sum, e := m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 1, 8), bvMergeMul(t, m, bvAddNum(t, m, 2, 8), xs[0]), bvMergeMul(t, m, bvAddNum(t, m, 5, 8), xs[1])}))
	astTestMust(t, e)
	astTestPin(t, m, sum)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteMultiplicationEquality(six, sum, out)
	astTestMust(t, e)
	right := bvMergeMul(t, m, bvAddNum(t, m, 205, 8), sum)
	want, e := m.MkEq(bvAddNum(t, m, 206, 8), right)
	astTestMust(t, e)
	astTestPin(t, m, want)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != Rewrite3 || got != want {
		t.Fatal(st, got, want)
	}
	evenSum, e := m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments([]ast.ASTHandle{bvMergeMul(t, m, bvAddNum(t, m, 2, 8), xs[0]), bvAddNum(t, m, 1, 8)}))
	astTestMust(t, e)
	astTestPin(t, m, evenSum)
	unsupported, e := m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments(xs[:2]))
	astTestMust(t, e)
	astTestPin(t, m, unsupported)
	for _, pair := range [][2]ast.ASTHandle{
		{xs[0], xs[1]}, {bvMergeMul(t, m, bvAddNum(t, m, 2, 8), xs[0]), six},
		{bvMergeMul(t, m, xs[0], bvAddNum(t, m, 3, 8)), six},
		{bvMergeMul(t, m, bvAddNum(t, m, 3, 8), xs[0], xs[1]), six},
		{bvMergeMul(t, m, bvAddNum(t, m, 3, 8), xs[0]), unsupported},
		{six, evenSum},
	} {
		st, e = p.RewriteMultiplicationEquality(pair[0], pair[1], out)
		astTestMust(t, e)
		got, e = m.RefGet(out)
		astTestMust(t, e)
		if st != RewriteFailed || got != want || p.mulEquality != nil {
			t.Fatal(st, got, want)
		}
	}
}

func TestBVRewriterMultiplicationEqualityEveryColdReservation(t *testing.T) {
	for _, branch := range []string{"constant", "coefficient", "sum", "search"} {
		complete := false
		sawPublished := false
		for ordinal := int64(0); ordinal < 2048 && !complete; ordinal++ {
			t.Run(fmt.Sprintf("%s/%d", branch, ordinal), func(t *testing.T) {
				c, g, _ := bvPolynomialParamsFixture(t)
				m, _ := bvAddFixture(t)
				p, e := NewBVPolynomialRewriter(m, c, g, nil)
				astTestMust(t, e)
				xs := bvMergeVars(t, m, 3)
				lhs := bvMergeMul(t, m, bvAddNum(t, m, 3, 8), xs[0])
				rhs := bvAddNum(t, m, 6, 8)
				status := Rewrite1
				if branch == "coefficient" {
					rhs = bvMergeMul(t, m, bvAddNum(t, m, 5, 8), xs[1])
				}
				if branch == "sum" || branch == "search" {
					rhs, e = m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments([]ast.ASTHandle{bvMergeMul(t, m, bvAddNum(t, m, 2, 8), xs[1]), bvMergeMul(t, m, bvAddNum(t, m, 5, 8), xs[2])}))
					astTestMust(t, e)
					astTestPin(t, m, rhs)
					status = Rewrite2
					if branch == "search" {
						lhs = bvAddNum(t, m, 6, 8)
						status = Rewrite3
					}
				}
				s, e := m.BitVectorSort(8)
				astTestMust(t, e)
				older := make([]ast.ASTHandle, 32)
				for i := range older {
					older[i], e = m.MkVar(uint32(i+100), s)
					astTestMust(t, e)
				}
				old, e := m.MkBitVectorOr(older)
				astTestMust(t, e)
				out, e := m.NewRef(old)
				astTestMust(t, e)
				defer func() { astTestMust(t, m.DelRef(&out)) }()
				m.Budget().FailAfter(ordinal)
				st, body := p.RewriteMultiplicationEquality(lhs, rhs, out)
				m.Budget().FailAfter(-1)
				if body == nil {
					complete = true
					nativeResult, nativeErr := m.RefGet(out)
					astTestMust(t, nativeErr)
					if nativeResult != old {
						sawPublished = true
					}
					if st != status || p.mulEquality != nil {
						t.Fatal(st, status)
					}
				} else {
					if !numericHoistRefusal(body) {
						t.Fatal(body)
					}
					got, e := m.RefGet(out)
					astTestMust(t, e)
					if got != old {
						sawPublished = true
						if st != status {
							t.Fatal("published status", st, status)
						}
					}
					cause := p.cause
					if _, e = p.RewriteMultiplicationEquality(lhs, rhs, out); e != cause {
						t.Fatal("stopped replay", e, cause)
					}
					astTestMust(t, p.Close())
					if p.cause != cause {
						t.Fatal("history")
					}
				}
				astTestMust(t, p.Close())
			})
		}
		if !complete || !sawPublished {
			t.Fatal(branch, complete, sawPublished)
		}
	}
}

// Raft compares symbolic bit-vector terms frequently. Neither branch of
// mk_mul_eq can apply to x = y or 1 = y when x and y are variables.
func TestBVRewriterSymbolicMulEqualityAllocations(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, err := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	x := bvMergeVars(t, m, 2)
	out, err := m.NewRef(x[0])
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	for _, lhs := range []ast.ASTHandle{x[0], bvAddNum(t, m, 1, 8)} {
		allocs := testing.AllocsPerRun(100, func() {
			status, err := p.RewriteMultiplicationEquality(lhs, x[1], out)
			if err != nil || status != RewriteFailed {
				t.Fatalf("symbolic equality: %v, %v", status, err)
			}
		})
		if allocs != 0 {
			t.Errorf("inapplicable equality rule allocated %.0f times per attempt", allocs)
		}
	}
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got != x[0] {
		t.Fatal("failed rewrite changed the output")
	}
}
