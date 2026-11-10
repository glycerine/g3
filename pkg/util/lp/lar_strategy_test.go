package lp

// Source: src/util/lp/init_lar_solver.h:229-263;
// src/util/lp/lar_solver.h:985-994.

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func lpLARStrategyFixture(t *testing.T, h *lpPairHarness, world int, materialized bool) (*LARSolver, []*big.Rat) {
	t.Helper()
	s, err := NewLARSolver(h.m)
	h.must(err)
	s.Settings.SimplexStrategy = StrategyUndecided
	_, err = s.AddVar(17)
	h.must(err)
	_, err = s.AddVar(31)
	h.must(err)
	a, b := big.NewRat(int64(1+world%19), int64(1+world%7)), big.NewRat(-int64(1+world%13), int64(1+world%11))
	v := h.pair(a.RatString(), b.RatString())
	input := NewSparseRationalVector(h.m)
	h.must(input.Append(0, &v.X))
	h.must(input.Append(1, &v.Y))
	zero := h.pair("0", "0")
	term, err := s.AddTerm(input.Data, &zero.X)
	h.must(err)
	if materialized {
		_, err = s.AddVarBound(term, LConstraintGE, &zero.X)
		h.must(err)
	}
	h.must(input.Close())
	return s, []*big.Rat{new(big.Rat).Neg(a), new(big.Rat).Neg(b), big.NewRat(1, 1)}
}

func lpLARSourceDouble(r *big.Rat) float64 {
	n, _ := new(big.Float).SetInt(r.Num()).Float64()
	d, _ := new(big.Float).SetInt(r.Denom()).Float64()
	return n / d
}

func TestLARStrategyIndependentThresholdModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 100; world++ {
		for _, materialized := range []bool{false, true} {
			for _, offset := range []int{-1, 0, 1} {
				t.Run(fmt.Sprintf("%d/%v/%d", world, materialized, offset), func(t *testing.T) {
					h := newLPPairHarness(t)
					s, row := lpLARStrategyFixture(t, h, world, materialized)
					// The term row lists the term's coefficients in libstdc++
					// unordered_map order, then the term column.
					termOrder := newLPTermOrder()
					termOrder.keys.Put(0, lpTermOrderValue)
					termOrder.keys.Put(1, lpTermOrderValue)
					rowOrder := append(termOrder.Keys(), 2)
					n := s.Bounds.Witnesses.Size()
					s.Settings.ColumnThresholdForLUInLAR = uint32(int(n) + offset)
					limit := &lpTestResourceLimit{err: lpErrExactSolveCancel}
					s.Settings.SetResourceLimit(limit)
					h.must(s.DecideOnStrategyAndAdjustInitialState())
					lu := offset < 0
					want := StrategyTableauRows
					if lu {
						want = StrategyLU
					}
					if s.Settings.SimplexStrategy != want || limit.calls != 0 || s.Core.RPrimal.Factorization != nil || s.Core.DPrimal.Factorization != nil {
						t.Fatal("literal threshold/no solve", want, s.Settings.SimplexStrategy, limit.calls)
					}
					if lu {
						wantRows := uint32(0)
						if materialized {
							wantRows = 1
							for pos, j := range rowOrder {
								value := row[j]
								cell := s.Core.DA.Rows[0][pos]
								if cell.Column != j || cell.Value != lpLARSourceDouble(value) || s.Core.DA.Columns[j][cell.Offset] != (ColumnCell{0, uint32(pos)}) {
									t.Fatal("independent sparse conversion", cell, value)
								}
							}
						}
						if s.Core.DA.RowCount() != wantRows || s.Core.DA.ColumnCount() != n || len(s.Core.DX) != int(n) || len(s.Core.DLow) != int(n) || len(s.Core.DUpper) != int(n) || !reflect.DeepEqual(s.Core.DBasis, s.Core.RBasis) || !reflect.DeepEqual(s.Core.DHeading, s.Core.RHeading) || len(s.Core.DNBasis) != 0 || len(s.Core.DA.RowOffsets) != 0 {
							t.Fatal("source floating prefix and omitted nonbasis/offsets")
						}
					} else {
						if s.Core.RA.RowCount() != 1 || s.Core.RA.ColumnCount() != 3 || s.Core.DA.ColumnCount() != 0 || s.ExternalToColumns[LARTermStartIndex] != 2 {
							t.Fatal("deferred term must materialize only for tableau")
						}
						for pos, j := range rowOrder {
							cell := &s.Core.RA.Rows[0][pos]
							if cell.Column != j {
								t.Fatal("source row order")
							}
							lpSparseRationalCheck(t, h, &cell.Value, row[j])
						}
					}
					h.must(s.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
			}
		}
	}
}

func TestLARStrategyLUResizesAndPreservesSourceContents(t *testing.T) {
	for _, large := range []bool{false, true} {
		h := newLPPairHarness(t)
		s, row := lpLARStrategyFixture(t, h, 7, true)
		c := s.Core
		dim := uint32(1)
		if large {
			dim = 5
		}
		c.DA.Rows = make([][]FloatRowCell, dim)
		c.DA.Columns = make([][]ColumnCell, dim)
		c.DA.RowOffsets = []int32{71, 72, 73, 74}
		h.must(c.DA.Set(0, 0, 23))
		c.DX, c.DLow, c.DUpper = make([]float64, dim), make([]float64, dim), make([]float64, dim)
		c.DX[0], c.DLow[0], c.DUpper[0] = 11, 12, 13
		c.DHeading, c.DBasis, c.DNBasis = []int32{19}, []uint32{18}, []uint32{17, 16}
		s.Settings.SimplexStrategy = StrategyLU
		h.must(s.AdjustInitialState())
		if c.DA.RowCount() != 1 || c.DA.ColumnCount() != 3 || len(c.DA.Rows[0]) != 4 || c.DA.Rows[0][0].Value != 23 || c.DX[0] != 11 || c.DLow[0] != 12 || c.DUpper[0] != 13 || !reflect.DeepEqual(c.DNBasis, []uint32{17, 16}) || !reflect.DeepEqual(c.DA.RowOffsets, []int32{71, 72, 73, 74}) {
			t.Fatal("source resize preserves nested contents and unrelated metadata")
		}
		// Appended cells follow the source row's (lar_term map) order; check
		// each converted value against its column.
		for _, cell := range c.DA.Rows[0][1:] {
			if cell.Value != lpLARSourceDouble(row[cell.Column]) {
				t.Fatal("append conversion order")
			}
		}
		c.RHeading[0] = -77
		c.RBasis[0] = 0
		if c.DHeading[0] == -77 || c.DBasis[0] == 0 {
			t.Fatal("metadata copies alias exact core")
		}
		h.must(s.CopyPairMatrixToFloat(c.DA))
		if len(c.DA.Rows[0]) != 7 || len(c.DA.Columns[0]) != 3 {
			t.Fatal("repeated source copy must append duplicates")
		}
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARStrategyDispatcherProfiles(t *testing.T) {
	for _, strategy := range []SimplexStrategy{StrategyUndecided, StrategyTableauRows, StrategyTableauCosts, SimplexStrategy(255)} {
		h := newLPPairHarness(t)
		s, _ := lpLARStrategyFixture(t, h, 0, false)
		s.Settings.SimplexStrategy = strategy
		want := error(nil)
		if strategy == StrategyTableauCosts && sourceDebugEnabled {
			want = ErrAssertion
		}
		if err := s.AdjustInitialState(); err != want {
			t.Fatal("source dispatcher", strategy, err, want)
		}
		rows := uint32(1)
		if strategy == SimplexStrategy(255) || want != nil {
			rows = 0
		}
		if s.Core.RA.RowCount() != rows {
			t.Fatal("source fallthrough", strategy, s.Core.RA.RowCount())
		}
		if want != nil && s.AdjustInitialState() != ErrExactVectorPending {
			t.Fatal("sticky failure")
		}
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARStrategyFloatConversionSourceExtremes(t *testing.T) {
	h := newLPPairHarness(t)
	s, _ := lpLARStrategyFixture(t, h, 0, true)
	huge := new(big.Int).Exp(big.NewInt(10), big.NewInt(400), nil)
	values := []*big.Rat{new(big.Rat).SetFrac(huge, big.NewInt(1)), new(big.Rat).SetFrac(big.NewInt(1), huge), new(big.Rat).SetFrac(new(big.Int).Add(huge, big.NewInt(1)), new(big.Int).Sub(huge, big.NewInt(1)))}
	for j := range s.Core.RA.Rows[0] {
		cell := &s.Core.RA.Rows[0][j]
		h.must(h.m.SetString(&cell.Value, values[cell.Column].RatString()))
	}
	h.must(s.AdjustInitialStateForLU())
	got := s.Core.DA.Rows[0]
	// Separate source numerator/denominator conversion gives +Inf, zero
	// (omitted by set), and Inf/Inf=NaN, rather than exact-ratio rounding.
	if len(got) != 2 || got[0].Column != 0 || !math.IsInf(got[0].Value, 1) || got[1].Column != 2 || !math.IsNaN(got[1].Value) {
		t.Fatal("source scalar conversions", got)
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARStrategyReachedErrorPrefixes(t *testing.T) {
	for _, phase := range []string{"stopped", "already-decided", "nil-target", "bad-column", "stale-value"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			s, _ := lpLARStrategyFixture(t, h, 0, true)
			c := s.Core
			column := c.RA.Rows[0][1].Column
			value := c.RA.Rows[0][1].Value
			want := error(nil)
			var err error
			switch phase {
			case "stopped":
				s.stopped = true
				want = ErrExactVectorPending
				err = s.AdjustInitialStateForLU()
				if s.AdjustInitialState() != want || s.DecideOnStrategyAndAdjustInitialState() != want || s.CopyPairMatrixToFloat(c.DA) != want {
					t.Fatal("stopped entry points")
				}
			case "already-decided":
				s.Settings.SimplexStrategy = StrategyLU
				if sourceDebugEnabled {
					want = ErrAssertion
				}
				err = s.DecideOnStrategyAndAdjustInitialState()
				if sourceDebugEnabled && s.Settings.SimplexStrategy != StrategyLU || !sourceDebugEnabled && s.Settings.SimplexStrategy != StrategyTableauRows {
					t.Fatal("assertion before strategy publication")
				}
			case "nil-target":
				want = ErrBounds
				err = s.CopyPairMatrixToFloat(nil)
			case "bad-column", "stale-value":
				s.Settings.ColumnThresholdForLUInLAR = 0
				if phase == "bad-column" {
					c.RA.Rows[0][1].Column = 3
					want = ErrBounds
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				} else {
					want = arena.ErrInvalidSlot
					expired := h.pair(strings.Repeat("1234567890", 20)+"/7", "0")
					c.RA.Rows[0][1].Value = expired.X
					h.must(h.m.Del(&expired.X))
				}
				err = s.DecideOnStrategyAndAdjustInitialState()
				if s.Settings.SimplexStrategy != StrategyLU || c.DA.RowCount() != 1 || c.DA.ColumnCount() != 3 || len(c.DA.Rows[0]) != 1 || len(c.DX) != 0 || len(c.DHeading) != 0 {
					t.Fatal("matrix failure retains first cell before floating vector resize")
				}
			}
			if err != want {
				t.Fatal("source error prefix", err, want)
			}
			if err != nil && s.AdjustInitialState() != ErrExactVectorPending {
				t.Fatal("failed initialization must stop")
			}
			c.RA.Rows[0][1].Column = column
			c.RA.Rows[0][1].Value = value
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARStrategyDeferredRowsPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		done, failures := false, 0
		for ordinal := int64(0); ordinal < 1200 && !done; ordinal++ {
			t.Run(fmt.Sprintf("%v/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				s, _ := lpLARStrategyFixture(t, h, 0, false)
				large := h.pair(strings.Repeat("1234567890", 20)+"/7", "0")
				h.must(s.Terms[0].AddCoefficient(0, &large.X))
				// Reach actual basic-value reconstruction as well as copying.
				h.must(s.Core.RX.Data[0].Copy(h.m, h.pair("123456789012345678901/19", "1/23")))
				h.budget.FailAfter(ordinal)
				err := s.DecideOnStrategyAndAdjustInitialState()
				h.budget.FailAfter(-1)
				if err == nil {
					done = true
					if s.Core.RA.RowCount() != 1 {
						t.Fatal("deferred source row not materialized")
					}
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) || s.Settings.SimplexStrategy != StrategyTableauRows || s.DecideOnStrategyAndAdjustInitialState() != ErrExactVectorPending {
						t.Fatal("source failure/selected strategy", err)
					}
				}
				h.must(s.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
		if !done || direct && failures == 0 {
			t.Fatal("allocation campaign did not reach failure and success", direct, failures)
		}
	}
}
