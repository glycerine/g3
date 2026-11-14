package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2170-2205.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVRewriterMultiplicationPowerOfTwoCaller(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	ex, e := NewMkExtractProc(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ex.Close()) }()
	x := bvMergeVars(t, m, 1)[0]
	four := bvAddNum(t, m, 4, 8)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBitVectorMultiplication(ast.SliceArguments([]ast.ASTHandle{four, x}), out, true, ex)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	hi, e := ex.Apply(5, 0, x)
	astTestMust(t, e)
	want, e := m.MkBitVectorConcatPair(hi, bvAddNum(t, m, 0, 2))
	astTestMust(t, e)
	astTestPin(t, m, want)
	if st != Rewrite2 || got != want {
		t.Fatal(st, got, want)
	}
}

func TestBVRewriterMultiplicationCallerOriginalGuards(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	xs := bvMergeVars(t, m, 3)
	four := bvAddNum(t, m, 4, 8)
	three := bvAddNum(t, m, 3, 8)
	old := bvAddNum(t, m, 9, 8)
	out, e := m.NewRef(old)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	cases := []struct {
		args    []ast.ASTHandle
		enabled bool
		status  RewriteStatus
	}{
		{[]ast.ASTHandle{four, xs[0]}, false, RewriteFailed},
		{[]ast.ASTHandle{three, xs[0]}, true, RewriteFailed},
		{[]ast.ASTHandle{xs[0], xs[1]}, true, RewriteFailed},
		{[]ast.ASTHandle{xs[0], xs[1], xs[2]}, true, RewriteFailed},
	}
	for i, tc := range cases {
		st, e := p.RewriteBitVectorMultiplication(ast.SliceArguments(tc.args), out, tc.enabled, nil)
		if e != nil {
			t.Fatalf("case %d: %v", i, e)
		}
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if st != tc.status || got != old || p.multiplicationNumerals != nil {
			t.Fatal(st, got, old)
		}
	}
	// Sorting puts the coefficient first, then the DONE branch must borrow the
	// assigned two-factor product rather than inspect the original first factor.
	ex, e := NewMkExtractProc(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ex.Close()) }()
	st, e := p.RewriteBitVectorMultiplication(ast.SliceArguments([]ast.ASTHandle{xs[0], four}), out, true, ex)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	yes, e := m.IsBitVectorOperation(got, 37)
	astTestMust(t, e)
	if st != Rewrite2 || !yes {
		t.Fatal("assigned product not used", st, got)
	}
	// A unit factor is eliminated by the actual core and never reaches shift=0.
	st, e = p.RewriteBitVectorMultiplication(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 1, 8), xs[0]}), out, true, nil)
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteDone || got != xs[0] {
		t.Fatal(st, got)
	}
}

func TestBVRewriterMultiplicationCallerEveryColdReservation(t *testing.T) {
	complete := false
	sawAfterCore := false
	sawExtract := false
	for ordinal := int64(0); ordinal < 256 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			c, g, _ := bvPolynomialParamsFixture(t)
			m, _ := bvAddFixture(t)
			p, e := NewBVPolynomialRewriter(m, c, g, nil)
			astTestMust(t, e)
			ex, e := NewMkExtractProc(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, ex.Close()) }()
			x := bvMergeVars(t, m, 1)[0]
			four := bvAddNum(t, m, 4, 8)
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			args := ast.SliceArguments([]ast.ASTHandle{four, x})
			m.Budget().FailAfter(ordinal)
			st, body := p.RewriteBitVectorMultiplication(args, out, true, ex)
			m.Budget().FailAfter(-1)
			if body == nil {
				complete = true
				if st != Rewrite2 || p.multiplicationNumerals != nil {
					t.Fatal(st)
				}
			} else {
				if !numericHoistRefusal(body) {
					t.Fatal(body)
				}
				if p.multiplication == nil && p.multiplicationNumerals != nil {
					sawAfterCore = true
				}
				if !p.multiplicationConcat[0].IsNull() {
					sawExtract = true
				}
				cause := p.cause
				if _, e = p.RewriteBitVectorMultiplication(args, out, true, ex); e != cause {
					t.Fatal("stopped replay", e, cause)
				}
				astTestMust(t, p.Close())
				if p.cause != cause {
					t.Fatal("changed history")
				}
			}
			astTestMust(t, p.Close())
		})
	}
	// Native coordinate/argument slices no longer create extraction refusal points.
	if !complete || !sawAfterCore {
		t.Fatal(complete, sawAfterCore, sawExtract)
	}
}

func TestBVRewriterMultiplicationCallerNaryCoreResultAndStatusBypass(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	ex, e := NewMkExtractProc(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ex.Close()) }()
	xs := bvMergeVars(t, m, 3)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBitVectorMultiplication(ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 4, 8), xs[0], xs[1]}), out, true, ex)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	product := bvMergeMul(t, m, xs[0], xs[1])
	hi, e := ex.Apply(5, 0, product)
	astTestMust(t, e)
	want, e := m.MkBitVectorConcatPair(hi, bvAddNum(t, m, 0, 2))
	astTestMust(t, e)
	astTestPin(t, m, want)
	if st != Rewrite2 || got != want {
		t.Fatal("nary original must use binary core result", st, got, want)
	}
	local, e := c.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Del(&local)) }()
	key := "som"
	astTestMust(t, c.SetBoolName(&local, &key, true))
	q, e := NewBVPolynomialRewriter(m, c, g, &local)
	astTestMust(t, e)
	defer func() { astTestMust(t, q.Close()) }()
	sum, e := m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments(xs[:2]))
	astTestMust(t, e)
	astTestPin(t, m, sum)
	st, e = q.RewriteBitVectorMultiplication(ast.SliceArguments([]ast.ASTHandle{sum, xs[2]}), out, true, nil)
	astTestMust(t, e)
	if st != Rewrite2 || q.multiplicationNumerals != nil || !q.multiplicationConcat[0].IsNull() {
		t.Fatal("core rewrite depth bypass", st)
	}
}

func TestBVRewriterMultiplicationCallerNativeWideFinalAssignment(t *testing.T) {
	complete := false
	sawPublished := false
	for ordinal := int64(0); ordinal < 256 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			c, g, _ := bvPolynomialParamsFixture(t)
			m, _ := bvAddFixture(t)
			p, e := NewBVPolynomialRewriter(m, c, g, nil)
			astTestMust(t, e)
			x := bvMergeVars(t, m, 1)[0]
			four := bvAddNum(t, m, 4, 8)
			ex, e := NewMkExtractProc(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, ex.Close()) }()
			s, e := m.GetSort(x)
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
			st, body := p.RewriteBitVectorMultiplication(ast.SliceArguments([]ast.ASTHandle{four, x}), out, true, ex)
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
				yes, e := m.IsBitVectorOperation(nativeResult, 37)
				astTestMust(t, e)
				if !yes {
					t.Fatal("native replacement shape", nativeResult)
				}
				if m.IsLiveHandle(old) {
					t.Fatal("native assignment retained the old root")
				}
				if st != Rewrite2 {
					t.Fatal(st)
				}
			} else {
				if !numericHoistRefusal(body) {
					t.Fatal(body)
				}
				if got != old {
					sawPublished = true
					if st != Rewrite2 {
						t.Fatal("consumed assignment status", st)
					}
					yes, e := m.IsBitVectorOperation(got, 37)
					astTestMust(t, e)
					if !yes {
						t.Fatal("published node is not concat")
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
