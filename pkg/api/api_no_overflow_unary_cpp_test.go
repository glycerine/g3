package api

// Source: src/test/no_overflow.cpp test_neg and test_div.
import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestNoOverflowSourceNegationAndDivisionBounds(t *testing.T) {
	for _, width := range []uint32{1, 16, 32, 42} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
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
			min := f.pin(f.c.BitVectorSignedMin(sort))
			max := f.pin(f.c.BitVectorSignedMax(sort))
			neg := f.pin(f.c.BitVectorNegNoOverflow(x))
			div := f.pin(f.c.BitVectorSDivNoOverflow(x, y))
			check := func(predicate ast.ASTHandle, want bool) {
				t.Helper()
				for _, positive := range []bool{true, false} {
					astTestMust(t, s.Push())
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
						t.Fatalf("expected %v, got %v", expected, status)
					}
					astTestMust(t, s.Pop(1))
				}
			}
			for _, tc := range []struct {
				a    ast.ASTHandle
				want bool
			}{{zero, true}, {one, width != 1}, {minusOne, width != 1}, {max, true}, {min, false}} {
				astTestMust(t, s.Push())
				astTestMust(t, s.Assert(f.pin(f.c.Equal(x, tc.a))))
				check(neg, tc.want)
				astTestMust(t, s.Pop(1))
			}
			for _, tc := range []struct {
				a, b      ast.ASTHandle
				want, any bool
			}{
				{one, one, width != 1, false}, {zero, one, true, false}, {zero, minusOne, true, false},
				{one, zero, false, true}, {minusOne, zero, false, true}, {zero, zero, false, true},
				{min, minusOne, false, false}, {min, one, width != 1, false}, {min, min, width != 1, false}, {max, min, true, false},
			} {
				astTestMust(t, s.Push())
				astTestMust(t, s.Assert(f.pin(f.c.Equal(x, tc.a))))
				astTestMust(t, s.Assert(f.pin(f.c.Equal(y, tc.b))))
				if tc.any {
					astTestMust(t, s.Push())
					astTestMust(t, s.Assert(div))
					_, err = s.Check()
					astTestMust(t, err)
					astTestMust(t, s.Pop(1))
				} else {
					check(div, tc.want)
				}
				astTestMust(t, s.Pop(1))
			}
		})
	}
}
