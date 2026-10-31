package api

// Source: src/test/no_overflow.cpp binary boundary tests and TEST macros.
import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type overflowSourceCase struct {
	a, b            ast.ASTHandle
	noOver, noUnder bool
}

func TestNoOverflowSourceAdditionBounds(t *testing.T)    { testNoOverflowSourceBinaryBounds(t, "add") }
func TestNoOverflowSourceSubtractionBounds(t *testing.T) { testNoOverflowSourceBinaryBounds(t, "sub") }
func TestNoOverflowSourceMultiplicationBounds(t *testing.T) {
	testNoOverflowSourceBinaryBounds(t, "mul")
}

func testNoOverflowSourceBinaryBounds(t *testing.T, operation string) {
	for _, width := range []uint32{1, 16, 32, 42} {
		for _, signed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d/signed=%t", width, signed), func(t *testing.T) {
				f := newSMTAPIFixture(t, "ALL", nil)
				s, err := f.c.NewSolver()
				astTestMust(t, err)
				defer func() { astTestMust(t, s.Close()) }()
				sort := f.pin(f.c.BitVectorSort(width))
				x := f.pin(f.c.Constant("x", sort))
				y := f.pin(f.c.Constant("y", sort))
				zero := f.pin(f.c.BitVectorValue(0, sort))
				one := f.pin(f.c.BitVectorValue(1, sort))
				minusOne := f.pin(f.c.BitVectorValue(-1, sort))
				min := zero
				max := f.pin(f.c.BitVectorUnsignedMax(sort))
				if signed {
					min = f.pin(f.c.BitVectorSignedMin(sort))
					max = f.pin(f.c.BitVectorSignedMax(sort))
				}
				var overflow, underflow ast.ASTHandle
				var cases []overflowSourceCase
				switch operation {
				case "add":
					overflow = f.pin(f.c.BitVectorAddNoOverflow(x, y, signed))
					if signed {
						underflow = f.pin(f.c.BitVectorAddNoUnderflow(x, y))
					}
					cases = []overflowSourceCase{{zero, one, true, true}, {zero, max, true, true}, {one, max, width == 1 && signed, true}, {one, min, true, !(width == 1 && signed)}, {min, min, true, !signed}, {min, max, true, true}, {max, max, width == 1 && signed, true}}
					if signed {
						cases = append(cases, overflowSourceCase{minusOne, min, true, false})
					}
				case "sub":
					if signed {
						overflow = f.pin(f.c.BitVectorSubNoOverflow(x, y))
					}
					underflow = f.pin(f.c.BitVectorSubNoUnderflow(x, y, signed))
					cases = []overflowSourceCase{{zero, one, !(width == 1 && signed), signed}, {zero, max, true, signed}, {one, max, true, width == 1 || signed}, {max, one, !(width == 1 && signed), true}, {one, min, !(width != 1 && signed), true}, {min, one, true, width == 1 && signed}, {min, min, true, true}, {min, max, true, width == 1 && signed}, {max, min, !signed, true}, {max, max, true, true}}
					if signed {
						cases = append(cases, overflowSourceCase{minusOne, min, true, true}, overflowSourceCase{min, minusOne, true, true})
					}
				case "mul":
					overflow = f.pin(f.c.BitVectorMulNoOverflow(x, y, signed))
					if signed {
						underflow = f.pin(f.c.BitVectorMulNoUnderflow(x, y))
					}
					cases = []overflowSourceCase{{one, one, !(width == 1 && signed), true}, {zero, max, true, true}, {min, zero, true, true}, {max, one, true, true}, {one, min, !(width == 1 && signed), true}, {min, min, !signed, true}, {min, max, true, !(width != 1 && signed)}, {max, max, width == 1, true}}
					if signed {
						cases = append(cases, overflowSourceCase{minusOne, min, false, true}, overflowSourceCase{min, minusOne, false, true}, overflowSourceCase{minusOne, max, true, true})
					}
				}
				check := func(t *testing.T, predicate ast.ASTHandle, want bool) {
					t.Helper()
					if predicate.IsNull() {
						return
					}
					for _, positive := range []bool{true, false} {
						func() {
							astTestMust(t, s.Push())
							defer func() { astTestMust(t, s.Pop(1)) }()
							term := predicate
							if !positive {
								term = f.pin(f.c.Not(predicate))
							}
							astTestMust(t, s.Assert(term))
							status, e := s.Check()
							astTestMust(t, e)
							expected := lbool.False
							if positive == want {
								expected = lbool.True
							}
							if status != expected {
								t.Fatalf("%s positive=%v predicate=%v expected %v, got %v", operation, positive, predicate, expected, status)
							}
						}()
					}
				}
				for i, tc := range cases {
					astTestMust(t, s.Push())
					astTestMust(t, s.Assert(f.pin(f.c.Equal(x, tc.a))))
					astTestMust(t, s.Assert(f.pin(f.c.Equal(y, tc.b))))
					t.Run(fmt.Sprint("case-", i), func(t *testing.T) { check(t, overflow, tc.noOver); check(t, underflow, tc.noUnder) })
					astTestMust(t, s.Pop(1))
				}
			})
		}
	}
}
