package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:198-367.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestBVPolynomialNonflatMultiplicationCoefficientsAndNestedProduct(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 2)
	s, e := m.GetSort(xs[0])
	astTestMust(t, e)
	two := bvAddNum(t, m, 2, 8)
	three := bvAddNum(t, m, 3, 8)
	nested := bvMergeMul(t, m, three, xs[0])
	astTestPin(t, m, nested)
	for _, row := range []struct {
		args   []ast.ASTHandle
		status RewriteStatus
		want   ast.ASTHandle
	}{
		{[]ast.ASTHandle{two, three}, RewriteDone, bvAddNum(t, m, 6, 8)},
		{[]ast.ASTHandle{two, xs[0]}, RewriteFailed, xs[1]},
		{[]ast.ASTHandle{two, three, xs[0]}, RewriteDone, bvMergeMul(t, m, bvAddNum(t, m, 6, 8), xs[0])},
		{[]ast.ASTHandle{two, nested}, Rewrite1, bvMergeMul(t, m, bvAddNum(t, m, 6, 8), xs[0])},
		{[]ast.ASTHandle{two, bvMergeMul(t, m, bvAddNum(t, m, 128, 8), xs[0])}, Rewrite1, bvMergeMul(t, m, bvAddNum(t, m, 0, 8), xs[0])},
		{[]ast.ASTHandle{bvAddNum(t, m, 0, 8), xs[0]}, RewriteDone, bvAddNum(t, m, 0, 8)},
		{[]ast.ASTHandle{bvAddNum(t, m, 1, 8), xs[0]}, RewriteDone, xs[0]},
	} {
		astTestPin(t, m, row.want)
		for _, h := range row.args {
			astTestPin(t, m, h)
		}
		out, e := m.NewRef(xs[1])
		astTestMust(t, e)
		p, e := NewBVPolynomialMultiplicationCore(m)
		astTestMust(t, e)
		st, e := p.Rewrite(s, ast.SliceArguments(row.args), out, BVPolynomialMultiplicationOptions{})
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if st != row.status || got != row.want {
			t.Fatal("coefficient branch", st, got, row.status, row.want)
		}
		astTestMust(t, p.Close())
		astTestMust(t, m.DelRef(&out))
	}
}

func TestBVPolynomialNonflatMultiplicationDistributionAndOrdering(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 4)
	s, e := m.GetSort(xs[0])
	astTestMust(t, e)
	two := bvAddNum(t, m, 2, 8)
	sum := bvMulHoistSum(t, m, xs[0], xs[1])
	astTestPin(t, m, sum)
	for _, row := range []struct {
		args   []ast.ASTHandle
		o      BVPolynomialMultiplicationOptions
		status RewriteStatus
		want   ast.ASTHandle
	}{
		{[]ast.ASTHandle{two, sum}, BVPolynomialMultiplicationOptions{}, Rewrite2, bvMulHoistSum(t, m, bvMergeMul(t, m, two, xs[0]), bvMergeMul(t, m, two, xs[1]))},
		{[]ast.ASTHandle{two, sum}, BVPolynomialMultiplicationOptions{HoistCoefficients: true}, RewriteFailed, xs[3]},
		{[]ast.ASTHandle{xs[1], xs[0]}, BVPolynomialMultiplicationOptions{}, RewriteDone, bvMergeMul(t, m, xs[0], xs[1])},
		{[]ast.ASTHandle{xs[0], xs[1]}, BVPolynomialMultiplicationOptions{}, RewriteFailed, xs[3]},
		{[]ast.ASTHandle{two, xs[1], xs[0]}, BVPolynomialMultiplicationOptions{ASTOrder: true}, RewriteDone, bvMergeMul(t, m, two, bvMergeMul(t, m, xs[0], xs[1]))},
		{[]ast.ASTHandle{bvAddNum(t, m, 255, 8), bvAddNum(t, m, 255, 8), xs[0]}, BVPolynomialMultiplicationOptions{}, RewriteDone, xs[0]},
	} {
		astTestPin(t, m, row.want)
		for _, h := range row.args {
			astTestPin(t, m, h)
		}
		out, e := m.NewRef(xs[3])
		astTestMust(t, e)
		p, e := NewBVPolynomialMultiplicationCore(m)
		astTestMust(t, e)
		st, e := p.Rewrite(s, ast.SliceArguments(row.args), out, row.o)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if st != row.status || got != row.want {
			t.Fatal("distribution/ordering", st, row.status, got, row.want)
		}
		astTestMust(t, p.SourceReady())
		astTestMust(t, p.Close())
		astTestMust(t, m.DelRef(&out))
	}
}

// AUDIT.md ast_rewriter_02-F1: mk_nflat_mul_core throws
// rewriter_exception("sum of monomials blowup").
func TestBVPolynomialMonomialBlowupIsRewriterException(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 5)
	s, e := m.GetSort(xs[0])
	astTestMust(t, e)
	left := bvMulHoistSum(t, m, xs[0], xs[1])
	right := bvMulHoistSum(t, m, xs[2], xs[3])
	astTestPin(t, m, left)
	astTestPin(t, m, right)
	out, e := m.NewRef(xs[4])
	astTestMust(t, e)
	p, e := NewBVPolynomialMultiplicationCore(m)
	astTestMust(t, e)
	_, err := p.Rewrite(s, ast.SliceArguments([]ast.ASTHandle{left, right}), out, BVPolynomialMultiplicationOptions{SumOfMonomials: true, MonomialLimit: 2})
	if !IsRewriterException(err) || err.Error() != "sum of monomials blowup" {
		t.Fatal("sum of monomials blowup is not a rewriter exception", err)
	}
	astTestMust(t, p.Close())
	astTestMust(t, m.DelRef(&out))
}

func TestBVPolynomialNonflatMultiplicationCartesianOrderAndLimit(t *testing.T) {
	for _, limit := range []uint32{0, 2, 3, 4} {
		m, _ := bvAddFixture(t)
		xs := bvMergeVars(t, m, 5)
		s, e := m.GetSort(xs[0])
		astTestMust(t, e)
		left := bvMulHoistSum(t, m, xs[0], xs[1])
		right := bvMulHoistSum(t, m, xs[2], xs[3])
		astTestPin(t, m, left)
		astTestPin(t, m, right)
		out, e := m.NewRef(xs[4])
		astTestMust(t, e)
		p, e := NewBVPolynomialMultiplicationCore(m)
		astTestMust(t, e)
		st, body := p.Rewrite(s, ast.SliceArguments([]ast.ASTHandle{left, right}), out, BVPolynomialMultiplicationOptions{SumOfMonomials: true, MonomialLimit: limit})
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if limit < 3 {
			if !errors.Is(body, ErrBVPolynomialMonomialBlowup) || got != xs[4] {
				t.Fatal("blowup publication", limit, body)
			}
			count := uint32(len(p.sum))
			for _, h := range p.sum {
				refs, e := m.GetRefCount(h)
				astTestMust(t, e)
				if refs == 0 {
					t.Fatal("retained Cartesian root lost its reference")
				}
			}
			if count != limit+1 {
				t.Fatal("strict limit frontier", limit, count)
			}
		} else {
			astTestMust(t, body)
			if st != Rewrite2 {
				t.Fatal(st)
			}
			for i, pair := range [][2]int{{0, 2}, {1, 2}, {0, 3}, {1, 3}} {
				h, e := m.GetArg(got, uint32(i))
				astTestMust(t, e)
				if h != bvMergeMul(t, m, xs[pair[0]], xs[pair[1]]) {
					t.Fatal("first coordinate must advance fastest", i)
				}
			}
		}
		ready := p.SourceReady()
		astTestMust(t, p.Close())
		if p.SourceReady() != ready {
			t.Fatal("source history changed")
		}
		astTestMust(t, m.DelRef(&out))
	}
}

func TestBVPolynomialNonflatMultiplicationEveryCartesianOwnerAndFactoryReservation(t *testing.T) {
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 300 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, _ := bvAddFixture(t)
			xs := bvMergeVars(t, m, 24)
			s, e := m.GetSort(xs[0])
			astTestMust(t, e)
			sum := bvMulHoistSum(t, m, xs[0], xs[1])
			astTestPin(t, m, sum)
			args := make([]ast.ASTHandle, 24)
			copy(args, xs)
			args[0] = sum
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			p, e := NewBVPolynomialMultiplicationCore(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, p.Close()) }()
			m.Budget().FailAfter(ordinal)
			st, body := p.Rewrite(s, ast.SliceArguments(args), out, BVPolynomialMultiplicationOptions{SumOfMonomials: true, MonomialLimit: 2})
			m.Budget().FailAfter(-1)
			if body != nil {
				failures++
				if !numericHoistRefusal(body) {
					t.Fatal("unexpected refusal", ordinal, body)
				}
				if _, e = p.Rewrite(s, ast.SliceArguments(args), out, BVPolynomialMultiplicationOptions{}); !errors.Is(e, ast.ErrASTPending) {
					t.Fatal("failed call retried", e)
				}
			} else {
				complete = true
				if st != Rewrite2 {
					t.Fatal(st)
				}
				astTestMust(t, p.SourceReady())
			}
			ready := p.SourceReady()
			astTestMust(t, p.Close())
			if p.SourceReady() != ready {
				t.Fatal("owning cleanup changed history")
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("Cartesian horizon", complete, failures)
	}
}

func TestBVPolynomialNonflatMultiplicationEveryActualCoefficientReservation(t *testing.T) {
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 200 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			budget := arena.NewBudget(1 << 27)
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error { return f }})
			astTestMust(t, e)
			q, e := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
			astTestMust(t, e)
			sy, e := usymbol.NewManager(usymbol.Config{Budget: budget})
			astTestMust(t, e)
			m, e := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: budget})
			astTestMust(t, e)
			t.Cleanup(func() {
				astTestMust(t, m.Close())
				astTestMust(t, sy.Close())
				astTestMust(t, q.CloseChecked())
				astTestMust(t, rt.Close())
				if budget.Used() != 0 {
					t.Fatal("actual owner graph retained", budget.Used())
				}
			})
			_, e = m.RegisterArithmeticPlugin()
			astTestMust(t, e)
			_, e = m.RegisterBitVectorPlugin()
			astTestMust(t, e)
			s, e := m.BitVectorSort(65)
			astTestMust(t, e)
			x, e := m.MkVar(0, s)
			astTestMust(t, e)
			astTestPin(t, m, x)
			v, e := q.NewValue()
			astTestMust(t, e)
			astTestMust(t, q.SetString(&v, "9223372036854775809"))
			c, e := m.BitVectorNumeral(&v, 65)
			astTestMust(t, e)
			astTestPin(t, m, c)
			astTestMust(t, q.Del(&v))
			args := ast.SliceArguments([]ast.ASTHandle{c, c, x})
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			p, e := NewBVPolynomialMultiplicationCore(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, p.Close()) }()
			budget.FailAfter(ordinal)
			st, body := p.Rewrite(s, args, out, BVPolynomialMultiplicationOptions{})
			budget.FailAfter(-1)
			if body != nil {
				failures++
				if !numericHoistRefusal(body) {
					t.Fatal("actual refusal", ordinal, body)
				}
				if _, e = p.Rewrite(s, args, out, BVPolynomialMultiplicationOptions{}); !errors.Is(e, ast.ErrASTPending) {
					t.Fatal("failed call retried", e)
				}
			} else {
				complete = true
				if st != RewriteDone {
					t.Fatal(st)
				}
				got, e := m.RefGet(out)
				astTestMust(t, e)
				literal, e := m.GetArg(got, 0)
				astTestMust(t, e)
				value, e := q.NewValue()
				astTestMust(t, e)
				var width uint32
				yes, e := m.BitVectorNumeralValue(literal, &value, &width)
				astTestMust(t, e)
				text, e := q.String(&value)
				astTestMust(t, e)
				astTestMust(t, q.Del(&value))
				if !yes || width != 65 || text != "18446744073709551617" {
					t.Fatal("normalized product coefficient", yes, width, text)
				}
			}
			ready := p.SourceReady()
			astTestMust(t, p.Close())
			if p.SourceReady() != ready {
				t.Fatal("owning cleanup changed source history")
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("actual coefficient horizon", complete, failures)
	}
}

func TestBVPolynomialNonflatMultiplicationCheapGuardAndHeaderRefusal(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 2)
	out, e := m.NewRef(xs[1])
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	p, e := NewBVPolynomialMultiplicationCore(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	st, e := p.Rewrite(ast.ASTHandle{}, ast.SliceArguments([]ast.ASTHandle{bvAddNum(t, m, 2, 8), xs[0]}), out, BVPolynomialMultiplicationOptions{})
	astTestMust(t, e)
	if st != RewriteFailed {
		t.Fatal("cheap guard read invalid current sort", st)
	}
	astTestMust(t, p.SourceReady())
	copied := *p
	if e = copied.Close(); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal("copied owner accepted", e)
	}
	before := m.Budget().Used()
	m.Budget().FailAfter(0)
	other, e := NewBVPolynomialMultiplicationCore(m)
	m.Budget().FailAfter(-1)
	if other != nil || !errors.Is(e, arena.ErrInjectedFailure) || m.Budget().Used() != before {
		t.Fatal("header refusal", e)
	}
}

func TestBVPolynomialNonflatMultiplicationIntermediateReferencePublication(t *testing.T) {
	complete, intermediateFailures := false, 0
	for ordinal := int64(0); ordinal < 200 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, _ := bvAddFixture(t)
			xs := bvMergeVars(t, m, 2)
			s, e := m.GetSort(xs[0])
			astTestMust(t, e)
			coefficient := bvAddNum(t, m, 2, 8)
			out, e := m.NewRef(ast.ASTHandle{})
			astTestMust(t, e)
			defer func() { astTestMust(t, m.DelRef(&out)) }()
			p, e := NewBVPolynomialMultiplicationCore(m)
			astTestMust(t, e)
			defer func() { astTestMust(t, p.Close()) }()
			m.Budget().FailAfter(ordinal)
			st, body := p.Rewrite(s, ast.SliceArguments([]ast.ASTHandle{coefficient, xs[0], xs[1]}), out, BVPolynomialMultiplicationOptions{})
			m.Budget().FailAfter(-1)
			got, e := m.RefGet(out)
			astTestMust(t, e)
			if body != nil {
				if !numericHoistRefusal(body) {
					t.Fatal("unexpected refusal", body)
				}
				if !got.IsNull() {
					intermediateFailures++
					first, e := m.GetArg(got, 0)
					astTestMust(t, e)
					second, e := m.GetArg(got, 1)
					astTestMust(t, e)
					if first != xs[0] || second != xs[1] {
						t.Fatal("intermediate receiver lost", first, second)
					}
				}
			} else {
				complete = true
				if st != RewriteDone {
					t.Fatal(st)
				}
				base := bvMergeMul(t, m, xs[0], xs[1])
				want := bvMergeMul(t, m, coefficient, base)
				if got != want {
					t.Fatal("coefficient wrapper result")
				}
			}
			astTestMust(t, p.Close())
		})
	}
	if !complete || intermediateFailures == 0 {
		t.Fatal("intermediate publication frontier", complete, intermediateFailures)
	}
}

func TestBVPolynomialMultiplicationNativeSortingWithoutArenaReservations(t *testing.T) {
	m, _ := bvAddFixture(t)
	xs := bvMergeVars(t, m, 257)
	currentSort, e := m.GetSort(xs[0])
	astTestMust(t, e)
	family, e := m.GetFamilyID(currentSort)
	astTestMust(t, e)
	p, e := NewBVPolynomialMultiplicationCore(m)
	astTestMust(t, e)
	defer func() { astTestMust(t, p.Close()) }()
	before := m.Budget().Used()
	for _, astOrder := range []bool{false, true} {
		m.Budget().FailAfter(0)
		for i := len(xs) - 1; i >= 0; i-- {
			p.terms = append(p.terms, xs[i])
		}
		e = p.sort(family, BVPolynomialMultiplicationOptions{ASTOrder: astOrder})
		m.Budget().FailAfter(-1)
		astTestMust(t, e)
		for i, h := range p.terms {
			if h != xs[i] {
				t.Fatal("native factor ordering", astOrder, i)
			}
		}
		if m.Budget().Used() != before {
			t.Fatal("native sorting changed arena charge")
		}
		p.terms = nil
	}
}
