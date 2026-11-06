package api

// Source: src/test/no_overflow.cpp test_equiv.
import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestNoOverflowSourceSymbolicEquivalences(t *testing.T) {
	for _, width := range []uint32{1, 2, 7, 16} {
		for _, tc := range []struct {
			name                 string
			unsigned             bool
			under                bool
			ext                  uint32
			nonzero, signCompare bool
		}{
			{"add-overflow", true, false, 1, false, false}, {"add-underflow", false, true, 1, false, false},
			{"sub-overflow", false, false, 1, false, false}, {"sub-underflow", true, true, 2, false, true},
			{"mul-overflow", true, false, 0, false, false}, {"mul-underflow", false, true, 0, false, false},
			{"div-overflow", false, false, 1, true, false}, {"neg-overflow", false, false, 1, false, false},
		} {
			for _, signed := range []bool{true, false} {
				if !signed && !tc.unsigned {
					continue
				}
				t.Run(fmt.Sprintf("%s/%d/signed=%t", tc.name, width, signed), func(t *testing.T) {
					f := newSMTAPIFixture(t, "ALL", nil)
					s, err := f.c.NewSolver()
					astTestMust(t, err)
					defer func() { astTestMust(t, s.Close()) }()
					sort := f.pin(f.c.BitVectorSort(width))
					// Source mk_min/mk_max construct numerals, rather than the
					// separate API extrema macros (which build shift/not terms).
					minValue, maxValue := int64(0), int64(1<<width)-1
					if signed {
						minValue, maxValue = -int64(1<<(width-1)), int64(1<<(width-1))-1
					}
					min := f.pin(f.c.BitVectorValue(minValue, sort))
					max := f.pin(f.c.BitVectorValue(maxValue, sort))
					x := f.pin(f.c.Constant("x", sort))
					y := f.pin(f.c.Constant("y", sort))
					var pred ast.ASTHandle
					switch tc.name {
					case "add-overflow":
						pred = f.pin(f.c.BitVectorAddNoOverflow(x, y, signed))
					case "add-underflow":
						pred = f.pin(f.c.BitVectorAddNoUnderflow(x, y))
					case "sub-overflow":
						pred = f.pin(f.c.BitVectorSubNoOverflow(x, y))
					case "sub-underflow":
						pred = f.pin(f.c.BitVectorSubNoUnderflow(x, y, signed))
					case "mul-overflow":
						pred = f.pin(f.c.BitVectorMulNoOverflow(x, y, signed))
					case "mul-underflow":
						pred = f.pin(f.c.BitVectorMulNoUnderflow(x, y))
					case "div-overflow":
						pred = f.pin(f.c.BitVectorSDivNoOverflow(x, y))
					case "neg-overflow":
						pred = f.pin(f.c.BitVectorNegNoOverflow(x))
					}
					var nonzero ast.ASTHandle
					if tc.nonzero {
						zero := f.pin(f.c.BitVectorValue(0, sort))
						nonzero = f.pin(f.c.Not(f.pin(f.c.Equal(y, zero))))
					}
					ext := tc.ext
					if ext == 0 {
						ext = width
					}
					extend := func(h ast.ASTHandle) ast.ASTHandle {
						if signed {
							return f.pin(f.c.BitVectorSignExtend(ext, h))
						}
						return f.pin(f.c.BitVectorZeroExtend(ext, h))
					}
					lo, hi, a, b := extend(min), extend(max), extend(x), extend(y)
					var result ast.ASTHandle
					switch tc.name {
					case "add-overflow", "add-underflow":
						result = f.pin(f.c.BitVectorAdd(a, b))
					case "sub-overflow", "sub-underflow":
						result = f.pin(f.c.BitVectorSub(a, b))
					case "mul-overflow", "mul-underflow":
						result = f.pin(f.c.BitVectorMul(a, b))
					case "div-overflow":
						result = f.pin(f.c.BitVectorSDiv(a, b))
					case "neg-overflow":
						result = f.pin(f.c.BitVectorNeg(a))
					}
					left, right := result, hi
					if tc.under {
						left, right = lo, result
					}
					var bound ast.ASTHandle
					if signed || tc.signCompare {
						bound = f.pin(f.c.BitVectorSLE(left, right))
					} else {
						bound = f.pin(f.c.BitVectorULE(left, right))
					}
					equiv := f.pin(f.c.Iff(pred, bound))
					if tc.nonzero {
						equiv = f.pin(f.c.Implies(nonzero, equiv))
					}
					// Match test_equiv's push/assert/check/pop protocol. Push selects
					// the incremental SMT backend in the source combined solver.
					astTestMust(t, s.Push())
					astTestMust(t, s.Assert(f.pin(f.c.Not(equiv))))
					status, e := s.Check()
					astTestMust(t, e)
					if width == 16 && tc.ext == 0 {
						stats, e := s.Statistics()
						astTestMust(t, e)
						text, e := stats.String()
						astTestMust(t, e)
						t.Log("OVERFLOW-PATH Go", text)
						astTestMust(t, stats.Close())
					}
					if status != lbool.False {
						t.Fatal("non-equivalent overflow predicate", status)
					}
					astTestMust(t, s.Pop(1))
				})
			}
		}
	}
}
