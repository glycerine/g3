package lp

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestLARSignatureExactPositionIndependentModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 500; world++ {
		h := newLPPairHarness(t)
		f := lpNewPairCoreFixture(t, h, 0, 1)
		f.settings.SimplexStrategy = StrategyTableauRows
		kinds := []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnFixed, ColumnBoxed}
		kind := kinds[world%5]
		f.types[0] = kind
		lo, up := big.NewRat(int64(world%17-8), 7), big.NewRat(int64(world%17-8), 7)
		if kind != ColumnFixed {
			up.Add(up, big.NewRat(2, 1))
		}
		x := new(big.Rat).Set(lo)
		epsilon := new(big.Rat)
		switch world / 5 % 5 {
		case 1:
			x.Set(up)
		case 2:
			x.Add(x, big.NewRat(1, 1))
		case 3:
			epsilon.SetFrac64(1, 13)
		case 4:
			epsilon.SetFrac64(-1, 13)
		}
		h.must(f.low.Data[0].Copy(h.m, h.pair(lo.RatString(), "0")))
		h.must(f.up.Data[0].Copy(h.m, h.pair(up.RatString(), "0")))
		h.must(f.x.Data[0].Copy(h.m, h.pair(x.RatString(), epsilon.RatString())))
		m, err := NewPairPrimalCoreSolver(f.input())
		h.must(err)
		want := PositionNotAtBound
		if kind == ColumnFree {
			want = PositionFreeOfBounds
		} else if x.Cmp(lo) == 0 && epsilon.Sign() == 0 && kind != ColumnUpperBound {
			want = PositionAtLowBound
			if kind == ColumnFixed {
				want = PositionAtFixed
			}
		} else if x.Cmp(up) == 0 && epsilon.Sign() == 0 && (kind == ColumnUpperBound || kind == ColumnBoxed) {
			want = PositionAtUpperBound
		}
		got, err := m.GetNonBasicColumnValuePosition(0)
		h.must(err)
		if got != want {
			t.Fatal(world, kind, got, want)
		}
		h.must(m.Close())
		f.close()
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARSignatureExtractionIndependentModels(t *testing.T) {
	for world := 0; world < 120; world++ {
		h := newLPPairHarness(t)
		c := lpNewCoreScopeFixture(t, h)
		c.Bounds.Types.values[1], c.Bounds.Types.values[2] = ColumnBoxed, ColumnUpperBound
		h.must(c.Bounds.Upper.Set(1, h.pair("2", "0")))
		h.must(c.Bounds.Upper.Set(2, h.pair("3", "0")))
		c.DUpper[1], c.DUpper[2] = 2, 3
		v := int64(world % 3)
		h.must(c.RX.Data[1].Copy(h.m, h.pair(fmt.Sprint(v), "0")))
		h.must(c.RX.Data[2].Copy(h.m, h.pair("3", "0")))
		c.DX[1], c.DX[2] = float64(v), 3
		want := PositionNotAtBound
		if v == 0 {
			want = PositionAtLowBound
		} else if v == 2 {
			want = PositionAtUpperBound
		}
		signature := lpSignature(map[uint32]NonBasicColumnValuePosition{99: PositionFreeOfBounds, 0: PositionAtFixed})
		h.must(c.ExtractSignatureFromPairSolver(signature, c.RPrimal))
		if !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: want, 2: PositionAtUpperBound}) {
			t.Fatal("pair extraction", signature)
		}
		signature.Clear()
		signature.Set(88, PositionAtFixed)
		h.must(c.ExtractSignatureFromFloatSolver(signature, c.DPrimal))
		if !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: want, 2: PositionAtUpperBound}) {
			t.Fatal("float extraction", signature)
		}
		h.must(c.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARSignaturePreparationIndependentSystems(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 160; world++ {
		for _, floating := range []bool{false, true} {
			h := newLPPairHarness(t)
			c := lpNewCoreScopeFixture(t, h)
			h.must(c.PrefixR())
			h.must(c.PrefixD())
			c.settings.RandomSeed(uint32(world + 1))
			rng := z3rand.New(uint32(world + 1))
			kinds := []ColumnType{ColumnLowBound, ColumnUpperBound, ColumnFixed, ColumnBoxed}
			positions := []NonBasicColumnValuePosition{PositionAtLowBound, PositionAtUpperBound, PositionAtFixed, PositionFreeOfBounds, PositionNotAtBound}
			signature := NewLARSolutionSignature()
			realValues, epsValues := make([]*big.Rat, 3), make([]*big.Rat, 3)
			for j := uint32(1); j < 3; j++ {
				kind := kinds[(world+int(j))%4]
				c.Bounds.Types.values[j] = kind
				low, up := big.NewRat(int64(world%13-6), 1), big.NewRat(int64(world%13+1), 1)
				ly, uy := big.NewRat(int64(world%7-3), 11), big.NewRat(int64(world%5-2), 13)
				h.must(c.Bounds.Low.Set(j, h.pair(low.RatString(), ly.RatString())))
				h.must(c.Bounds.Upper.Set(j, h.pair(up.RatString(), uy.RatString())))
				c.DLow[j], c.DUpper[j] = float64(world%13-6), float64(world%13+1)
				pos := positions[(world+int(j))%5]
				signature.Set(j, pos)
				side := 1
				switch pos {
				case PositionAtUpperBound, PositionAtFixed:
					side = 2
				case PositionFreeOfBounds:
					side = 0
				case PositionNotAtBound:
					if kind == ColumnUpperBound {
						side = 2
					} else if kind == ColumnBoxed {
						if rng.Next()%2 == 0 {
							side = 2
						}
					}
				}
				if side == 0 {
					realValues[j], epsValues[j] = new(big.Rat), new(big.Rat)
				} else if side == 1 {
					realValues[j], epsValues[j] = low, ly
				} else {
					realValues[j], epsValues[j] = up, uy
				}
			}
			var err error
			if floating {
				err = c.PrepareFloatSolverXWithSignature(signature, c.DPrimal)
			} else {
				err = c.PreparePairSolverXWithSignature(signature, c.RPrimal)
			}
			h.must(err)
			if c.settings.RandomNext() != rng.Next() {
				t.Fatal("RNG", world, floating)
			}
			basic := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), realValues[1]), new(big.Rat).Mul(big.NewRat(3, 1), realValues[2])))
			basicEps := new(big.Rat).Neg(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), epsValues[1]), new(big.Rat).Mul(big.NewRat(3, 1), epsValues[2])))
			if floating {
				if c.DX[0] != float64(-2*realValues[1].Num().Int64()-3*realValues[2].Num().Int64()) {
					t.Fatal("floating reconstruction", world, c.DX)
				}
			} else {
				h.check(&c.RX.Data[0], basic, basicEps)
				for j := 1; j < 3; j++ {
					h.check(&c.RX.Data[j], realValues[j], epsValues[j])
				}
			}
			h.must(c.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARSignatureReachedPrefixes(t *testing.T) {
	for _, floating := range []bool{false, true} {
		for _, phase := range []string{"nil-signature", "nil-solver", "stopped", "extract-short-types", "prepare-short-x", "prepare-short-low", "prepare-short-upper", "prepare-nonzero-rhs", "prepare-basic-heading", "prepare-invalid-position", "prepare-free-interior", "prepare-no-lu"} {
			t.Run(fmt.Sprintf("%t/%s", floating, phase), func(t *testing.T) {
				h := newLPPairHarness(t)
				c := lpNewCoreScopeFixture(t, h)
				h.must(c.PrefixR())
				h.must(c.PrefixD())
				signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound, 2: PositionAtUpperBound})
				h.must(c.Bounds.Low.Set(1, h.pair("7", "0")))
				h.must(c.Bounds.Upper.Set(2, h.pair("11", "0")))
				c.DLow[1], c.DUpper[2] = 7, 11
				var err error
				savedTypes, savedX, savedLow, savedUp := c.Bounds.Types.values, c.RX.Data, c.Bounds.Low.values, c.Bounds.Upper.values
				savedDX, savedDLow, savedDUp := c.DX, c.DLow, c.DUpper
				extract := false
				succeeds := false
				switch phase {
				case "nil-signature":
					extract = true
					if floating {
						err = c.ExtractSignatureFromFloatSolver(nil, c.DPrimal)
					} else {
						err = c.ExtractSignatureFromPairSolver(nil, c.RPrimal)
					}
				case "nil-solver":
					extract = true
					if floating {
						err = c.ExtractSignatureFromFloatSolver(signature, nil)
					} else {
						err = c.ExtractSignatureFromPairSolver(signature, nil)
					}
				case "stopped":
					c.stopped = true
				case "extract-short-types":
					extract = true
					c.Bounds.Types.values = c.Bounds.Types.values[:2]
					signature.Set(99, PositionAtFixed)
					if floating {
						err = c.ExtractSignatureFromFloatSolver(signature, c.DPrimal)
					} else {
						err = c.ExtractSignatureFromPairSolver(signature, c.RPrimal)
					}
				case "prepare-short-x":
					if floating {
						c.DX = c.DX[:2]
					} else {
						c.RX.Data = c.RX.Data[:2]
					}
				case "prepare-short-low":
					if floating {
						c.DLow = nil
					} else {
						c.Bounds.Low.values = nil
					}
				case "prepare-short-upper":
					if floating {
						c.DUpper = nil
					} else {
						c.Bounds.Upper.values = nil
					}
				case "prepare-nonzero-rhs":
					h.must(c.RightSides.Data[0].Copy(h.m, h.pair("3", "0")))
					c.DRightSides[0] = 3
					succeeds = !sourceDebugEnabled
				case "prepare-basic-heading":
					c.RHeading[1] = 0
					succeeds = !sourceDebugEnabled
				case "prepare-invalid-position":
					signature.Set(2, NonBasicColumnValuePosition(99))
					succeeds = !sourceDebugEnabled
				case "prepare-free-interior":
					c.Bounds.Types.values[2] = ColumnFree
					signature.Set(2, PositionNotAtBound)
					succeeds = !sourceDebugEnabled
				case "prepare-no-lu":
					if floating {
						h.must(c.DPrimal.Factorization.Close())
						c.DPrimal.Factorization = nil
					} else {
						h.must(c.RPrimal.Factorization.Close())
						c.RPrimal.Factorization = nil
					}
				}
				if !extract {
					if floating {
						err = c.PrepareFloatSolverXWithSignature(signature, c.DPrimal)
					} else {
						err = c.PreparePairSolverXWithSignature(signature, c.RPrimal)
					}
				}
				if (err == nil) != succeeds {
					t.Fatal("result", err, succeeds)
				}
				if phase == "extract-short-types" {
					if !lpSignatureEqual(signature, map[uint32]NonBasicColumnValuePosition{1: PositionFreeOfBounds}) {
						t.Fatal("clear and extraction prefix", signature)
					}
				}
				// libstdc++ visits column 2 before column 1 (AUDIT.md
				// util_lp_03-F1), so a failure on column 2 leaves column 1
				// unwritten.
				// The debug-only column 2 assertions stop the walk the same way.
				column2Fails := phase == "prepare-short-x" || phase == "prepare-short-upper" || sourceDebugEnabled && (phase == "prepare-invalid-position" || phase == "prepare-free-interior")
				if column2Fails {
					if floating {
						if c.DX[1] != 0 {
							t.Fatal("floating prefix", c.DX)
						}
					} else {
						h.check(&c.RX.Data[1], new(big.Rat), new(big.Rat))
					}
				}
				if !column2Fails && (phase == "prepare-no-lu" || phase == "prepare-nonzero-rhs" || phase == "prepare-invalid-position" || phase == "prepare-free-interior") {
					if floating {
						if c.DX[1] != 7 {
							t.Fatal("floating prefix", c.DX)
						}
					} else {
						h.check(&c.RX.Data[1], big.NewRat(7, 1), new(big.Rat))
					}
				}
				if !succeeds {
					if c.PrepareFloatSolverXWithSignature(nil, c.DPrimal) != ErrExactVectorPending {
						t.Fatal("stopped owner retry")
					}
				}
				c.Bounds.Types.values, c.RX.Data, c.Bounds.Low.values, c.Bounds.Upper.values = savedTypes, savedX, savedLow, savedUp
				c.DX, c.DLow, c.DUpper = savedDX, savedDLow, savedDUp
				h.must(c.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

func TestLARSignaturePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		success, failures := false, 0
		for ordinal := int64(0); ordinal < 1000; ordinal++ {
			done := false
			t.Run(fmt.Sprintf("%t/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				c := lpNewCoreScopeFixture(t, h)
				h.must(c.PrefixR())
				huge := strings.Repeat("1234567890", 20) + "7/13"
				h.must(c.Bounds.Low.Set(1, h.pair(huge, "0")))
				h.must(c.Bounds.Upper.Set(2, h.pair(huge, "0")))
				signature := lpSignature(map[uint32]NonBasicColumnValuePosition{1: PositionAtLowBound, 2: PositionAtUpperBound})
				h.budget.FailAfter(ordinal)
				err := c.PreparePairSolverXWithSignature(signature, c.RPrimal)
				h.budget.FailAfter(-1)
				if err == nil {
					success, done = true, true
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					if c.PreparePairSolverXWithSignature(signature, c.RPrimal) != ErrExactVectorPending {
						t.Fatal("stopped retry")
					}
				}
				h.must(c.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
			if done {
				break
			}
		}
		if !success || (direct && failures == 0) {
			t.Fatal("campaign", direct, success, failures)
		}
	}
}

// lpSignature builds a signature by inserting its keys in ascending order,
// the order extract_signature_from_lp_core_solver inserts nonbasic columns.
func lpSignature(positions map[uint32]NonBasicColumnValuePosition) *LARSolutionSignature {
	keys := make([]uint32, 0, len(positions))
	for j := range positions {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })
	s := NewLARSolutionSignature()
	for _, j := range keys {
		s.Set(j, positions[j])
	}
	return s
}

// lpSignatureEqual compares a signature's key/position pairs with want.
func lpSignatureEqual(s *LARSolutionSignature, want map[uint32]NonBasicColumnValuePosition) bool {
	return reflect.DeepEqual(s.Positions(), want)
}

// lpLARSignatureControlOrders reads the libstdc++ output recorded from
// src/test/stl_controls/stl_lar_signature_order_control.cpp:
// tag -> "<bucket_count> <k1> <k2> ...".
func lpLARSignatureControlOrders(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile("test_vectors/libstdcxx_lar_signature_order.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "order" {
			want[fields[1]] = strings.Join(fields[2:], " ")
		}
	}
	return want
}

func lpLARSignatureOrderLine(s *LARSolutionSignature) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d", s.order.BucketCount())
	for _, j := range s.Keys() {
		fmt.Fprintf(&b, " %d", j)
	}
	return b.String()
}

// AUDIT.md util_lp_03-F1: lar_solution_signature is a
// std::unordered_map<unsigned, ...>; extract_signature_from_lp_core_solver
// clears it (keeping the buckets) and inserts ascending columns, and
// prepare_solver_x_with_signature walks it in libstdc++ node order. The
// expected orders come only from the g++ 13 control program; never edit
// them by hand.
func TestLARSignatureIteratesInLibstdcxxOrder(t *testing.T) {
	want := lpLARSignatureControlOrders(t)
	fill := func(s *LARSolutionSignature, keys []uint32) {
		s.Clear()
		for _, j := range keys {
			s.Set(j, PositionNotAtBound)
		}
	}
	check := func(tag string, s *LARSolutionSignature) {
		t.Helper()
		w, ok := want[tag]
		if !ok {
			t.Fatalf("control output has no %s", tag)
		}
		if got := lpLARSignatureOrderLine(s); got != w {
			t.Fatalf("%s:\n got %s\nwant %s", tag, got, w)
		}
	}
	var third []uint32
	for j := uint32(0); j < 60; j += 3 {
		third = append(third, j)
	}
	s := NewLARSolutionSignature()
	fill(s, []uint32{0, 2, 3, 5, 7, 8, 11, 12, 20, 21, 22, 23, 30})
	check("first", s)
	if got := fmt.Sprint(s.Keys()); got != "[30 23 22 12 11 21 8 20 7 5 3 2 0]" {
		t.Fatal("first order", got)
	}
	fill(s, []uint32{1, 2, 4, 6, 9, 13, 14, 15, 16, 30, 31})
	check("cleared-second", s)
	fill(s, third)
	check("cleared-third", s)
	fill(s, []uint32{4, 9})
	check("cleared-small", s)
	fresh := NewLARSolutionSignature()
	fill(fresh, []uint32{4, 9})
	check("fresh-small", fresh)
	boxed := NewLARSolutionSignature()
	fill(boxed, []uint32{1, 2, 3, 4, 5})
	check("fresh-boxed", boxed)
}

// AUDIT.md util_lp_03-F1: with several boxed not_at_bound columns, each
// random bit of prepare_solver_x_with_signature lands on the column the
// libstdc++ iteration order visits next (5 4 3 2 1 for columns 1..5, from
// the control's fresh-boxed line), not on ascending columns.
func TestLARSignaturePreparationDrawsInLibstdcxxOrder(t *testing.T) {
	order := strings.Fields(lpLARSignatureControlOrders(t)["fresh-boxed"])[1:]
	for seed := uint32(1); seed <= 12; seed++ {
		h := newLPPairHarness(t)
		s, err := NewSettings()
		h.must(err)
		s.SimplexStrategy = StrategyLU
		c, err := NewLARCoreSolver(h.m, s, nil)
		h.must(err)
		lpCoreScopeAddColumn(t, h, c, true)
		for j := uint32(1); j <= 5; j++ {
			lpCoreScopeAddColumn(t, h, c, false)
			h.must(c.RA.Set(0, j, &h.pair(fmt.Sprint(j), "0").X))
			h.must(c.DA.Set(0, j, float64(j)))
		}
		h.must(InitPairFactorization(&c.RPrimal.Factorization, c.RA, c.RBasis, s))
		h.must(InitFloatFactorization(&c.DPrimal.Factorization, c.DA, c.DBasis, s))
		h.must(c.PrefixR())
		h.must(c.PrefixD())
		for j := uint32(1); j <= 5; j++ {
			c.Bounds.Types.values[j] = ColumnBoxed
			c.DLow[j], c.DUpper[j] = -float64(j), float64(10*j)
		}
		c.settings.RandomSeed(seed)
		rng := z3rand.New(seed)
		signature := NewLARSolutionSignature()
		for j := uint32(1); j <= 5; j++ {
			signature.Set(j, PositionNotAtBound)
		}
		want := make([]float64, 6)
		for _, text := range order {
			var j uint32
			fmt.Sscan(text, &j)
			if rng.Next()%2 != 0 {
				want[j] = -float64(j)
			} else {
				want[j] = float64(10 * j)
			}
		}
		h.must(c.PrepareFloatSolverXWithSignature(signature, c.DPrimal))
		for j := 1; j <= 5; j++ {
			if c.DX[j] != want[j] {
				t.Fatal("seed", seed, "column", j, "got", c.DX[1:], "want", want[1:])
			}
		}
		h.must(c.Close())
	}
}
