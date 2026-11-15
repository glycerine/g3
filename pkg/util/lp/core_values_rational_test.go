package lp

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestLPRationalCoreValuesIndependentReconstructionAndCosts(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 100; world++ {
		h := newLPPairHarness(t)
		r := 1 + world%4
		n := r + 3
		f := lpNewRationalCoreFixture(t, h, uint32(r), uint32(n))
		f.settings.SimplexStrategy = StrategyLU
		// Unit lower-triangular B gives an independently constructed exact solution.
		a := make([][]*big.Rat, r)
		point, costs := make([]*big.Rat, n), make([]*big.Rat, n)
		for j := 0; j < n; j++ {
			point[j] = big.NewRat(int64(world-47+j*3), 19)
			num := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(81+world)), big.NewInt(int64(j+1)))
			if (world+j)%2 == 0 {
				num.Neg(num)
			}
			costs[j] = new(big.Rat).SetFrac(num, big.NewInt(23))
			h.must(f.costs.copySlot(&f.costs.Data[j], &h.pair(costs[j].RatString(), "0").X))
			v := point[j]
			if j < r {
				v = big.NewRat(91, 1)
			}
			h.must(f.x.copySlot(&f.x.Data[j], &h.pair(v.RatString(), "0").X))
		}
		for i := 0; i < r; i++ {
			a[i] = make([]*big.Rat, n)
			rhs := new(big.Rat)
			for j := 0; j < n; j++ {
				v := new(big.Rat)
				if j == i {
					v.SetInt64(1)
				} else if j < i || j >= r {
					v.SetFrac64(int64((world+i+j)%9-4), 17)
				}
				a[i][j] = v
				h.must(f.a.Set(uint32(i), uint32(j), &h.pair(v.RatString(), "0").X))
				rhs.Add(rhs, new(big.Rat).Mul(v, point[j]))
			}
			h.must(f.b.copySlot(&f.b.Data[i], &h.pair(rhs.RatString(), "0").X))
		}
		c, e := NewRationalCoreSolverBase(f.input())
		h.must(e)
		rs := NewIndexedRationalVector(h.m, uint32(r))
		h.must(c.RsMinusAnX(rs.Data))
		for i := 0; i < r; i++ {
			want := new(big.Rat)
			for j := 0; j < r; j++ {
				want.Add(want, new(big.Rat).Mul(a[i][j], point[j]))
			}
			lpSparseRationalCheck(t, h, &rs.Data[i], want)
		}
		h.must(c.SolveAxEqualsB())
		for j := 0; j < n; j++ {
			lpSparseRationalCheck(t, h, &f.x.Data[j], point[j])
		}
		h.must(c.InitReducedCostsForOneIteration())
		// Solve yB=cB by independent backward substitution for this unit lower B.
		y := make([]*big.Rat, r)
		for j := r - 1; j >= 0; j-- {
			y[j] = new(big.Rat).Set(costs[j])
			for i := j + 1; i < r; i++ {
				y[j].Sub(y[j], new(big.Rat).Mul(y[i], a[i][j]))
			}
			lpSparseRationalCheck(t, h, &c.Y.Data[j], y[j])
		}
		for j := 0; j < n; j++ {
			want := new(big.Rat)
			if j >= r {
				want.Set(costs[j])
				for i := 0; i < r; i++ {
					want.Sub(want, new(big.Rat).Mul(y[i], a[i][j]))
				}
			}
			lpSparseRationalCheck(t, h, &c.D.Data[j], want)
		}
		wantCost := new(big.Rat)
		for j := 0; j < n; j++ {
			wantCost.Add(wantCost, new(big.Rat).Mul(costs[j], point[j]))
		}
		out := h.pair("91", "0")
		h.must(c.GetCost(&out.X))
		lpSparseRationalCheck(t, h, &out.X, wantCost)
		off, e := c.AMultXIsOffOnIndex([]uint32{999})
		h.must(e)
		if off {
			t.Fatal("precise indexed branch")
		}
		h.must(c.Close())
		h.must(rs.Close())
		f.close()
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalCoreValuesUpdateRestoreAndLiveDelta(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, tableau := range []bool{false, true} {
		for _, alias := range []string{"external", "entering", "basic"} {
			for world := 0; world < 60; world++ {
				h := newLPPairHarness(t)
				f := lpNewRationalCoreFixture(t, h, 2, 3)
				f.settings.SimplexStrategy = StrategyTableauRows
				vals := []*big.Rat{big.NewRat(int64(world-29), 13), big.NewRat(int64(world+7), 17), big.NewRat(int64(world-3), 19)}
				coeff := []*big.Rat{big.NewRat(3, 23), big.NewRat(-7, 29)}
				for j := range vals {
					h.must(f.x.copySlot(&f.x.Data[j], &h.pair(vals[j].RatString(), "0").X))
				}
				// Deliberately insert row1 first so tableau traversal differs from Ed order.
				h.must(f.a.Set(1, 2, &h.pair(coeff[1].RatString(), "0").X))
				h.must(f.a.Set(0, 2, &h.pair(coeff[0].RatString(), "0").X))
				c, e := NewRationalCoreSolverBase(f.input())
				h.must(e)
				c.Ed.Index = []uint32{0, 1}
				for i := 0; i < 2; i++ {
					h.must(c.Ed.copySlot(&c.Ed.Data[i], &h.pair(coeff[i].RatString(), "0").X))
					h.must(c.CopyXB.copySlot(&c.CopyXB.Data[i], &h.pair(vals[i].RatString(), "0").X))
				}
				if !tableau {
					f.settings.SimplexStrategy = StrategyLU
				}
				delta := h.pair("11/31", "0")
				arg := &delta.X
				current := big.NewRat(11, 31)
				if alias == "entering" {
					arg = &f.x.Data[2]
					current = vals[2]
				}
				if alias == "basic" {
					arg = &f.x.Data[0]
					current = vals[0]
				}
				expected := make([]*big.Rat, 3)
				for j := range vals {
					expected[j] = new(big.Rat).Set(vals[j])
				}
				expected[2].Add(expected[2], current)
				order := []int{0, 1}
				if tableau {
					order = []int{1, 0}
				}
				for _, i := range order {
					step := big.NewRat(11, 31)
					if alias == "entering" {
						step = expected[2]
					}
					if alias == "basic" {
						step = expected[0]
					}
					expected[i].Sub(expected[i], new(big.Rat).Mul(step, coeff[i]))
				}
				h.must(c.UpdateX(2, arg))
				for j := range expected {
					lpSparseRationalCheck(t, h, &f.x.Data[j], expected[j])
				}
				for i := 0; i < 2; i++ {
					lpSparseRationalCheck(t, h, &c.CopyXB.Data[i], vals[i])
				}
				// Restoration uses the literal saved values even for precise arithmetic.
				restore := h.pair("5/37", "0")
				h.must(c.RestoreX(2, &restore.X))
				lpSparseRationalCheck(t, h, &f.x.Data[2], new(big.Rat).Sub(expected[2], big.NewRat(5, 37)))
				for i := 0; i < 2; i++ {
					lpSparseRationalCheck(t, h, &f.x.Data[i], vals[i])
				}
				h.must(c.Close())
				f.close()
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPRationalCoreValuesSnapAndAliasedCopies(t *testing.T) {
	h := newLPPairHarness(t)
	f := lpNewRationalCoreFixture(t, h, 1, 7)
	f.settings.SimplexStrategy = StrategyLU
	h.must(f.a.Set(0, 0, &h.pair("1", "0").X))
	h.must(f.a.Set(0, 1, &h.pair("2", "0").X))
	h.must(f.b.copySlot(&f.b.Data[0], &h.pair("17", "0").X))
	f.types = []ColumnType{ColumnFree, ColumnFixed, ColumnBoxed, ColumnLowBound, ColumnUpperBound, ColumnFree, ColumnType(99)}
	for j := range f.x.Data {
		h.must(f.x.copySlot(&f.x.Data[j], &h.pair("91", "0").X))
		h.must(f.low.copySlot(&f.low.Data[j], &h.pair(fmt.Sprint(j), "0").X))
		h.must(f.up.copySlot(&f.up.Data[j], &h.pair(fmt.Sprint(j+10), "0").X))
	}
	c, e := NewRationalCoreSolverBase(f.input())
	h.must(e)
	h.must(c.SnapXNToBoundsAndFreeColumnsToZeroes())
	want := []int64{15, 1, 2, 3, 14, 0, 0}
	for j, v := range want {
		lpSparseRationalCheck(t, h, &f.x.Data[j], big.NewRat(v, 1))
	}
	// A cost return assignment may alias an input; only the final assignment changes it.
	for j := range f.costs.Data {
		h.must(f.costs.copySlot(&f.costs.Data[j], &h.pair("1", "0").X))
	}
	h.must(c.GetCost(&f.x.Data[0]))
	lpSparseRationalCheck(t, h, &f.x.Data[0], big.NewRat(35, 1))
	h.must(c.Close())
	f.close()
	lpExactVectorNamedCensus(t, h, nil)
	h = newLPPairHarness(t)
	f = lpNewRationalCoreFixture(t, h, 2, 2)
	f.basis = []uint32{1, 0}
	c, e = NewRationalCoreSolverBase(f.input())
	h.must(e)
	h.must(f.x.copySlot(&f.x.Data[0], &h.pair("3", "0").X))
	h.must(f.x.copySlot(&f.x.Data[1], &h.pair("7", "0").X))
	h.must(c.CopyRsToXB(f.x.Data))
	for j := range f.x.Data {
		lpSparseRationalCheck(t, h, &f.x.Data[j], big.NewRat(7, 1))
	}
	h.must(c.Close())
	f.close()
	lpExactVectorNamedCensus(t, h, nil)
}

// Source src/test/lp.cpp:1725-1769. Reconstruct the known basis and certify its
// answer; the unfinished exact optimizer is not credited with discovering it.
func TestLPRationalCoreValuesUpstreamSolveRationalBasis(t *testing.T) {
	h := newLPPairHarness(t)
	m := lpRationalInputModel(t, h)
	cost := []int64{-3, -1, -1, 2, -1, 1, 1, -4}
	a := [][]int64{{1, 0, 3, 1, -5, -2, 4, -6}, {0, 1, -2, -1, 4, 1, -3, 5}}
	rhs := []int64{7, -3}
	up := []int64{8, 6, 4, 15, 2, 10, 10, 3}
	point := []int64{0, 6, 0, 15, 2, 1, 1, 0}
	reduced := []int64{-1, 2, -1, 1, 1, 0, 0, -1}
	for i, v := range rhs {
		h.must(m.AddConstraint(RelationEqual, &h.pair(fmt.Sprint(v), "0").X, uint32(i)))
	}
	for j, v := range cost {
		h.must(m.SetCostForColumn(uint32(j), &h.pair(fmt.Sprint(v), "0").X))
		h.must(m.GiveSymbolicNameToColumn(fmt.Sprint("x", j+1), uint32(j)))
		h.must(m.SetLowBound(uint32(j), &h.pair("0", "0").X))
		h.must(m.SetUpperBound(uint32(j), &h.pair(fmt.Sprint(up[j]), "0").X))
	}
	for i, row := range a {
		for j, v := range row {
			h.must(m.SetRowColumnCoefficient(uint32(i), uint32(j), &h.pair(fmt.Sprint(v), "0").X))
		}
	}
	h.must(m.Cleanup())
	h.must(m.FillMatrixAAndInitRightSide())
	h.must(m.FillB())
	h.must(m.Scale())
	h.must(m.Costs.resizeData(8))
	m.Settings.SimplexStrategy = StrategyLU
	x, lo, hi := NewIndexedRationalVector(h.m, 8), NewIndexedRationalVector(h.m, 8), NewIndexedRationalVector(h.m, 8)
	types := make([]ColumnType, 8)
	for j := uint32(0); j < 8; j++ {
		ext := m.CoreColumnsToExternal[j]
		col, _ := m.Columns.Get2(ext)
		types[j] = col.ColumnType()
		h.must(lo.copySlot(&lo.Data[j], &col.Low))
		h.must(hi.copySlot(&hi.Data[j], &col.Upper))
		h.must(m.SetScaledCost(j))
		v := point[ext]
		if ext == 5 || ext == 6 {
			v = 91
		}
		h.must(x.copySlot(&x.Data[j], &h.pair(fmt.Sprint(v), "0").X))
	}
	c5, _ := m.Columns.Get2(5)
	c6, _ := m.Columns.Get2(6)
	basis := []uint32{c5.Index, c6.Index}
	var nb []uint32
	var heading []int32
	c, e := NewRationalCoreSolverBase(RationalCoreInputs{A: m.A, B: &m.B.Data, X: &x.Data, Costs: &m.Costs.Data, Basis: &basis, NBasis: &nb, Heading: &heading, ColumnTypes: &types, LowBounds: &lo.Data, UpperBounds: &hi.Data, Settings: m.Settings, Names: m})
	h.must(e)
	h.must(c.SolveAxEqualsB())
	ok, e := c.FindXBySolving()
	h.must(e)
	if !ok {
		t.Fatal("upstream known basis failed exact residual validation")
	}
	h.must(c.InitReducedCostsForOneIteration())
	out := h.pair("91", "0")
	h.must(c.GetCost(&out.X))
	lpSparseRationalCheck(t, h, &out.X, big.NewRat(24, 1))
	// Core rows follow m_A_values' libstdc++ order, so look the duals up
	// through the external row map.
	lpSparseRationalCheck(t, h, &c.Y.Data[m.ExternalRowsToCore[0]], big.NewRat(-2, 1))
	lpSparseRationalCheck(t, h, &c.Y.Data[m.ExternalRowsToCore[1]], big.NewRat(-3, 1))
	for ext, v := range point {
		j, _ := m.Columns.Get2(uint32(ext))
		h.must(m.ColumnValueFromCore(uint32(ext), x.Data, &out.X))
		lpSparseRationalCheck(t, h, &out.X, big.NewRat(v, 1))
		lpSparseRationalCheck(t, h, &c.D.Data[j.Index], big.NewRat(reduced[ext], 1))
	}
	// Independent dual support bound y*b + max(d_j*lo_j,d_j*up_j) = 24.
	bound := int64(-2*7 + -3*-3)
	for j, d := range reduced {
		if d > 0 {
			bound += d * up[j]
		}
	}
	if bound != 24 {
		t.Fatal("independent dual certificate", bound)
	}
	h.must(c.Close())
	h.must(x.Close())
	h.must(lo.Close())
	h.must(hi.Close())
	h.must(m.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalCoreValuesPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, phase := range []string{"update-lu", "update-tableau", "restore", "rs", "copy", "solve", "reduced", "iteration", "cost", "snap", "snap-solve"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 750 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct%t/%s/%d", direct, phase, ordinal), func(t *testing.T) {
					h := newLPPairHarness(t)
					if direct {
						h = lpNewDirectSparseHarness(t)
					}
					f := lpNewRationalCoreFixture(t, h, 2, 3)
					f.settings.SimplexStrategy = StrategyLU
					large := h.pair(strings.Repeat("1234567890", 20)+"7/13", "0")
					other := h.pair(strings.Repeat("9876543210", 20)+"1/17", "0")
					one := h.pair("1", "0")
					out := h.pair("91", "0")
					h.must(f.a.Set(0, 0, &one.X))
					h.must(f.a.Set(1, 1, &one.X))
					h.must(f.a.Set(0, 2, &large.X))
					h.must(f.a.Set(1, 2, &other.X))
					for j := range f.x.Data {
						if phase != "copy" && phase != "snap" {
							h.must(f.x.copySlot(&f.x.Data[j], &other.X))
						}
						h.must(f.costs.copySlot(&f.costs.Data[j], &large.X))
						h.must(f.low.copySlot(&f.low.Data[j], &large.X))
					}
					if phase == "snap" || phase == "snap-solve" {
						f.types[2] = ColumnLowBound
					}
					for i := range f.b.Data {
						h.must(f.b.copySlot(&f.b.Data[i], &large.X))
					}
					c, e := NewRationalCoreSolverBase(f.input())
					h.must(e)
					c.Ed.Index = []uint32{0, 1}
					for i := range c.Ed.Data {
						h.must(c.Ed.copySlot(&c.Ed.Data[i], &large.X))
						h.must(c.CopyXB.copySlot(&c.CopyXB.Data[i], &large.X))
						h.must(c.Y.copySlot(&c.Y.Data[i], &other.X))
					}
					rs := NewIndexedRationalVector(h.m, 2)
					if phase == "copy" {
						for i := range rs.Data {
							h.must(rs.copySlot(&rs.Data[i], &large.X))
						}
					}
					if phase == "update-tableau" {
						f.settings.SimplexStrategy = StrategyTableauRows
					}
					h.budget.FailAfter(ordinal)
					var err error
					switch phase {
					case "update-lu", "update-tableau":
						err = c.UpdateX(2, &large.X)
					case "restore":
						err = c.RestoreX(2, &large.X)
					case "rs":
						err = c.RsMinusAnX(rs.Data)
					case "copy":
						err = c.CopyRsToXB(rs.Data)
					case "solve":
						err = c.SolveAxEqualsB()
					case "reduced":
						err = c.FillReducedCostsFromYByRows()
					case "iteration":
						err = c.InitReducedCostsForOneIteration()
					case "cost":
						err = c.GetCost(&out.X)
					case "snap":
						err = c.SnapNonBasicXToBoundAndFreeToZeroes()
					case "snap-solve":
						err = c.SnapXNToBoundsAndFreeColumnsToZeroes()
					}
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if c.SolveAxEqualsB() != ErrExactVectorPending {
							t.Fatal("stopped value core accepted retry")
						}
					}
					h.must(c.Close())
					h.must(rs.Close())
					f.close()
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("incomplete value physical campaign", direct, phase, failures)
			}
		}
	}
}

func TestLPRationalCoreValuesSourcePrefixesAndGuards(t *testing.T) {
	for _, kind := range []string{"zero-restore", "nil-restore", "nil-delta", "entering", "settings-after-entering", "late-ed", "late-tableau", "restore-late", "rs-late", "copy-late", "solve-no-lu", "reduced-cost-prefix", "reduced-zero-y", "reduced-short-y", "reduced-short-d", "reduced-short-heading", "reduced-row-heading", "snap-types", "snap-prefix", "snap-low", "snap-upper", "snap-solve-no-lu", "cost-short-x", "cost-empty", "cost-nil", "index-no-reads", "index-size"} {
		t.Run(kind, func(t *testing.T) {
			h := newLPPairHarness(t)
			f := lpNewRationalCoreFixture(t, h, 2, 3)
			f.settings.SimplexStrategy = StrategyTableauRows
			one := h.pair("1", "0")
			two := h.pair("2", "0")
			out := h.pair("91", "0")
			h.must(f.a.Set(0, 0, &one.X))
			h.must(f.a.Set(1, 1, &one.X))
			h.must(f.a.Set(0, 2, &two.X))
			h.must(f.a.Set(1, 2, &two.X))
			for j := range f.x.Data {
				h.must(f.x.copySlot(&f.x.Data[j], &two.X))
				h.must(f.costs.copySlot(&f.costs.Data[j], &one.X))
			}
			for i := range f.b.Data {
				h.must(f.b.copySlot(&f.b.Data[i], &h.pair("17", "0").X))
			}
			c, e := NewRationalCoreSolverBase(f.input())
			h.must(e)
			h.must(c.Ed.copySlot(&c.Ed.Data[0], &two.X))
			c.Ed.Index = []uint32{0}
			rs := NewIndexedRationalVector(h.m, 2)
			var err, want error
			switch kind {
			case "zero-restore":
				zero := h.pair("0", "0")
				err = c.RestoreX(99, &zero.X)
			case "nil-delta":
				err = c.UpdateX(2, nil)
				want = ErrBounds
			case "entering":
				err = c.UpdateX(99, &one.X)
				want = ErrBounds
			case "settings-after-entering":
				c.Settings = nil
				err = c.UpdateX(2, &one.X)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[2], big.NewRat(3, 1))
			case "late-ed":
				f.settings.SimplexStrategy = StrategyLU
				c.Ed.Index = []uint32{0, 99}
				err = c.UpdateX(2, &one.X)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[0], new(big.Rat))
				lpSparseRationalCheck(t, h, &f.x.Data[2], big.NewRat(3, 1))
			case "late-tableau":
				f.a.Columns[2] = append(f.a.Columns[2], ColumnCell{Row: 99})
				err = c.UpdateX(2, &one.X)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[0], new(big.Rat))
			case "restore-late":
				c.Ed.Index = []uint32{0, 99}
				h.must(c.CopyXB.copySlot(&c.CopyXB.Data[0], &h.pair("7", "0").X))
				err = c.RestoreX(2, &one.X)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[2], big.NewRat(1, 1))
				lpSparseRationalCheck(t, h, &f.x.Data[0], big.NewRat(7, 1))
			case "rs-late":
				f.a.Rows[0][1].Column = 99
				err = c.RsMinusAnX(rs.Data)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &rs.Data[1], big.NewRat(13, 1))
				lpSparseRationalCheck(t, h, &rs.Data[0], big.NewRat(17, 1))
			case "copy-late":
				f.basis[0] = 99
				h.must(rs.copySlot(&rs.Data[1], &h.pair("7", "0").X))
				err = c.CopyRsToXB(rs.Data)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[1], big.NewRat(7, 1))
			case "solve-no-lu":
				err = c.SolveAxEqualsB()
				want = ErrBounds
				if c.reconstructionWork == nil {
					t.Fatal("lost local RHS")
				}
				lpSparseRationalCheck(t, h, &c.reconstructionWork.Data[1], big.NewRat(13, 1))
			case "reduced-cost-prefix":
				f.heading[2] = 0
				f.heading[1] = -1
				owner := f.costs.Data
				f.costs.Data = f.costs.Data[:0]
				h.must(c.D.copySlot(&c.D.Data[2], &out.X))
				err = c.FillReducedCostsFromYByRows()
				f.costs.Data = owner
				want = ErrBounds
				lpSparseRationalCheck(t, h, &c.D.Data[2], new(big.Rat))
			case "reduced-zero-y":
				f.a.Rows[1][0].Column = 99
				err = c.FillReducedCostsFromYByRows()
			case "nil-restore":
				err = c.RestoreX(2, nil)
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[2], big.NewRat(2, 1))
			case "reduced-short-y":
				owner := c.Y.Data
				c.Y.Data = c.Y.Data[:1]
				err = c.FillReducedCostsFromYByRows()
				c.Y.Data = owner
				want = ErrBounds
				lpSparseRationalCheck(t, h, &c.D.Data[2], big.NewRat(1, 1))
			case "reduced-short-d":
				owner := c.D.Data
				c.D.Data = c.D.Data[:2]
				err = c.FillReducedCostsFromYByRows()
				c.D.Data = owner
				want = ErrBounds
			case "reduced-short-heading":
				f.heading = f.heading[:2]
				err = c.FillReducedCostsFromYByRows()
				want = ErrBounds
			case "reduced-row-heading":
				h.must(c.Y.copySlot(&c.Y.Data[1], &one.X))
				f.a.Rows[1][0].Column = 99
				err = c.FillReducedCostsFromYByRows()
				want = ErrBounds
				lpSparseRationalCheck(t, h, &c.D.Data[2], big.NewRat(1, 1))
			case "snap-types":
				f.types = f.types[:2]
				err = c.SnapNonBasicXToBoundAndFreeToZeroes()
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[2], big.NewRat(2, 1))
			case "snap-prefix":
				f.nb = []uint32{2, 99}
				f.types[2] = ColumnLowBound
				h.must(f.low.copySlot(&f.low.Data[2], &h.pair("7", "0").X))
				err = c.SnapNonBasicXToBoundAndFreeToZeroes()
				want = ErrBounds
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				lpSparseRationalCheck(t, h, &f.x.Data[2], big.NewRat(7, 1))
			case "snap-low":
				f.types[2] = ColumnLowBound
				owner := f.low.Data
				f.low.Data = nil
				err = c.SnapNonBasicXToBoundAndFreeToZeroes()
				f.low.Data = owner
				want = ErrBounds
			case "snap-upper":
				f.types[2] = ColumnUpperBound
				owner := f.up.Data
				f.up.Data = nil
				err = c.SnapNonBasicXToBoundAndFreeToZeroes()
				f.up.Data = owner
				want = ErrBounds
			case "snap-solve-no-lu":
				err = c.SnapXNToBoundsAndFreeColumnsToZeroes()
				want = ErrBounds
				lpSparseRationalCheck(t, h, &f.x.Data[2], new(big.Rat))
			case "cost-short-x":
				owner := f.x.Data
				f.x.Data = f.x.Data[:2]
				err = c.GetCost(&out.X)
				f.x.Data = owner
				want = ErrBounds
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					lpSparseRationalCheck(t, h, &c.costWork.Data[0], big.NewRat(4, 1))
				}
				lpSparseRationalCheck(t, h, &out.X, big.NewRat(91, 1))
			case "cost-empty":
				owner := f.costs.Data
				f.costs.Data = nil
				err = c.GetCost(&out.X)
				f.costs.Data = owner
				if sourceDebugEnabled {
					want = ErrAssertion
				} else {
					lpSparseRationalCheck(t, h, &out.X, new(big.Rat))
				}
			case "cost-nil":
				err = c.GetCost(nil)
				if err == nil {
					t.Fatal("nil cost result accepted")
				}
				want = err
			case "index-no-reads":
				c.Settings = nil
				owner := f.b.Data
				f.b.Data = nil
				off, e := c.AMultXIsOffOnIndex([]uint32{99})
				f.b.Data = owner
				err = e
				if off {
					t.Fatal("precise indexed residual")
				}
			case "index-size":
				owner := f.x.Data
				f.x.Data = nil
				off, e := c.AMultXIsOffOnIndex(nil)
				f.x.Data = owner
				err = e
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				if off {
					t.Fatal("native indexed residual")
				}
			}
			if err != want {
				t.Fatal(kind, err, want)
			}
			h.must(c.Close())
			h.must(rs.Close())
			f.close()
			lpExactVectorNamedCensus(t, h, nil)
			for _, e := range []error{c.UpdateX(0, &one.X), c.RestoreX(0, &one.X), c.RsMinusAnX(nil), c.CopyRsToXB(nil), c.SolveAxEqualsB(), c.GetCost(&out.X), c.FillReducedCostsFromYByRows(), c.InitReducedCostsForOneIteration(), c.SnapNonBasicXToBoundAndFreeToZeroes(), c.SnapXNToBoundsAndFreeColumnsToZeroes()} {
				if e != ErrExactVectorPending {
					t.Fatal("closed value entry", e)
				}
			}
		})
	}
}

func TestLPRationalCoreValuesPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		f := lpNewRationalCoreFixture(t, h, 0, 1)
		c, e := NewRationalCoreSolverBase(f.input())
		h.must(e)
		v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
		out := h.pair("91", "0")
		var caller exactEtaCalls
		b, e := caller.begin(h.m)
		h.must(e)
		h.budget.FailAfter(ordinal)
		_, e = h.m.Compare(&v.X, &v.Y)
		h.budget.FailAfter(-1)
		e = caller.after(h.m, b, e)
		older := h.m.ComparisonInvocationState()
		if older.Active {
			matched = true
			if !lpExactPriorityPhysicalRefusal(e) {
				t.Fatal(e)
			}
			_ = c.GetCost(&out.X)
			if c.calls.comparison != 0 {
				t.Fatal("adopted older comparison")
			}
		}
		h.must(c.Close())
		if older.Active && !reflect.DeepEqual(h.m.ComparisonInvocationState(), older) {
			t.Fatal("consumed older comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		f.close()
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}
