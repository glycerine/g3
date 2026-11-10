package lp

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

const larTermMultilimbRepeats = 100

// lpTermOrder replays the coefficient insertions and erasures on a
// StdUnorderedRationalMap, whose libstdc++ iteration order is pinned by
// std_unordered_map_test against the C++ control vector.
type lpTermOrder struct{ keys *StdUnorderedRationalMap }

var lpTermOrderValue = new(numeral.Rational)

func newLPTermOrder() *lpTermOrder { return &lpTermOrder{keys: NewStdUnorderedRationalMap()} }

func (o *lpTermOrder) Keys() []uint32 {
	if o == nil {
		return nil
	}
	return o.keys.Keys()
}

func lpCheckLARTerm(t *testing.T, h *lpPairHarness, term *LARTerm, want map[uint32]*big.Rat, order []uint32) {
	t.Helper()
	if int(term.Size()) != len(want) {
		t.Fatal("term size", term.Size(), len(want))
	}
	var got []uint32
	for j, value := range term.Coefficients().All() {
		got = append(got, j)
		x := want[j]
		if x == nil {
			t.Fatal("unexpected coefficient", j)
		}
		lpSparseRationalCheck(t, h, value, x)
	}
	if !slices.Equal(got, order) {
		t.Fatal("term order", got, order)
	}
	for j := uint32(0); j < 13; j++ {
		if term.Contains(j) != (want[j] != nil) {
			t.Fatal("contains", j)
		}
	}
}
func lpTermOracleErase(want map[uint32]*big.Rat, order *lpTermOrder, j uint32) *lpTermOrder {
	delete(want, j)
	order.keys.Del(j)
	return order
}
func lpTermOracleAdd(want map[uint32]*big.Rat, order *lpTermOrder, j uint32, x *big.Rat) *lpTermOrder {
	if old := want[j]; old != nil {
		old.Add(old, x)
		if old.Sign() == 0 {
			return lpTermOracleErase(want, order, j)
		}
		return order
	}
	want[j] = new(big.Rat).Set(x)
	order.keys.Put(j, lpTermOrderValue)
	return order
}
func TestLARTermIndependentArithmeticAndOrderOracle(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	h := newLPPairHarness(t)
	p := h.pair("0", "17/19")
	term := NewLARTerm(h.m)
	v := NewIndexedRationalVector(h.m, 12)
	t.Cleanup(func() { h.must(term.Close()); h.must(v.Close()) })
	h.must(term.SetValue(&p.Y))
	want := make(map[uint32]*big.Rat)
	order := newLPTermOrder()
	r := rand.New(rand.NewPCG(7913, 159))
	for step := 0; step < 1600; step++ {
		j := uint32(r.IntN(12))
		if r.IntN(4) != 0 {
			x := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(int64(r.IntN(19)-9)), uint(r.IntN(120))), big.NewInt(int64(r.IntN(29)+1)))
			h.must(h.m.SetString(&p.X, x.RatString()))
			h.must(term.AddCoefficient(j, &p.X))
			order = lpTermOracleAdd(want, order, j, x)
		} else {
			h.must(v.Resize(12))
			for _, k := range r.Perm(12) {
				if uint32(k) == j || r.IntN(4) != 0 {
					continue
				}
				x := big.NewRat(int64(r.IntN(11)-5), int64(r.IntN(17)+1))
				h.must(h.m.SetString(&p.X, x.RatString()))
				h.must(v.SetValue(&p.X, uint32(k)))
			}
			if b := want[j]; b != nil {
				pivot := new(big.Rat).Set(b)
				for _, k := range v.Index {
					text, e := h.m.String(&v.Data[k])
					h.must(e)
					x, _ := new(big.Rat).SetString(text)
					delta := new(big.Rat).Neg(new(big.Rat).Mul(pivot, x))
					order = lpTermOracleAdd(want, order, k, delta)
				}
				order = lpTermOracleErase(want, order, j)
			}
			h.must(term.Substitute(j, v))
		}
		lpCheckLARTerm(t, h, term, want, order.Keys())
		lpSparseRationalCheck(t, h, &term.Value, big.NewRat(17, 19))
	}
	out, e := term.CoefficientsAsVector()
	h.must(e)
	if int(out.Size()) != order.keys.Len() {
		t.Fatal("export size")
	}
	for i, j := range order.Keys() {
		if out.Data[i].Index != j {
			t.Fatal("export order")
		}
		lpSparseRationalCheck(t, h, &out.Data[i].Value, want[j])
	}
	if len(out.Data) > 0 {
		h.must(h.m.SetInt64(&out.Data[0].Value, 999))
		lpCheckLARTerm(t, h, term, want, order.Keys())
	}
	h.must(out.Close())
	h.must(term.Close())
	h.must(term.Close())
	h.must(v.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARTermLiteralZeroAndLivePivotPrefixes(t *testing.T) {
	for _, mode := range []string{"live", "erased-midway", "erased-final", "bounds", "empty"} {
		t.Run(mode, func(t *testing.T) {
			h := newLPPairHarness(t)
			p := h.pair("2", "3")
			term := NewLARTerm(h.m)
			v := NewIndexedRationalVector(h.m, 2)
			t.Cleanup(func() { h.must(term.Close()); h.must(v.Close()) })
			h.must(term.AddCoefficient(0, &p.X))
			h.must(term.AddCoefficient(1, &p.Y))
			h.must(h.m.SetString(&v.Data[0], "1/2"))
			h.must(h.m.SetInt64(&v.Data[1], 4))
			v.Index = []uint32{0, 1}
			var wantErr error
			want := map[uint32]*big.Rat{1: big.NewRat(-1, 1)}
			order := []uint32{1}
			switch mode {
			case "erased-midway":
				h.must(h.m.SetInt64(&v.Data[0], 1))
				wantErr = ErrLARTermPivotErased
				want[1] = big.NewRat(3, 1)
			case "erased-final":
				h.must(h.m.SetInt64(&v.Data[0], 1))
				v.Index = []uint32{0}
				wantErr = ErrLARTermPivotErased
				want[1] = big.NewRat(3, 1)
			case "bounds":
				v.Index = []uint32{1, 7}
				wantErr = ErrBounds
				want[0] = big.NewRat(2, 1)
				want[1] = big.NewRat(-5, 1)
				order = []uint32{1, 0}
			case "empty":
				v.Index = nil
				want[1] = big.NewRat(3, 1)
			}
			if e := term.Substitute(0, v); e != wantErr {
				t.Fatal(e, wantErr)
			}
			lpCheckLARTerm(t, h, term, want, order)
			h.must(term.Close())
			h.must(v.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
	t.Run("new zero, exact cancellation and reinsertion", func(t *testing.T) {
		h := newLPPairHarness(t)
		p := h.pair("0", "2")
		term := NewLARTerm(h.m)
		h.must(term.AddCoefficient(4, &p.X))
		h.must(term.AddCoefficient(1, &p.Y))
		h.must(term.AddCoefficient(4, &p.X))
		h.must(term.AddCoefficient(4, &p.Y))
		lpCheckLARTerm(t, h, term, map[uint32]*big.Rat{1: big.NewRat(2, 1), 4: big.NewRat(2, 1)}, []uint32{4, 1})
		// Existing self-reference remains a live scalar, not a copied map value.
		h.must(term.AddCoefficient(1, term.Coefficients().Get(1)))
		lpSparseRationalCheck(t, h, term.Coefficients().Get(1), big.NewRat(4, 1))
		h.must(term.Close())
		lpExactVectorNamedCensus(t, h, nil)
	})
}

func TestLARTermAllocationFailureCensus(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, op := range []string{"constant", "insert", "increase", "cancel", "constructor", "sub-new", "sub-existing", "export"} {
		failures, successes := 0, 0
		for ordinal := int64(0); ordinal < 180; ordinal++ {
			done := false
			t.Run(fmt.Sprintf("%s/%d", op, ordinal), func(t *testing.T) {
				h := newLPPairHarness(t)
				xs := strings.Repeat("1234567890", larTermMultilimbRepeats) + "/7"
				ys := strings.Repeat("9876543210", larTermMultilimbRepeats) + "/13"
				p := h.pairExactLarge(xs, ys)
				negative := h.pair("0", "0")
				h.must(h.m.Copy(&negative.X, &p.X))
				h.must(h.m.Neg(&negative.X))
				term := NewLARTerm(h.m)
				v := NewIndexedRationalVector(h.m, 2)
				input := NewSparseRationalVector(h.m)
				var exported *SparseRationalVector
				t.Cleanup(func() {
					h.budget.FailAfter(-1)
					if exported != nil {
						h.must(exported.Close())
					}
					h.must(term.Close())
					h.must(v.Close())
					h.must(input.Close())
				})
				if op == "increase" || op == "cancel" || op == "sub-new" || op == "sub-existing" || op == "export" {
					h.must(term.AddCoefficient(0, &p.X))
				}
				if op == "sub-existing" {
					h.must(term.AddCoefficient(1, &p.Y))
				}
				h.must(v.SetValue(&p.Y, 1))
				h.must(input.Append(3, &p.X))
				h.must(input.Append(3, &p.Y))
				x, _ := new(big.Rat).SetString(xs)
				y, _ := new(big.Rat).SetString(ys)
				h.budget.FailAfter(ordinal)
				var err error
				switch op {
				case "constant":
					err = term.SetValue(&p.X)
				case "insert":
					err = term.AddCoefficient(0, &p.X)
				case "increase":
					err = term.AddCoefficient(0, &p.Y)
				case "cancel":
					err = term.AddCoefficient(0, &negative.X)
				case "constructor":
					h.must(term.Close())
					term, err = NewLARTermFromCoefficients(h.m, input.Data, &p.X)
				case "sub-new", "sub-existing":
					err = term.Substitute(0, v)
				case "export":
					exported, err = term.CoefficientsAsVector()
				}
				h.budget.FailAfter(-1)
				if err == nil {
					successes++
					done = true
					switch op {
					case "constant":
						lpSparseRationalCheck(t, h, &term.Value, x)
					case "insert":
						lpSparseRationalCheck(t, h, term.Coefficients().Get(0), x)
					case "increase":
						lpSparseRationalCheck(t, h, term.Coefficients().Get(0), new(big.Rat).Add(x, y))
					case "cancel":
						if term.Size() != 0 {
							t.Fatal("cancellation")
						}
					case "constructor":
						lpSparseRationalCheck(t, h, &term.Value, x)
						lpSparseRationalCheck(t, h, term.Coefficients().Get(3), new(big.Rat).Add(x, y))
					case "sub-new", "sub-existing":
						want := new(big.Rat).Neg(new(big.Rat).Mul(x, y))
						if op == "sub-existing" {
							want.Add(want, y)
						}
						if term.Contains(0) {
							t.Fatal("pivot retained")
						}
						lpSparseRationalCheck(t, h, term.Coefficients().Get(1), want)
					case "export":
						if exported.Size() != 1 {
							t.Fatal(exported.Size())
						}
						lpSparseRationalCheck(t, h, &exported.Data[0].Value, x)
					}
				} else {
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					failures++
					if op == "export" {
						if exported == nil || term.ready() != nil {
							t.Fatal("export ownership")
						}
					} else {
						if term.ready() != ErrExactVectorPending {
							t.Fatal("lost stopped term")
						}
						if e := term.SetValue(&p.X); e != ErrExactVectorPending {
							t.Fatal(e)
						}
						if e := term.AddCoefficient(7, &p.X); e != ErrExactVectorPending {
							t.Fatal(e)
						}
						if e := term.Substitute(0, v); e != ErrExactVectorPending {
							t.Fatal(e)
						}
						if _, e := term.CoefficientsAsVector(); e != ErrExactVectorPending {
							t.Fatal(e)
						}
						if _, e := NewLARTermIterator(term, 7); e != ErrExactVectorPending {
							t.Fatal(e)
						}
					}
				}
				if exported != nil {
					h.must(exported.Close())
				}
				h.must(term.Close())
				h.must(v.Close())
				h.must(input.Close())
				lpExactVectorNamedCensus(t, h, nil)
				if h.m.RawInvocationState().Active != [3]bool{} || h.m.DivisionInvocationState().Active {
					t.Fatal("orphan numeric frame")
				}
			})
			if done {
				break
			}
		}
		if failures == 0 || successes != 1 {
			t.Fatal("incomplete campaign", op, failures, successes)
		}
	}
}

func TestLARTermIteratorLiteralCursorAndCopyFailure(t *testing.T) {
	for ordinal := int64(0); ordinal < 60; ordinal++ {
		done := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			text := strings.Repeat("1234567890", larTermMultilimbRepeats) + "/7"
			p := h.pairExactLarge(text, "0")
			out := h.pair("0", "0")
			term := NewLARTerm(h.m)
			h.must(term.AddCoefficient(4, &p.X))
			it, e := NewLARTermIterator(term, 9)
			h.must(e)
			t.Cleanup(func() { h.budget.FailAfter(-1); h.must(it.Close()); h.must(term.Close()) })
			j, ok, e := it.Next(&out.X)
			h.must(e)
			if j != 9 || !ok || it.Size() != 2 {
				t.Fatal("basis")
			}
			lpSparseRationalCheck(t, h, &out.X, big.NewRat(-1, 1))
			h.budget.FailAfter(ordinal)
			j, ok, e = it.Next(&out.X)
			h.budget.FailAfter(-1)
			if e != nil {
				if !lpExactPriorityPhysicalRefusal(e) || j != 4 || ok || it.offset != 0 {
					t.Fatal("copy prefix", j, ok, e, it.offset)
				}
				if term.ready() != nil {
					t.Fatal("term adopted iterator call")
				}
				if it.calls.pending() {
					if e := it.Reset(); e != ErrExactVectorPending {
						t.Fatal(e)
					}
					if _, _, e := it.Next(&out.X); e != ErrExactVectorPending {
						t.Fatal(e)
					}
					if _, _, e := it.NextIndex(); e != ErrExactVectorPending {
						t.Fatal(e)
					}
				}
			} else {
				done = true
				want, _ := new(big.Rat).SetString(text)
				lpSparseRationalCheck(t, h, &out.X, want)
				if !ok || j != 4 || it.offset != 1 {
					t.Fatal("completed cursor")
				}
				for k := 0; k < 3; k++ {
					_, ok, e = it.Next(&out.X)
					h.must(e)
					if ok {
						t.Fatal("exhaustion")
					}
				}
				h.must(it.Reset())
				j, ok, e = it.NextIndex()
				h.must(e)
				if !ok || j != 9 {
					t.Fatal("reset basis")
				}
				j, ok, e = it.NextIndex()
				h.must(e)
				if !ok || j != 4 {
					t.Fatal("index-only term")
				}
				_, ok, e = it.NextIndex()
				h.must(e)
				if ok {
					t.Fatal("index-only end")
				}
			}
			h.must(it.Close())
			h.must(term.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
		if done {
			return
		}
	}
	t.Fatal("iterator copy never succeeded")
}

func TestLARTermBorrowedVectorFailureOwnership(t *testing.T) {
	matched := false
	for ordinal := int64(0); ordinal < 50 && !matched; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			h := newLPPairHarness(t)
			p := h.pairExactLarge(strings.Repeat("1234567890", larTermMultilimbRepeats)+"/7", strings.Repeat("9876543210", larTermMultilimbRepeats)+"/13")
			term := NewLARTerm(h.m)
			v := NewIndexedRationalVector(h.m, 1)
			h.must(term.AddCoefficient(0, &p.X))
			h.must(v.SetValue(&p.X, 0))
			t.Cleanup(func() { h.budget.FailAfter(-1); h.must(term.Close()); h.must(v.Close()) })
			h.budget.FailAfter(ordinal)
			err := v.Add(0, &p.Y)
			h.budget.FailAfter(-1)
			if v.calls.pending() {
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				matched = true
				before := v.calls
				if e := term.Substitute(0, v); e != ErrExactVectorPending {
					t.Fatal(e)
				}
				h.must(term.Close())
				if !reflect.DeepEqual(before, v.calls) {
					t.Fatal("term consumed borrowed vector receipts")
				}
			}
			h.must(term.Close())
			h.must(v.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
	if !matched {
		t.Fatal("no actual stopped vector call")
	}
	t.Run("foreign vector", func(t *testing.T) {
		h := newLPPairHarness(t)
		other := newLPPairHarness(t)
		term := NewLARTerm(h.m)
		v := NewIndexedRationalVector(other.m, 0)
		if e := term.Substitute(0, v); e != ErrExactVectorOwner {
			t.Fatal(e)
		}
		h.must(term.Close())
		other.must(v.Close())
	})
}

func TestLARTermIteratorLiveValuesAndPendingGuards(t *testing.T) {
	h := newLPPairHarness(t)
	p := h.pair("2", "-5")
	out := h.pair("0", "0")
	term := NewLARTerm(h.m)
	h.must(term.AddCoefficient(4, &p.X))
	it, e := NewLARTermIterator(term, 9)
	h.must(e)
	t.Cleanup(func() { h.must(it.Close()); h.must(term.Close()) })
	_, _, e = it.NextIndex()
	h.must(e)
	h.must(term.AddCoefficient(4, &p.X))
	j, ok, e := it.Next(&out.X)
	h.must(e)
	if j != 4 || !ok {
		t.Fatal(j, ok)
	}
	lpSparseRationalCheck(t, h, &out.X, big.NewRat(4, 1))
	h.must(it.Reset())
	_, _, e = it.NextIndex()
	h.must(e)
	h.must(h.m.SetInt64(&p.X, -4))
	h.must(term.AddCoefficient(4, &p.X))
	if j, ok, e = it.Next(&out.X); j != 4 || ok || e != ErrBounds {
		t.Fatal("erased borrow", j, ok, e)
	}
	h.must(it.Reset())
	if it.Size() != 1 {
		t.Fatal(it.Size())
	}
	// A real stopped term call prevents its borrowed iterator from reading
	// partial numerical state, without transferring the term's owner receipt.
	v := NewIndexedRationalVector(h.m, 1)
	h.must(term.AddCoefficient(0, &p.Y))
	v.Index = []uint32{7}
	if e = term.Substitute(0, v); e != ErrBounds {
		t.Fatal(e)
	}
	if e = it.Reset(); e != ErrExactVectorPending {
		t.Fatal(e)
	}
	if _, _, e = it.Next(&out.X); e != ErrExactVectorPending {
		t.Fatal(e)
	}
	if _, _, e = it.NextIndex(); e != ErrExactVectorPending {
		t.Fatal(e)
	}
	h.must(it.Close())
	h.must(term.Close())
	h.must(v.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARTermIteratorRetainsPhysicalCopyReceipt(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for cohort := 0; cohort < 4 && !matched; cohort++ {
		for ordinal := int64(0); ordinal < 30 && !matched; ordinal++ {
			t.Run(fmt.Sprintf("%d/%d", cohort, ordinal), func(t *testing.T) {
				h := newLPPairHarness(t)
				text := strings.Repeat("1234567890", larTermMultilimbRepeats) + "/7"
				p := h.pair(text, "0")
				out := h.pair("0", "0")
				for n := 0; n < cohort; n++ {
					h.pair(text, text)
				}
				term := NewLARTerm(h.m)
				h.must(term.AddCoefficient(4, &p.X))
				it, e := NewLARTermIterator(term, 9)
				h.must(e)
				t.Cleanup(func() { h.budget.FailAfter(-1); h.must(it.Close()); h.must(term.Close()) })
				_, _, e = it.NextIndex()
				h.must(e)
				h.budget.FailAfter(ordinal)
				_, _, e = it.Next(&out.X)
				h.budget.FailAfter(-1)
				if it.calls.pending() {
					if !lpExactPriorityPhysicalRefusal(e) {
						t.Fatal(e)
					}
					matched = true
					if e := it.Reset(); e != ErrExactVectorPending {
						t.Fatal(e)
					}
					if _, _, e := it.Next(&out.X); e != ErrExactVectorPending {
						t.Fatal(e)
					}
					if _, _, e := it.NextIndex(); e != ErrExactVectorPending {
						t.Fatal(e)
					}
				}
				h.must(it.Close())
				h.must(term.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
	if !matched {
		t.Fatal("no actual iterator physical receipt")
	}
}
