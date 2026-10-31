package api

// Source: src/test/no_overflow.cpp test_add and TEST macros.
import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestNoOverflowSourceAdditionBounds(t *testing.T) {
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
				overflow := f.pin(f.c.BitVectorAddNoOverflow(x, y, signed))
				var underflow ast.ASTHandle
				if signed {
					underflow = f.pin(f.c.BitVectorAddNoUnderflow(x, y))
				}
				cases := []struct {
					a, b            ast.ASTHandle
					noOver, noUnder bool
				}{
					{zero, one, true, true}, {zero, max, true, true}, {one, max, width == 1 && signed, true}, {one, min, true, !(width == 1 && signed)},
					{min, min, true, !signed}, {min, max, true, true}, {max, max, width == 1 && signed, true},
				}
				if signed {
					cases = append(cases, struct {
						a, b            ast.ASTHandle
						noOver, noUnder bool
					}{minusOne, min, true, false})
				}
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
				for i, tc := range cases {
					astTestMust(t, s.Push())
					astTestMust(t, s.Assert(f.pin(f.c.Equal(x, tc.a))))
					astTestMust(t, s.Assert(f.pin(f.c.Equal(y, tc.b))))
					t.Run(fmt.Sprint("case-", i), func(t *testing.T) {
						check(overflow, tc.noOver)
						if signed {
							check(underflow, tc.noUnder)
						}
					})
					astTestMust(t, s.Pop(1))
				}
			})
		}
	}
}
