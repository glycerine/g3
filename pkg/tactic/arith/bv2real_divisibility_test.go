package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:298-343.

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestBV2RealDivisibilityProjectLoopIndependentOracle(t *testing.T) {
	for _, width := range []uint32{1, 4, 9} {
		for _, power := range []uint32{0, 1, 3, 9, 17, 65, 521} {
			for _, odd := range []int64{1, 3} {
				if power == 0 && odd == 1 {
					continue
				}
				t.Run(fmt.Sprintf("width=%d/power=%d/odd=%d", width, power, odd), func(t *testing.T) {
					u, f := bv2RealFixture(t, 2048)
					value := uint32(13) & ((1 << width) - 1)
					h := f.bv(t, value, width)
					factor := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(odd), uint(power)))
					c := &bv2RealCall{u: u}
					t.Cleanup(func() { equationMust(t, c.close()) })
					outer := c.number()
					equationMust(t, c.err)
					equationMust(t, f.m.NumeralService().SetInt64(outer, 37))
					reduced, ok := c.divisible(h, factor)
					equationMust(t, c.err)
					if ok != (odd == 1) {
						t.Fatal("odd residual acceptance", ok)
					}
					gotWidth, err := f.m.BitVectorSize(reduced)
					equationMust(t, err)
					wantWidth := width
					if power > 0 {
						if power >= width {
							wantWidth = 1
						} else {
							wantWidth -= power
						}
					}
					if gotWidth != wantWidth {
						t.Fatal("coefficient width", gotWidth, wantWidth)
					}
					if power == 0 && reduced != h {
						t.Fatal("odd-only factor changed coefficient")
					}
					wantValue := int64(value)
					if power >= width {
						wantValue = 0
					} else {
						wantValue = int64(value >> power)
						if wantValue&(1<<(wantWidth-1)) != 0 {
							wantValue -= 1 << wantWidth
						}
					}
					if got := bv2RealSignedValue(t, f, reduced); got.Cmp(big.NewInt(wantValue)) != 0 {
						t.Fatal("reduced coefficient bits", got, wantValue)
					}
					side, err := u.SideConditions()
					equationMust(t, err)
					if power == 0 {
						if len(side) != 0 {
							t.Fatal("odd-only refusal published a side condition")
						}
					} else {
						if len(side) != 1 {
							t.Fatal("source divisibility side-condition count", len(side))
						}
						zeroLow := new(big.Int).Mod(new(big.Int).SetUint64(uint64(value)), new(big.Int).Lsh(big.NewInt(1), uint(power))).Sign() == 0
						f.wantBool(t, side[0], zeroLow)
					}
					text, err := f.m.NumeralService().String(outer)
					equationMust(t, err)
					if text != "37" || len(c.numbers) != 1 {
						t.Fatal("loop numeric scope escaped or destroyed caller owner", text, len(c.numbers))
					}
				})
			}
		}
	}
}

func TestBV2RealDivisibilityWideAllocationPrecedesSideConditions(t *testing.T) {
	for _, capDivisor := range []bool{false, true} {
		t.Run(fmt.Sprint(capDivisor), func(t *testing.T) {
			u, f := bv2RealFixture(t, 64)
			a, b := f.bv(t, 5, 7), f.bv(t, 6, 7)
			factor := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(3), 521))
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			divisor := new(big.Rat).Mul(factor, big.NewRat(4, 1))
			original := new(big.Rat).Set(divisor)
			f.m.Budget().FailAfter(0)
			var gotA, gotB ast.ASTHandle
			var ok bool
			if capDivisor {
				gotA, gotB, ok = c.alignDivisor(a, b, divisor)
			} else {
				gotA, ok = c.divisible(a, factor)
				gotB = b
			}
			f.m.Budget().FailAfter(-1)
			var failure umemory.Failure
			if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
				t.Fatal("wide numeric allocation failure not reached", c.err)
			}
			if ok || gotA != a || gotB != b || divisor.Cmp(original) != 0 || len(u.side) != 0 {
				t.Fatal("numeric failure published unfinished coefficient/divisor/side conditions")
			}
		})
	}
}

type bv2RealSecondDivisibilityTraceFailure struct {
	budget *arena.Budget
	output bytes.Buffer
	fired  bool
	target int
}

func (w *bv2RealSecondDivisibilityTraceFailure) Write(p []byte) (int, error) {
	n, err := w.output.Write(p)
	if !w.fired && strings.Count(w.output.String(), ".cpp:339") >= w.target {
		w.fired = true
		w.budget.FailAfter(0)
	}
	return n, err
}
func TestBV2RealDivisorCapStopsAfterCoefficientTraceFailure(t *testing.T) {
	for _, completed := range []int{1, 2} {
		t.Run(fmt.Sprint(completed), func(t *testing.T) {
			u, f := bv2RealFixture(t, 64)
			writer := &bv2RealSecondDivisibilityTraceFailure{budget: f.m.Budget(), target: completed}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("bv2real_rewriter")
			if !tags.ShouldTrace("bv2real_rewriter") {
				t.Skip("source TRACE macro erased")
			}
			equationMust(t, u.BindTrace(tags))
			a, b := f.bv(t, 4, 7), f.bv(t, 6, 7)
			divisor := big.NewRat(8, 1)
			c := &bv2RealCall{u: u}
			t.Cleanup(func() { equationMust(t, c.close()) })
			gotA, gotB, ok := c.alignDivisor(a, b, divisor)
			f.m.Budget().FailAfter(-1)
			if !writer.fired || !errors.Is(c.err, arena.ErrInjectedFailure) {
				t.Fatal("post-assignment trace operand failure not reached", c.err, writer.output.String())
			}
			if ok || gotA.IsNull() || gotA == a || gotB.IsNull() || divisor.Cmp(big.NewRat(8, 1)) != 0 || len(u.side) != completed {
				t.Fatal("trace error advanced past completed coefficients", ok, gotA, gotB, divisor, len(u.side))
			}
			if completed == 1 && gotB != b || completed == 2 && gotB == b {
				t.Fatal("second coefficient assignment order", gotB, b)
			}
			width, err := f.m.BitVectorSize(gotA)
			equationMust(t, err)
			if width != 6 {
				t.Fatal("first coefficient replacement lost", width)
			}
		})
	}
}

func TestBV2RealDivisibilityProjectAllocationFrontiers(t *testing.T) {
	for _, capDivisor := range []bool{false, true} {
		succeeded, failed := false, false
		for stop := int64(0); stop < 1024; stop++ {
			t.Run(fmt.Sprintf("cap=%v/%d", capDivisor, stop), func(t *testing.T) {
				u, f := bv2RealFixture(t, 64)
				a, b := f.bv(t, 5, 7), f.bv(t, 6, 7)
				factor := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 65))
				divisor := new(big.Rat).Mul(factor, big.NewRat(4, 1))
				original := new(big.Rat).Set(divisor)
				c := &bv2RealCall{u: u}
				t.Cleanup(func() { equationMust(t, c.close()) })
				f.m.Budget().FailAfter(stop)
				var gotA, gotB ast.ASTHandle
				var ok bool
				if capDivisor {
					gotA, gotB, ok = c.alignDivisor(a, b, divisor)
				} else {
					gotA, ok = c.divisible(a, factor)
					gotB = b
				}
				f.m.Budget().FailAfter(-1)
				if gotA.IsNull() || gotB.IsNull() {
					t.Fatal("failed assignment lost a coefficient")
				}
				if c.err == nil {
					succeeded = true
					wantSide := 1
					if capDivisor {
						wantSide = 2
					}
					if !ok || len(u.side) != wantSide {
						t.Fatal("completed loop lost source conditions", ok, len(u.side))
					}
					if capDivisor && divisor.Cmp(big.NewRat(4, 1)) != 0 {
						t.Fatal("completed cap did not publish divisor")
					}
				} else {
					failed = true
					var failure umemory.Failure
					if !errors.Is(c.err, arena.ErrInjectedFailure) && (!errors.As(c.err, &failure) || failure.Kind != umemory.FailureMemout || !f.m.NumeralService().RuntimeOwner().State().OutOfMemory) {
						t.Fatal(c.err)
					}
					if ok || divisor.Cmp(original) != 0 {
						t.Fatal("failed loop or coefficient published capped divisor")
					}
					if len(u.side) == 0 && (gotA != a || gotB != b) {
						t.Fatal("coefficient replacement preceded its side condition")
					}
					if capDivisor && len(u.side) < 2 && gotB != b {
						t.Fatal("second coefficient changed before its side condition")
					}
				}
				if len(c.numbers) != 0 {
					t.Fatal("loop numeric owners escaped helper return", len(c.numbers))
				}
			})
			if succeeded {
				break
			}
		}
		if !succeeded || !failed {
			t.Fatal("project numeric failure/success frontiers not both reached", capDivisor, failed, succeeded)
		}
	}
}
