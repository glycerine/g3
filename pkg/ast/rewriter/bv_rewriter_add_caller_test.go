package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2076-2079,2109-2137.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestBVRewriterAdditionCallerConnectsCoreAndDisjointBits(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	one := bvAddNum(t, m, 1, 8)
	two := bvAddNum(t, m, 2, 8)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{one, two}), out)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteDone || got != bvAddNum(t, m, 3, 8) {
		t.Fatal("numeral core must complete before OR rule", st, got)
	}
	xs := bvMergeVars(t, m, 2)
	low, e := m.MkBitVectorConcat([]ast.ASTHandle{bvAddNum(t, m, 0, 8), xs[0]})
	astTestMust(t, e)
	astTestPin(t, m, low)
	high, e := m.MkBitVectorConcat([]ast.ASTHandle{xs[1], bvAddNum(t, m, 0, 8)})
	astTestMust(t, e)
	astTestPin(t, m, high)
	st, e = p.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{low, high}), out)
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	want, e := m.MkBitVectorOrArguments(ast.SliceArguments([]ast.ASTHandle{low, high}))
	astTestMust(t, e)
	astTestPin(t, m, want)
	if st != Rewrite1 || got != want {
		t.Fatal("FAILED core disjoint terms not reduced", st, got, want)
	}
}

func TestBVRewriterAdditionCallerUsesAssignedCoreArgsAndPassesFullStatus(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	xs := bvMergeVars(t, m, 3)
	zero := bvAddNum(t, m, 0, 8)
	low, e := m.MkBitVectorConcat([]ast.ASTHandle{zero, xs[0]})
	astTestMust(t, e)
	astTestPin(t, m, low)
	high, e := m.MkBitVectorConcat([]ast.ASTHandle{xs[1], zero})
	astTestMust(t, e)
	astTestPin(t, m, high)
	astTestMust(t, p.SetSortSums(true))
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{high, low}), out)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	first, e := m.GetArg(got, 0)
	astTestMust(t, e)
	if st != Rewrite1 || first != low || p.addition != nil {
		t.Fatal("DONE branch ignored assigned sorted core result", st, first, low)
	}
	// A fresh owner has no bit-read invocation. FULL bypasses the after-core rule.
	local, e := c.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Del(&local)) }()
	key := "hoist_mul"
	astTestMust(t, c.SetBoolName(&local, &key, true))
	q, e := NewBVPolynomialRewriter(m, c, g, &local)
	astTestMust(t, e)
	defer func() { astTestMust(t, q.Close()) }()
	left := bvMergeMul(t, m, xs[0], xs[1])
	astTestPin(t, m, left)
	right := bvMergeMul(t, m, xs[0], xs[2])
	astTestPin(t, m, right)
	st, e = q.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{left, right}), out)
	astTestMust(t, e)
	if st != RewriteFull || q.additionBits != (ast.ASTBitVectorNumeralInvocation{}) {
		t.Fatal("FULL entered bit reader", st)
	}
}

func TestBVRewriterAdditionCallerEveryColdReservation(t *testing.T) {
	complete := false
	failures := 0
	for ordinal := int64(0); ordinal < 256 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			c, g, _ := bvPolynomialParamsFixture(t)
			m, _ := bvAddFixture(t)
			p, e := NewBVPolynomialRewriter(m, c, g, nil)
			astTestMust(t, e)
			xs := bvMergeVars(t, m, 2)
			zero := bvAddNum(t, m, 0, 8)
			low, e := m.MkBitVectorConcat([]ast.ASTHandle{zero, xs[0]})
			astTestMust(t, e)
			astTestPin(t, m, low)
			high, e := m.MkBitVectorConcat([]ast.ASTHandle{xs[1], zero})
			astTestMust(t, e)
			astTestPin(t, m, high)
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			m.Budget().FailAfter(ordinal)
			st, body := p.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{low, high}), out)
			m.Budget().FailAfter(-1)
			if body == nil {
				complete = true
				if st != Rewrite1 || p.addition != nil {
					t.Fatal(st)
				}
			} else {
				if !errors.Is(body, arena.ErrInjectedFailure) {
					t.Fatal(body)
				}
				failures++
				cause := p.cause
				if _, e = p.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{low, high}), out); e != cause {
					t.Fatal("stopped retry", e, cause)
				}
				astTestMust(t, p.Close())
				if p.cause != cause {
					t.Fatal("history changed")
				}
			}
			astTestMust(t, p.Close())
		})
	}
	if !complete || failures == 0 {
		t.Fatal("actual addition reservation campaign incomplete", complete, failures)
	}
}

func TestBVRewriterAdditionCallerNativeWideFinalAssignment(t *testing.T) {
	complete := false
	sawPublished := false
	for ordinal := int64(0); ordinal < 256 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			c, g, _ := bvPolynomialParamsFixture(t)
			m, _ := bvAddFixture(t)
			p, e := NewBVPolynomialRewriter(m, c, g, nil)
			astTestMust(t, e)
			xs := bvMergeVars(t, m, 2)
			zero := bvAddNum(t, m, 0, 8)
			low, e := m.MkBitVectorConcat([]ast.ASTHandle{zero, xs[0]})
			astTestMust(t, e)
			astTestPin(t, m, low)
			high, e := m.MkBitVectorConcat([]ast.ASTHandle{xs[1], zero})
			astTestMust(t, e)
			astTestPin(t, m, high)
			s, e := m.GetSort(low)
			astTestMust(t, e)
			older := make([]ast.ASTHandle, 0, 32)
			for i := uint32(0); i < 32; i++ {
				h, e := m.MkVar(i+20, s)
				astTestMust(t, e)
				older = append(older, h)
			}
			old, e := m.MkBitVectorOrArguments(ast.SliceArguments(older))
			astTestMust(t, e)
			out, e := m.NewRef(old)
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			m.Budget().FailAfter(ordinal)
			st, body := p.RewriteBitVectorAddition(ast.SliceArguments([]ast.ASTHandle{low, high}), out)
			m.Budget().FailAfter(-1)
			got, e := m.RefGet(out)
			astTestMust(t, e)
			if body == nil {
				complete = true
				nativeResult, nativeErr := m.RefGet(out)
				astTestMust(t, nativeErr)
				if nativeResult != old {
					sawPublished = true
				}
				yes, e := m.IsBitVectorOperation(nativeResult, 31)
				astTestMust(t, e)
				if !yes {
					t.Fatal("native replacement shape", nativeResult)
				}
				if m.IsLiveHandle(old) {
					t.Fatal("native assignment retained the old root")
				}
				if st != Rewrite1 {
					t.Fatal(st)
				}
			} else {
				if !numericHoistRefusal(body) {
					t.Fatal(body)
				}
				if got != old {
					sawPublished = true
					if st != Rewrite1 {
						t.Fatal("consumed assignment status", st)
					}
					yes, e := m.IsBitVectorOperation(got, 31)
					astTestMust(t, e)
					if !yes {
						t.Fatal("published node is not BOR")
					}
				}
			}
			cause := p.cause
			astTestMust(t, p.Close())
			if p.cause != cause {
				t.Fatal("cleanup changed history")
			}
		})
	}
	if !complete || !sawPublished {
		t.Fatal("native final assignment did not publish", complete, sawPublished)
	}
}
