package numeral

// Added independent MPBQ tests.

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestBinaryMixedConversionsAndIntervals(t *testing.T) {
	m, _ := binaryManager(t)
	qm := m.Diagnostics.Manager
	q := NewRational()
	r := NewRational()
	defer qm.drop(&q)
	defer qm.drop(&r)
	for _, den := range []uint64{1, 2, 8, 3, 6} {
		serviceMust(t, qm.SetFraction(&q, 5, den))
		var a BinaryRational
		ok, e := m.FromRational(qm, &q, &a)
		serviceMust(t, e)
		wantExact := den == 1 || den == 2 || den == 8
		if ok != wantExact {
			t.Fatal(den, ok)
		}
		serviceMust(t, m.ToRational(qm, &a, &r))
		got, e := qm.String(&r)
		serviceMust(t, e)
		want := binaryRat(t, m, &a).RatString()
		if got != want {
			t.Fatal(got, want)
		}
		v, e := m.ToOwnedRational(m.Diagnostics, &a)
		serviceMust(t, e)
		got, e = m.Diagnostics.String(&v)
		serviceMust(t, e)
		if got != want {
			t.Fatal(got, want)
		}
		serviceMust(t, m.Diagnostics.Del(&v))
		serviceMust(t, m.Del(&a))
	}
	// A real BQ owner borrowing the real rational owner's IntegerManager
	// exercises the shared-integer fast path, with no synthetic owner header.
	shared, e := NewBinaryRationalManager(qm.Integers(), BinaryRationalConfig{Diagnostics: m.Diagnostics})
	serviceMust(t, e)
	defer shared.Close()
	for _, bm := range []*BinaryRationalManager{m, shared} {
		owner, input := qm, &q
		serviceMust(t, qm.SetFraction(&q, 3, 5))
		a := NewBinaryIntPower(3, 2)
		eq, e := bm.EqualRational(&a, owner, input)
		serviceMust(t, e)
		lt, e := bm.LessRational(&a, owner, input)
		serviceMust(t, e)
		le, e := bm.LessEqualRational(&a, owner, input)
		serviceMust(t, e)
		gt, e := bm.GreaterRational(&a, owner, input)
		serviceMust(t, e)
		ge, e := bm.GreaterEqualRational(&a, owner, input)
		serviceMust(t, e)
		neq, e := bm.NotEqualRational(&a, owner, input)
		serviceMust(t, e)
		if eq || lt || le || !gt || !ge || !neq {
			t.Fatal("mixed ordering")
		}
	}
	for kind := 0; kind < 3; kind++ {
		serviceMust(t, qm.SetFraction(&q, 1, 3))
		serviceMust(t, qm.SetFraction(&r, 2, 3))
		l, u, out := NewBinaryIntPower(1, 2), NewBinaryIntPower(3, 2), NewBinaryRational()
		var e error
		if kind == 0 {
			e = m.SelectSmallRationalLower(qm, &q, &u, &out)
		}
		if kind == 1 {
			e = m.SelectSmallRationalUpper(qm, &l, &r, &out)
		}
		if kind == 2 {
			e = m.SelectSmallRationalBounds(qm, &q, &r, &out)
		}
		serviceMust(t, e)
		binaryText(t, m, &out, "1/2")
		serviceMust(t, m.Del(&out))
	}
	// Open integer endpoints use Inc/Dec, while a closed BQ integer endpoint
	// is selected immediately in the corresponding mixed overload.
	for kind := 0; kind < 3; kind++ {
		serviceMust(t, qm.SetFraction(&q, 0, 1))
		serviceMust(t, qm.SetFraction(&r, 1, 1))
		l, u, out := NewBinaryIntPower(1, 2), NewBinaryIntPower(1, 1), NewBinaryRational()
		var e error
		if kind == 0 {
			e = m.SelectSmallRationalLower(qm, &q, &u, &out)
		}
		if kind == 1 {
			e = m.SelectSmallRationalUpper(qm, &l, &r, &out)
		}
		if kind == 2 {
			e = m.SelectSmallRationalBounds(qm, &q, &r, &out)
		}
		serviceMust(t, e)
		binaryText(t, m, &out, "1/2")
		serviceMust(t, m.Del(&out))
	}
	for dir := 0; dir < 2; dir++ {
		n := int64(1)
		if dir == 0 {
			n = 2
		}
		serviceMust(t, qm.SetFraction(&q, n, 3))
		l, u := NewBinaryInt(0), NewBinaryInt(1)
		var e error
		if dir == 0 {
			e = m.RefineUpper(qm, &q, &l, &u)
		} else {
			e = m.RefineLower(qm, &q, &l, &u)
		}
		serviceMust(t, e)
		lo, e := m.LessRational(&l, qm, &q)
		serviceMust(t, e)
		hi, e := m.GreaterRational(&u, qm, &q)
		serviceMust(t, e)
		if !lo || !hi {
			t.Fatal("refinement enclosure")
		}
		if dir == 0 {
			binaryText(t, m, &l, "1/2")
			binaryText(t, m, &u, "3/2^2")
		} else {
			binaryText(t, m, &l, "1/2^2")
			binaryText(t, m, &u, "1/2")
		}
		serviceMust(t, m.Del(&l))
		serviceMust(t, m.Del(&u))
	}
	// Reachable raw negative denominator: source cross products do not flip.
	serviceMust(t, qm.Z.SetInt32(&q.Num, -1))
	serviceMust(t, qm.Z.SetInt32(&q.Den, -3))
	var rawSelected BinaryRational
	upperZero := NewBinaryInt(0)
	serviceMust(t, m.SelectSmallRationalLower(qm, &q, &upperZero, &rawSelected))
	binaryText(t, m, &rawSelected, "0")
	serviceMust(t, m.Del(&rawSelected))
	for dir := 0; dir < 2; dir++ {
		l, u := NewBinaryInt(1), NewBinaryInt(0)
		var e error
		if dir == 0 {
			e = m.RefineUpper(qm, &q, &l, &u)
		} else {
			e = m.RefineLower(qm, &q, &l, &u)
		}
		serviceMust(t, e)
		binaryText(t, m, &l, "1/2")
		want := "1/2^2"
		if dir == 1 {
			want = "0"
		}
		binaryText(t, m, &u, want)
		serviceMust(t, m.Del(&l))
		serviceMust(t, m.Del(&u))
	}
	// Unreduced dyadic denominator escapes the source !power2 guard; equality
	// is published before the final debug strict-enclosure assertion.
	serviceMust(t, qm.Z.SetInt32(&q.Num, 3))
	serviceMust(t, qm.Z.SetInt32(&q.Den, 6))
	for dir := 0; dir < 2; dir++ {
		l, u := NewBinaryInt(0), NewBinaryInt(1)
		var e error
		if dir == 0 {
			e = m.RefineUpper(qm, &q, &l, &u)
		} else {
			e = m.RefineLower(qm, &q, &l, &u)
		}
		if SourceExtendedGCDDebug {
			if !errors.Is(e, ErrBinaryInvariant) {
				t.Fatal(e)
			}
		} else {
			serviceMust(t, e)
		}
		if dir == 0 {
			binaryText(t, m, &l, "1/2")
			binaryText(t, m, &u, "3/2^2")
		} else {
			binaryText(t, m, &l, "1/2^2")
			binaryText(t, m, &u, "1/2")
		}
		serviceMust(t, m.Del(&l))
		serviceMust(t, m.Del(&u))
	}
}

func TestBinarySourceShapeDomainsAndPrefixes(t *testing.T) {
	m, budget := binaryManager(t)
	raw := NewBinaryIntPower(2, 1)
	one := NewBinaryInt(1)
	eq, e := m.Equal(&raw, &one)
	serviceMust(t, e)
	le, e := m.LessEqual(&raw, &one)
	serviceMust(t, e)
	ge, e := m.GreaterEqual(&raw, &one)
	serviceMust(t, e)
	if eq || !le || !ge {
		t.Fatal("structural equality")
	}
	a, b := NewBinaryInt(2), NewBinaryIntPower(1, 1)
	serviceMust(t, m.Mul(&a, &b, &a))
	if a.KField != 1 || a.Num.Val != 2 {
		t.Fatal("source alias normalization")
	}
	a = NewBinaryIntPower(1, math.MaxUint32)
	serviceMust(t, m.Div2(&a))
	if a.KField != 0 {
		t.Fatal("uint32 increment")
	}
	a = NewBinaryIntPower(1, 3)
	e = m.PowerInPlace(&a, 1<<31)
	if SourceExtendedGCDDebug {
		if !errors.Is(e, ErrBinaryInvariant) || a.KField != 3 {
			t.Fatal(e, a.KField)
		}
	} else {
		serviceMust(t, e)
		if a.KField != 1<<31 {
			t.Fatal(a.KField)
		}
	}
	a = NewBinaryIntPower(3, 1)
	budget.FailAfter(0)
	e = m.PowerInPlace(&a, 1<<31)
	binaryAudit(t, m, &raw, &one, &a, &b)
	if !numericPhysicalReservationRefusal(e) || a.KField != 1<<31 || a.Num != NewInt(1) {
		t.Fatal("inherited finite Power growth refusal after denominator and numerator publication", a, e)
	}
	budget.FailAfter(-1)
	a = NewBinaryIntPower(math.MinInt32, 31)
	e = m.normalize(&a)
	if SourceExtendedGCDDebug {
		if !errors.Is(e, ErrBinaryInvariant) {
			t.Fatal(e)
		}
	} else {
		serviceMust(t, e)
	}
	if a.Num.Val != 1 || a.KField != 0 {
		t.Fatal("inherited shift defect", a)
	}
	a = NewBinaryIntPower(0, 3)
	ok, e := m.RootLowerInPlace(&a, 2)
	serviceMust(t, e)
	if ok || a.KField != 0 {
		t.Fatal("raw zero exactness flag", ok, a)
	}
	if modularSourceDebug {
		_, _ = bindMPNDebugActions(t, "T")
		a = NewBinaryIntPower(-3, 2)
		var copiedOutput BinaryRational
		_, e = m.RootLower(&a, 2, &copiedOutput)
		if !isMPNVerificationThrow(e) || copiedOutput.Num.Val != -3 || copiedOutput.KField != 2 {
			t.Fatal("copy precedes root source assertion", e, copiedOutput)
		}
		serviceMust(t, m.Del(&copiedOutput))
	}
	a = NewBinaryInt(0)
	if _, e = m.RootUpperInPlace(&a, 0); !errors.Is(e, ErrBinaryRootDegreeZero) {
		t.Fatal(e)
	}
	a = NewBinaryIntPower(0, 8)
	e = m.Approx(&a, 2, false)
	if SourceExtendedGCDDebug {
		if !errors.Is(e, ErrBinaryInvariant) {
			t.Fatal(e)
		}
	} else {
		serviceMust(t, e)
	}
	if a.KField != 0 || a.Num.Val != 0 {
		t.Fatal(a)
	}
	a = NewBinaryIntPower(5, 3)
	lb, e := m.MagnitudeLower(&a)
	serviceMust(t, e)
	ub, e := m.MagnitudeUpper(&a)
	serviceMust(t, e)
	if lb != -1 || ub != 0 {
		t.Fatal(lb, ub)
	}
	a = NewBinaryIntPower(-3, 4)
	lb, e = m.MagnitudeLower(&a)
	serviceMust(t, e)
	ub, e = m.MagnitudeUpper(&a)
	serviceMust(t, e)
	if lb != -2 || ub != -3 {
		t.Fatal(lb, ub)
	}
	a = NewBinaryIntPower(1, math.MaxUint32)
	lb, e = m.MagnitudeLower(&a)
	serviceMust(t, e)
	if lb != 1 {
		t.Fatal("source unsigned magnitude wrap", lb)
	}
	var out BinaryRational
	before := NewBinaryInt(19)
	serviceMust(t, m.Copy(&out, &before))
	a, b = NewBinaryInt(2), NewBinaryInt(1)
	ok, e = m.SelectSmall(&a, &b, &out)
	serviceMust(t, e)
	if ok || out.Num.Val != 19 {
		t.Fatal("reversed leaves output")
	}
	if e = m.ApproxDiv(&a, &BinaryRational{}, &out, 4, false); !errors.Is(e, ErrDivideByZero) {
		t.Fatal(e)
	}
	serviceMust(t, m.Del(&a))
	serviceMust(t, m.Del(&b))
	serviceMust(t, m.Del(&out))
}

func TestBinaryRootBoundsFinite(t *testing.T) {
	m, _ := binaryManager(t)
	for n := uint32(1); n <= 4; n++ {
		for num := int32(-12); num <= 12; num++ {
			if num < 0 && n%2 == 0 {
				continue
			}
			for k := uint32(0); k <= 5; k++ {
				var a, lo, hi BinaryRational
				serviceMust(t, m.SetInt32Power(&a, num, k))
				want := binaryRat(t, m, &a)
				exactLo, e := m.RootLower(&a, n, &lo)
				serviceMust(t, e)
				exactHi, e := m.RootUpper(&a, n, &hi)
				serviceMust(t, e)
				lv, hv := binaryRat(t, m, &lo), binaryRat(t, m, &hi)
				lp, hp := new(big.Rat).SetInt64(1), new(big.Rat).SetInt64(1)
				for i := uint32(0); i < n; i++ {
					lp.Mul(lp, lv)
					hp.Mul(hp, hv)
				}
				if lp.Cmp(want) > 0 || hp.Cmp(want) < 0 {
					t.Fatal(num, k, n, lp, want, hp)
				}
				if exactLo != (lp.Cmp(want) == 0) || exactHi != (hp.Cmp(want) == 0) {
					t.Fatal("exactness", num, k, n, exactLo, exactHi)
				}
				serviceMust(t, m.Del(&a))
				serviceMust(t, m.Del(&lo))
				serviceMust(t, m.Del(&hi))
			}
		}
	}
}

func TestBinaryScalarForwardersAndWarmWorkspace(t *testing.T) {
	m, budget := binaryManager(t)
	var a, b, r BinaryRational
	var num, floor, ceil Int
	serviceMust(t, m.SetUint32(&a, math.MaxUint32))
	serviceMust(t, m.Numerator(&a, &num))
	if integerString(t, m.Z, &num) != "4294967295" {
		t.Fatal(num)
	}
	serviceMust(t, m.SetIntegerPower(&a, &num, 4))
	if a.K() != 4 {
		t.Fatal(a.K())
	}
	serviceMust(t, m.SetInt64Power(&b, -9, 2))
	one := NewInt(1)
	serviceMust(t, m.AddInteger(&b, &one, &r))
	binaryText(t, m, &r, "-5/2^2")
	serviceMust(t, m.SubInteger(&b, &one, &r))
	binaryText(t, m, &r, "-13/2^2")
	serviceMust(t, m.MulInteger(&b, &one, &r))
	binaryText(t, m, &r, "-9/2^2")
	serviceMust(t, m.AddMulInteger(&b, &one, &b, &r))
	binaryText(t, m, &r, "-9/2")
	eq, e := m.EqualInteger(&b, &one)
	serviceMust(t, e)
	neq, e := m.NotEqualInteger(&b, &one)
	serviceMust(t, e)
	lt, e := m.LessInteger(&b, &one)
	serviceMust(t, e)
	le, e := m.LessEqualInteger(&b, &one)
	serviceMust(t, e)
	gt, e := m.GreaterInteger(&b, &one)
	serviceMust(t, e)
	ge, e := m.GreaterEqualInteger(&b, &one)
	serviceMust(t, e)
	if eq || !neq || !lt || !le || gt || ge {
		t.Fatal("integer comparison")
	}
	serviceMust(t, m.FloorInto(&b, m.Diagnostics.Manager.Z, &floor))
	serviceMust(t, m.CeilInto(&b, m.Diagnostics.Manager.Z, &ceil))
	if floor.Val != -3 || ceil.Val != -2 {
		t.Fatal(floor, ceil)
	}
	serviceMust(t, m.Diagnostics.Manager.Z.Del(&floor))
	serviceMust(t, m.Diagnostics.Manager.Z.Del(&ceil))
	serviceMust(t, m.Reset(&a))
	serviceMust(t, m.SetInt32Power(&a, 1, 3))
	serviceMust(t, m.SetInt32Power(&b, 3, 3))
	var dst [128]byte
	for i := 0; i < 3; i++ {
		serviceMust(t, m.Add(&a, &b, &r))
		_, e = m.FormatInto(&r, dst[:])
		serviceMust(t, e)
		_, e = m.FormatSMT2Into(&r, true, dst[:])
		serviceMust(t, e)
		_, e = m.FormatPrettyInto(&r, dst[:])
		serviceMust(t, e)
		_, e = m.FormatDecimalInto(&r, 8, dst[:])
		serviceMust(t, e)
	}
	used := budget.Used()
	budget.FailAfter(0)
	allocations := testing.AllocsPerRun(50, func() {
		if e := m.Add(&a, &b, &r); e != nil {
			panic(e)
		}
		if _, e := m.FormatDecimalInto(&r, 8, dst[:]); e != nil {
			panic(e)
		}
	})
	budget.FailAfter(-1)
	if allocations != 0 || budget.Used() != used {
		t.Fatal("warm workspace", allocations, used, budget.Used())
	}
	numRoot := BinaryRational{Num: num} // Nonowning audit view of the caller Int.
	binaryAudit(t, m, &a, &b, &r, &numRoot)
	serviceMust(t, m.Z.Del(&num))
	serviceMust(t, m.Del(&a))
	serviceMust(t, m.Del(&b))
	serviceMust(t, m.Del(&r))
}

func TestBinaryDebugEnclosureShortCircuit(t *testing.T) {
	m, budget := binaryManager(t)
	q := NewRational()
	qm := m.Diagnostics.Manager
	serviceMust(t, qm.Z.SetInt32(&q.Den, 3))
	defer qm.drop(&q)
	l := NewBinaryInt(0)
	u := NewBinaryInt(1)
	serviceMust(t, m.Mul2k(&u, 1024))
	defer m.Del(&u)
	budget.FailAfter(0)
	e := m.debugEnclosure(qm, &q, &l, &u)
	budget.FailAfter(-1)
	if SourceExtendedGCDDebug {
		if !errors.Is(e, ErrBinaryInvariant) {
			t.Fatal("source && must not allocate through upper comparison", e)
		}
	} else {
		serviceMust(t, e)
	}
	binaryAudit(t, m, &l, &u)
}

func binaryRat(t *testing.T, m *BinaryRationalManager, a *BinaryRational) *big.Rat {
	t.Helper()
	n, e := m.Z.String(&a.Num)
	serviceMust(t, e)
	v, ok := new(big.Int).SetString(n, 10)
	if !ok {
		t.Fatal(n)
	}
	return new(big.Rat).SetFrac(v, new(big.Int).Lsh(big.NewInt(1), uint(a.KField)))
}
func TestBinaryArithmeticFiniteAliases(t *testing.T) {
	m, _ := binaryManager(t)
	for n := int32(-9); n <= 9; n++ {
		for d := int32(-5); d <= 5; d++ {
			for k := uint32(0); k < 4; k++ {
				for j := uint32(0); j < 4; j++ {
					for op := 0; op < 3; op++ {
						for alias := 0; alias < 3; alias++ {
							var a, b, r BinaryRational
							serviceMust(t, m.SetInt32Power(&a, n, k))
							serviceMust(t, m.SetInt32Power(&b, d, j))
							av, bv := binaryRat(t, m, &a), binaryRat(t, m, &b)
							out := &r
							if alias == 1 {
								out = &a
							}
							if alias == 2 {
								out = &b
							}
							want := new(big.Rat)
							var e error
							if op == 0 {
								want.Add(av, bv)
								e = m.Add(&a, &b, out)
							}
							if op == 1 {
								want.Sub(av, bv)
								e = m.Sub(&a, &b, out)
							}
							if op == 2 {
								want.Mul(av, bv)
								e = m.Mul(&a, &b, out)
							}
							serviceMust(t, e)
							if got := binaryRat(t, m, out); got.Cmp(want) != 0 {
								t.Fatal(n, d, k, j, op, alias, got, want)
							}
							serviceMust(t, m.Del(&a))
							serviceMust(t, m.Del(&b))
							serviceMust(t, m.Del(&r))
						}
					}
				}
			}
		}
	}
}
func TestBinarySelectionRootsAndApprox(t *testing.T) {
	m, _ := binaryManager(t)
	for _, k := range []uint32{1, 8, 9, 17} {
		l, u, r := NewBinaryIntPower(1, k), NewBinaryIntPower(3, k), NewBinaryRational()
		ok, e := m.SelectSmall(&l, &u, &r)
		serviceMust(t, e)
		if !ok {
			t.Fatal(k)
		}
		v := binaryRat(t, m, &r)
		if v.Cmp(binaryRat(t, m, &l)) < 0 || v.Cmp(binaryRat(t, m, &u)) > 0 {
			t.Fatal(k, v)
		}
		if r.KField != k-1 {
			t.Fatal("minimum denominator", k, r.KField)
		}
		serviceMust(t, m.Del(&r))
	}
	// The singleton forces the binary-search final VERIFY branch, where the
	// last failed mid is one below maxK, so select_integer must execute again.
	l, u, r := NewBinaryIntPower(1, 9), NewBinaryIntPower(1, 9), NewBinaryRational()
	ok, e := m.SelectSmall(&l, &u, &r)
	serviceMust(t, e)
	if !ok || r.KField != 9 {
		t.Fatal(ok, r.KField)
	}
	serviceMust(t, m.Del(&r))
	a := NewBinaryIntPower(9, 4)
	var lo, hi BinaryRational
	ok, e = m.RootLower(&a, 2, &lo)
	serviceMust(t, e)
	if !ok {
		t.Fatal("exact square")
	}
	binaryText(t, m, &lo, "3/2^2")
	ok, e = m.RootUpper(&a, 2, &hi)
	serviceMust(t, e)
	if !ok {
		t.Fatal("exact square")
	}
	a = NewBinaryIntPower(-2, 1)
	var f, c Int
	serviceMust(t, m.Floor(&a, &f))
	serviceMust(t, m.Ceil(&a, &c))
	if f.Val != -2 || c.Val != -1 {
		t.Fatal("raw source rounding", f, c)
	}
	serviceMust(t, m.Del(&lo))
	serviceMust(t, m.Del(&hi))
	serviceMust(t, m.Z.Del(&f))
	serviceMust(t, m.Z.Del(&c))
	for n := int32(-8); n <= 8; n++ {
		for d := int32(-7); d <= 7; d++ {
			if d == 0 {
				continue
			}
			for dir := 0; dir < 2; dir++ {
				var x, y, r BinaryRational
				serviceMust(t, m.SetInt32Power(&x, n, 3))
				serviceMust(t, m.SetInt32Power(&y, d, 2))
				want := new(big.Rat).Quo(binaryRat(t, m, &x), binaryRat(t, m, &y))
				serviceMust(t, m.ApproxDiv(&x, &y, &r, 7, dir == 1))
				got := binaryRat(t, m, &r)
				diff := new(big.Rat).Sub(got, want)
				if dir == 1 && diff.Sign() < 0 || dir == 0 && diff.Sign() > 0 {
					t.Fatal(n, d, dir, got, want)
				}
				diff.Abs(diff)
				if diff.Cmp(new(big.Rat).SetFrac64(1, 128)) > 0 {
					t.Fatal("error", n, d, diff)
				}
				serviceMust(t, m.Del(&x))
				serviceMust(t, m.Del(&y))
				serviceMust(t, m.Del(&r))
			}
		}
	}
}

// TestMPBQSelectSmallRegisteredAlgebraicSourceCases ports the concrete
// mpbq_manager::select_small cases registered by src/test/algebraic.cpp.
// Keep these inputs here, next to the concrete binary-rational implementation
// used by the algebraic root manager.
func TestMPBQSelectSmallRegisteredAlgebraicSourceCases(t *testing.T) {
	m, _ := binaryManager(t)
	cases := []struct {
		lowerNum int64
		lowerK   uint32
		upperNum int64
		upperK   uint32
	}{
		{1, 3, 3, 2},
		{10_000_000_000_000, 40, 11_000, 10},
		{10_000_000_000_000, 40, 10_001, 10},
		{1, 0, 1, 0},
		{1, 0, 2, 0},
		{-1, 0, -1, 0},
		{-2, 0, -1, 0},
		{0, 0, 1_100, 10},
		{7, 3, 1_001, 10},
		{1_000, 10, 1_001, 10},
	}
	for i, tc := range cases {
		l, u, out := NewBinaryRational(), NewBinaryRational(), NewBinaryRational()
		serviceMust(t, m.SetInt64Power(&l, tc.lowerNum, tc.lowerK))
		serviceMust(t, m.SetInt64Power(&u, tc.upperNum, tc.upperK))
		ok, e := m.SelectSmall(&l, &u, &out)
		serviceMust(t, e)
		if !ok {
			t.Fatalf("source case %d unexpectedly rejected", i)
		}
		lv, uv, got := binaryRat(t, m, &l), binaryRat(t, m, &u), binaryRat(t, m, &out)
		if got.Cmp(lv) < 0 || got.Cmp(uv) > 0 {
			t.Fatalf("source case %d selected %s outside [%s,%s]", i, got, lv, uv)
		}
		serviceMust(t, m.Del(&l))
		serviceMust(t, m.Del(&u))
		serviceMust(t, m.Del(&out))
	}

	// The final two C++ fixtures build (11^64)/2^192 and the adjacent
	// numerator values +1 and +256 before dividing by 2^192.
	for _, delta := range []int32{1, 256} {
		l, u, out := NewBinaryRational(), NewBinaryRational(), NewBinaryRational()
		serviceMust(t, m.SetInt32Power(&l, 11, 0))
		serviceMust(t, m.PowerInPlace(&l, 64))
		serviceMust(t, m.Copy(&u, &l))
		d := NewInt(delta)
		serviceMust(t, m.AddInteger(&u, &d, &u))
		serviceMust(t, m.Div2k(&l, 192))
		serviceMust(t, m.Div2k(&u, 192))
		ok, e := m.SelectSmall(&l, &u, &out)
		serviceMust(t, e)
		if !ok {
			t.Fatalf("source large-boundary case delta=%d unexpectedly rejected", delta)
		}
		lv, uv, got := binaryRat(t, m, &l), binaryRat(t, m, &u), binaryRat(t, m, &out)
		if got.Cmp(lv) < 0 || got.Cmp(uv) > 0 {
			t.Fatalf("source large-boundary case delta=%d selected %s outside [%s,%s]", delta, got, lv, uv)
		}
		serviceMust(t, m.Z.Del(&d))
		serviceMust(t, m.Del(&l))
		serviceMust(t, m.Del(&u))
		serviceMust(t, m.Del(&out))
	}
}

// TestMPBQRefineRegisteredAlgebraicSourceSequence covers both 20-step loops
// from src/test/algebraic.cpp::tst_refine_mpbq with its exact 5/7 input.
func TestMPBQRefineRegisteredAlgebraicSourceSequence(t *testing.T) {
	m, _ := binaryManager(t)
	qm := m.Diagnostics.Manager
	q := NewRational()
	serviceMust(t, qm.SetFraction(&q, 5, 7))
	qValue := new(big.Rat).SetFrac(big.NewInt(5), big.NewInt(7))
	for _, upper := range []bool{true, false} {
		l, u := NewBinaryRational(), NewBinaryRational()
		exact, e := m.FromRational(qm, &q, &l)
		serviceMust(t, e)
		if exact {
			t.Fatal("5/7 unexpectedly has a binary representation")
		}
		serviceMust(t, m.Copy(&u, &l))
		serviceMust(t, m.Mul2(&u))
		previousWidth := new(big.Rat).Sub(binaryRat(t, m, &u), binaryRat(t, m, &l))
		for i := 0; i < 20; i++ {
			if upper {
				e = m.RefineUpper(qm, &q, &l, &u)
			} else {
				e = m.RefineLower(qm, &q, &l, &u)
			}
			serviceMust(t, e)
			lv, uv := binaryRat(t, m, &l), binaryRat(t, m, &u)
			if lv.Cmp(qValue) >= 0 || uv.Cmp(qValue) <= 0 {
				t.Fatalf("step %d upper=%t lost strict enclosure: %s < %s < %s", i, upper, lv, qValue, uv)
			}
			width := new(big.Rat).Sub(uv, lv)
			if width.Cmp(previousWidth) >= 0 {
				t.Fatalf("step %d upper=%t did not narrow: %s >= %s", i, upper, width, previousWidth)
			}
			previousWidth = width
		}
		serviceMust(t, m.Del(&l))
		serviceMust(t, m.Del(&u))
	}
	serviceMust(t, qm.Del(&q))
}
