package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2279-2304.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVRewriterBlastEqualityOriginalBitOrder(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	ex, e := NewMkExtractProc(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ex.Close()) }()
	xs := bvMergeVars(t, m, 2)
	lhs, e := m.MkBitVectorOr(xs)
	astTestMust(t, e)
	astTestPin(t, m, lhs)
	rhs := bvAddNum(t, m, 165, 8)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBlastEqualityValue(lhs, rhs, out, ex)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	terms := make([]ast.ASTHandle, 8)
	for i := uint32(0); i < 8; i++ {
		bit, e := ex.Apply(i, i, lhs)
		astTestMust(t, e)
		terms[i], e = m.MkEq(bit, bvAddNum(t, m, (165>>i)&1, 1))
		astTestMust(t, e)
		astTestPin(t, m, terms[i])
	}
	want, e := m.MkAndArgs(terms)
	astTestMust(t, e)
	astTestPin(t, m, want)
	if st != Rewrite3 || got != want {
		t.Fatal(st, got, want)
	}
}

func TestBVRewriterBlastEqualityGuardsSwapAndRHSWidth(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	ex, e := NewMkExtractProc(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ex.Close()) }()
	xs := bvMergeVars(t, m, 2)
	or, e := m.MkBitVectorOr(xs)
	astTestMust(t, e)
	astTestPin(t, m, or)
	xor, e := m.MkBitVectorXor(xs)
	astTestMust(t, e)
	astTestPin(t, m, xor)
	not, e := m.MkBitVectorNot(xs[0])
	astTestMust(t, e)
	astTestPin(t, m, not)
	old, e := m.MkTrue()
	astTestMust(t, e)
	astTestPin(t, m, old)
	out, e := m.NewRef(old)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBlastEqualityValue(bvAddNum(t, m, 1, 1), ast.ASTHandle{}, out, nil)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != RewriteFailed || got != old || p.blastEquality != nil {
		t.Fatal(st, got)
	}
	for _, pair := range [][2]ast.ASTHandle{{xs[0], xs[1]}, {xs[0], bvAddNum(t, m, 9, 8)}} {
		st, e = p.RewriteBlastEqualityValue(pair[0], pair[1], out, nil)
		astTestMust(t, e)
		got, e = m.RefGet(out)
		astTestMust(t, e)
		if st != RewriteFailed || got != old {
			t.Fatal(st, got)
		}
	}
	for _, lhs := range []ast.ASTHandle{or, xor, not} {
		for _, swapped := range []bool{false, true} {
			rhs := bvAddNum(t, m, 165, 8)
			a, b := lhs, rhs
			if swapped {
				a, b = b, a
			}
			st, e = p.RewriteBlastEqualityValue(a, b, out, ex)
			astTestMust(t, e)
			got, e = m.RefGet(out)
			astTestMust(t, e)
			terms := make([]ast.ASTHandle, 8)
			for i := uint32(0); i < 8; i++ {
				h, e := ex.Apply(i, i, lhs)
				astTestMust(t, e)
				terms[i], e = m.MkEq(h, bvAddNum(t, m, (165>>i)&1, 1))
				astTestMust(t, e)
				astTestPin(t, m, terms[i])
			}
			want, e := m.MkAndArgs(terms)
			astTestMust(t, e)
			astTestPin(t, m, want)
			if st != Rewrite3 || got != want {
				t.Fatal(st, swapped, got, want)
			}
		}
	}
	// The copied numeral overwrites sz; no new equal-width validation is added.
	st, e = p.RewriteBlastEqualityValue(or, bvAddNum(t, m, 5, 4), out, ex)
	astTestMust(t, e)
	got, e = m.RefGet(out)
	astTestMust(t, e)
	count, e := m.GetNumArgs(got)
	astTestMust(t, e)
	if st != Rewrite3 || count != 4 {
		t.Fatal("rhs width not published", st, count)
	}
}

func TestBVRewriterBlastEqualityEveryColdReservation(t *testing.T) {
	complete := false
	sawLongArguments := false
	sawAssignment := false
	for ordinal := int64(0); ordinal < 2048 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			c, g, _ := bvPolynomialParamsFixture(t)
			m, _ := bvAddFixture(t)
			p, e := NewBVPolynomialRewriter(m, c, g, nil)
			astTestMust(t, e)
			ex, e := NewMkExtractProc(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, ex.Close()) }()
			s, e := m.BitVectorSort(40)
			astTestMust(t, e)
			x, e := m.MkVar(0, s)
			astTestMust(t, e)
			astTestPin(t, m, x)
			lhs, e := m.MkBitVectorNot(x)
			astTestMust(t, e)
			astTestPin(t, m, lhs)
			rhs := bvAddNum(t, m, 0xdeadbeef, 40)
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
			st, body := p.RewriteBlastEqualityValue(lhs, rhs, out, ex)
			m.Budget().FailAfter(-1)
			if body == nil {
				complete = true
				nativeResult, nativeErr := m.RefGet(out)
				astTestMust(t, nativeErr)
				if nativeResult != old {
					sawAssignment = true
				}
				if st != Rewrite3 || p.blastEquality != nil {
					t.Fatal(st)
				}
			} else {
				if !numericHoistRefusal(body) {
					t.Fatal(body)
				}
				if p.blastEquality != nil && len(p.blastEquality.args) > 16 {
					sawLongArguments = true
				}
				got, e := m.RefGet(out)
				astTestMust(t, e)
				if got != old {
					sawAssignment = true
					if st != Rewrite3 {
						t.Fatal(st)
					}
				}
				cause := p.cause
				if _, e = p.RewriteBlastEqualityValue(lhs, rhs, out, ex); e != cause {
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
	if !complete || !sawLongArguments || !sawAssignment {
		t.Fatal(complete, sawLongArguments, sawAssignment)
	}
}

// AUDIT.md ast_rewriter_02-F3: GCC evaluates mk_eq(m_mk_extract(i, i, lhs),
// mk_numeral(bit, 1)) right to left, so the bit numeral is created before
// the extract.
func TestBVRewriterBlastEqualityNumeralBeforeExtract(t *testing.T) {
	c, g, _ := bvPolynomialParamsFixture(t)
	m, _ := bvAddFixture(t)
	p, e := NewBVPolynomialRewriter(m, c, g, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	ex, e := NewMkExtractProc(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, ex.Close()) }()
	xs := bvMergeVars(t, m, 2)
	lhs, e := m.MkBitVectorOr(xs)
	astTestMust(t, e)
	astTestPin(t, m, lhs)
	rhs := bvAddNum(t, m, 1, 8)
	out, e := m.NewRef(ast.ASTHandle{})
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	st, e := p.RewriteBlastEqualityValue(lhs, rhs, out, ex)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if st != Rewrite3 {
		t.Fatal("blast status", st)
	}
	first, e := m.GetArg(got, 0)
	astTestMust(t, e)
	bit, e := m.GetArg(first, 0)
	astTestMust(t, e)
	value, e := m.GetArg(first, 1)
	astTestMust(t, e)
	bitID, e := m.GetASTID(bit)
	astTestMust(t, e)
	valueID, e := m.GetASTID(value)
	astTestMust(t, e)
	if valueID > bitID {
		t.Fatal("bit numeral created after the extract", valueID, bitID)
	}
}
