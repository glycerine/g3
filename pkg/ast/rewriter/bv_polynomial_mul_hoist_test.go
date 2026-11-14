package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:903-943.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func bvMulHoistSum(t *testing.T, m *ast.ASTManager, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, e := m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, ast.SliceArguments(args))
	astTestMust(t, e)
	return h
}

func TestBVPolynomialMultiplicationHoistDisabledAndNoChange(t *testing.T) {
	var none *BVPolynomialMultiplicationHoist
	changed, e := none.Hoist(false, ast.ASTRef{})
	if changed || e != nil {
		t.Fatal("disabled branch read state", e)
	}
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 3)
	for _, sum := range []ast.ASTHandle{xs[0], bvMulHoistSum(t, m, xs[0], xs[1]), bvMulHoistSum(t, m, bvMergeMul(t, m, xs[0], xs[0]), xs[1])} {
		out, e := m.NewRef(sum)
		astTestMust(t, e)
		p, e := NewBVPolynomialMultiplicationHoist(m)
		astTestMust(t, e)
		changed, e := p.Hoist(true, out)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if changed || got != sum {
			t.Fatal("no-common output changed")
		}
		astTestMust(t, p.Close())
		astTestMust(t, m.DelRef(&out))
	}
}

func TestBVPolynomialMultiplicationHoistValidityAndZeroPlaceholders(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 6)
	// Only one merge per participating term: the third term sharing x0 must
	// remain unchanged because the map's prior owner has become invalid.
	a := bvMergeMul(t, m, xs[0], xs[1])
	b := bvMergeMul(t, m, xs[0], xs[2])
	c := bvMergeMul(t, m, xs[0], xs[3])
	d := bvMergeMul(t, m, xs[4], xs[5])
	sum := bvMulHoistSum(t, m, a, b, c, d)
	out, e := m.NewRef(sum)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	p, e := NewBVPolynomialMultiplicationHoist(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	changed, e := p.Hoist(true, out)
	astTestMust(t, e)
	if !changed {
		t.Fatal("shared factor not hoisted")
	}
	got, e := m.RefGet(out)
	astTestMust(t, e)
	n, e := m.GetNumArgs(got)
	astTestMust(t, e)
	if n != 4 {
		t.Fatal("source zero placeholder was removed", n)
	}
	zero, e := m.GetArg(got, 1)
	astTestMust(t, e)
	yes, e := m.IsBitVectorOperation(zero, 0)
	astTestMust(t, e)
	if !yes {
		t.Fatal("missing zero placeholder")
	}
	for i, want := range []ast.ASTHandle{c, d} {
		h, e := m.GetArg(got, uint32(i+2))
		astTestMust(t, e)
		if h != want {
			t.Fatal("invalid map entry caused another merge")
		}
	}
	astTestMust(t, p.SourceReady())
}

func TestBVPolynomialMultiplicationHoistSoleInputOwnershipAndGrowth(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 40)
	terms := make([]ast.ASTHandle, 40)
	for i := range terms {
		terms[i] = bvMulHoistSum(t, m, xs[i], xs[i])
	}
	// Flatten ADD at the head follows the source macro's append-at-end order.
	sum := bvMulHoistSum(t, m, terms...)
	out, e := m.NewRef(sum)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	p, e := NewBVPolynomialMultiplicationHoist(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	changed, e := p.Hoist(true, out)
	astTestMust(t, e)
	if !changed {
		t.Fatal("long sum not hoisted")
	}
	if m.IsLiveHandle(sum) {
		t.Fatal("original sum retained after owning replacement")
	}
	got, e := m.RefGet(out)
	astTestMust(t, e)
	n, e := m.GetNumArgs(got)
	astTestMust(t, e)
	if n != 80 {
		t.Fatal("flattened source arity", n)
	}
}

func TestBVPolynomialMultiplicationHoistEveryColdReservation(t *testing.T) {
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 200 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, _ := bvAddFixture(t)
			xs := bvMergeVars(t, m, 20)
			terms := make([]ast.ASTHandle, 20)
			for i := range terms {
				terms[i] = bvMergeMul(t, m, xs[0], xs[i])
			}
			sum := bvMulHoistSum(t, m, terms...)
			astTestPin(t, m, sum)
			out, e := m.NewRef(sum)
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			p, e := NewBVPolynomialMultiplicationHoist(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, p.Close()) }()
			m.Budget().FailAfter(ordinal)
			changed, body := p.Hoist(true, out)
			m.Budget().FailAfter(-1)
			if body != nil {
				failures++
				if !errors.Is(body, arena.ErrInjectedFailure) {
					var f umemory.Failure
					if !errors.As(body, &f) || f.Kind != umemory.FailureMemout {
						t.Fatal("unexpected refusal", body)
					}
				}
				if _, e = p.Hoist(true, out); !errors.Is(e, ast.ErrASTPending) {
					t.Fatal("failed call retried", e)
				}
			} else {
				complete = true
				if !changed {
					t.Fatal("no hoist at horizon")
				}
			}
			ready := p.SourceReady()
			astTestMust(t, p.Close())
			if p.SourceReady() != ready {
				t.Fatal("source history changed")
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("hoist horizon", complete, failures)
	}
}
