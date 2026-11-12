package lp

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"
)

func TestLPExactLUExtensionAllocationCensus(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprint(direct), func(t *testing.T) {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 500 && !success; ordinal++ {
				t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					a := lpExactLUStatic(t, h, lpExactIdentity(1), []uint32{0})
					var p *PairLU
					var state *exactLUState
					var err error

					p, err = NewPairLU(a, []uint32{0}, lpFloatLUSettings(math.Inf(1)))
					state = p.exactLUState

					h.must(err)
					defer func() {
						h.must(p.Close())
					}()
					h.must(a.AddRow())
					v := h.pair("1"+strings.Repeat("0", 31)+"1/7", "0")
					h.must(a.Set(1, 0, &v.X))
					h.must(h.m.SetInt64(&v.Y, 1))
					h.must(a.Set(1, 1, &v.Y))
					heading := []int32{0, 1, -1}
					h.budget.FailAfter(ordinal)

					columns := NewStdUnorderedUintSet()
					columns.Insert(0)
					columns.Insert(1)
					err = p.AddLastRowsToB(heading, columns)

					h.budget.FailAfter(-1)
					if err == nil {
						success = true
						b := lpExactIdentity(2)
						b[1][0] = mustLPRat(t, "1"+strings.Repeat("0", 31)+"1/7")
						x := []*big.Rat{big.NewRat(2, 3), big.NewRat(-5, 11)}
						lpExactLUUpdateCheckSolutions(t, h, nil, p, b, x, x)
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if !state.stopped || state.Dim != 2 {
							t.Fatal("extension failure prefix", state.Dim, state.stopped)
						}

						err = p.AdjustDimensionWithMatrixA()

						if err != ErrLUStopped {
							t.Fatal(err)
						}
					}

					h.must(p.Close())

					lpExactVectorNamedCensus(t, h, lpExactLUStaticCells(a))
					lpSparseRationalCheck(t, h, &a.Rows[1][0].Value, mustLPRat(t, "1"+strings.Repeat("0", 31)+"1/7"))
				})
			}
			if !success || (direct && failures == 0) {
				t.Fatal("extension allocation walk incomplete", success, failures)
			}
			t.Logf("physical refusals=%d", failures)
		})
	}
}

func TestLPLUExtensionLiteralGuardsAndPrefixes(t *testing.T) {
	for _, kind := range []string{"float", "pair"} {
		for _, phase := range []string{"empty", "heading size", "heading bounds", "negative heading", "column bounds", "shrink", "row work", "extension work", "prepare bounds", "degenerated", "closed"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				h := newLPPairHarness(t)
				a := lpExactLUStatic(t, h, lpExactIdentity(2), []uint32{0, 1})
				fa := lpFloatLUSource(t, lpExactIdentity(2), []uint32{0, 1})
				s := lpFloatLUSettings(math.Inf(1))
				f, err := NewFloatLU(fa, []uint32{0, 1}, s)
				lpStaticMust(t, err)
				defer f.Close()
				p, err := NewPairLU(a, []uint32{0, 1}, s)
				h.must(err)
				defer func() { h.must(p.Close()) }()
				heading := []int32{0, 1, -1, -1, -1}
				columns := NewStdUnorderedUintSet()
				want := error(nil)
				switch phase {
				case "empty":
				case "heading size":
					heading = nil
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				case "heading bounds":
					h.must(a.AddRow())
					fa.AddRow()
					value := h.pair("1", "0")
					h.must(a.Set(2, 4, &value.X))
					lpStaticMust(t, fa.Set(2, 4, 1))
					heading = heading[:2]
					want = ErrBounds
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				case "negative heading":
					columns.Insert(2)
					want = ErrBounds
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				case "column bounds":
					columns.Insert(5)
					want = ErrBounds
				case "shrink":
					a.Rows = a.Rows[:1]
					fa.Rows = fa.Rows[:1]
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				case "row work":
					f.rowEtaWork.Index = []uint32{2}
					p.rowEtaWork.Index = []uint32{2}
					want = ErrBounds
				case "extension work":
					f.wExtension.Index = []uint32{0}
					p.wExtension.Index = []uint32{0}
					want = ErrBounds
				case "prepare bounds":
					want = ErrBounds
					if sourceDebugEnabled {
						want = ErrAssertion
					}
				case "closed":
					lpStaticMust(t, f.Close())
					h.must(p.Close())
					want = ErrLUClosed
				case "degenerated":
					h.must(a.AddRow())
					fa.AddRow()
					heading[2] = 2
					columns.Insert(2)
				}
				switch phase {
				case "heading bounds":
					if kind == "float" {
						_, err = f.ColumnsToReplaceForAddLastRows(heading)
					} else {
						_, err = p.ColumnsToReplaceForAddLastRows(heading)
					}
				case "prepare bounds":
					if kind == "float" {
						err = f.ReplaceColumnWithOnlyChangeAtLastRows(5, 0)
					} else {
						err = p.ReplaceColumnWithOnlyChangeAtLastRows(5, 0)
					}
				default:
					if kind == "float" {
						err = f.AddLastRowsToB(heading, columns)
					} else {
						err = p.AddLastRowsToB(heading, columns)
					}
				}
				if err != want {
					t.Fatal(phase, err, want)
				}
				if phase == "degenerated" {
					var status LUStatus
					var count uint32
					if kind == "float" {
						status = f.GetStatus()
						count = f.RefactorCounter
					} else {
						status = p.GetStatus()
						count = p.RefactorCounter
					}
					if status != LUDegenerated || count != 1 {
						t.Fatal("degeneration break", status, count)
					}
				}
				if phase == "closed" {
					if kind == "float" {
						_, err = f.ColumnsToReplaceForAddLastRows(heading)
						if err != ErrLUClosed {
							t.Fatal(err)
						}
						err = f.AdjustDimensionWithMatrixA()
						if err != ErrLUClosed {
							t.Fatal(err)
						}
						err = f.ReplaceColumnWithOnlyChangeAtLastRows(0, 0)
					} else {
						_, err = p.ColumnsToReplaceForAddLastRows(heading)
						if err != ErrLUClosed {
							t.Fatal(err)
						}
						err = p.AdjustDimensionWithMatrixA()
						if err != ErrLUClosed {
							t.Fatal(err)
						}
						err = p.ReplaceColumnWithOnlyChangeAtLastRows(0, 0)
					}
					if err != ErrLUClosed {
						t.Fatal(err)
					}
				}

				if phase == "extension work" {
					if f.Dim != 2 || p.Dim != 2 {
						t.Fatal("dimension prefix")
					}
				}
			})
		}
	}
}

func TestLPExactLUExtensionRetainsStoppedChildren(t *testing.T) {
	for _, child := range []string{"U", "Q", "R"} {
		t.Run(child, func(t *testing.T) {
			reached := false
			for ordinal := int64(0); ordinal < 100 && !reached; ordinal++ {
				t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
					h := lpNewDirectSparseHarness(t)
					a := lpExactLUStatic(t, h, lpExactIdentity(1), []uint32{0})
					var p *PairLU
					var state *exactLUState
					var err error

					p, err = NewPairLU(a, []uint32{0}, lpFloatLUSettings(math.Inf(1)))
					state = p.exactLUState

					h.must(err)
					defer func() {
						h.must(p.Close())
					}()
					v := h.pair("1"+strings.Repeat("0", 100)+"/7", "2"+strings.Repeat("0", 100)+"/13")
					var buffer *IndexedRationalVector
					if child == "Q" {
						buffer = p.Q.tBuffer
					} else if child == "R" {
						buffer = p.R.tBuffer
					}
					if buffer != nil {
						h.must(buffer.SetValue(&v.X, 0))
					}
					h.budget.FailAfter(ordinal)
					if child == "U" {
						err = state.U.AddNewElement(0, 0, &v.X)
					} else {
						err = buffer.Add(0, &v.Y)
					}
					h.budget.FailAfter(-1)
					stopped := state.U.work != nil || state.U.calls.pending()
					if buffer != nil {
						stopped = buffer.calls.pending()
					}
					if stopped {
						reached = true
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						h.must(a.AddRow())

						err = p.AdjustDimensionWithMatrixA()

						if err != ErrExactVectorPending || state.Dim != 2 || !state.stopped {
							t.Fatal("child failure not retained", child, err, state.Dim)
						}
						if child != "U" && state.U.Dimension() != 2 {
							t.Fatal("U extension precedes permutation stop")
						}
						if child == "R" {
							var qsize uint32

							qsize = p.Q.Size()

							if qsize != 2 {
								t.Fatal("Q resize precedes R stop")
							}
						}
					}

					h.must(p.Close())

					lpExactVectorNamedCensus(t, h, lpExactLUStaticCells(a))
				})
			}
			if !reached {
				t.Fatal("no genuine stopped child", child)
			}
		})
	}
}
