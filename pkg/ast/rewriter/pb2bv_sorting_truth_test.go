package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp MiniSat+ sorting profile.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVSortingProfileIndependentTruthOracle(t *testing.T) {
	for _, equality := range []bool{false, true} {
		for k := int64(0); k <= 9; k++ {
			m, r := pb2BVFixture(t)
			pb2BVSetProfile(t, r, "pb.solver", "sorting")
			args := []ast.ASTHandle{goalTestAtom(t, m, "sort-a"), goalTestAtom(t, m, "sort-b"), goalTestAtom(t, m, "sort-c")}
			values := pbTestRationals(t, m, "1", "3", "5", big.NewInt(k).String())
			var formula ast.ASTHandle
			var e error
			if equality {
				formula, e = m.MkPseudoBooleanEQ(values[:3], args, &values[3])
			} else {
				formula, e = m.MkPseudoBooleanLE(values[:3], args, &values[3])
			}
			astTestMust(t, e)
			out := astTestPin(t, m, ast.ASTHandle{})
			astTestMust(t, r.Rewrite(formula, out, ast.ASTRef{}))
			result, e := m.RefGet(out)
			astTestMust(t, e)
			if result == formula {
				t.Fatal("sorting profile retained weighted formula", equality, k)
			}
			clauses, e := r.FlushSideConstraints()
			astTestMust(t, e)
			oracle := pb2BVBooleanOracle{ids: map[ast.ASTHandle]int{}}
			rid := oracle.gate(t, m, result)
			for _, clause := range clauses {
				oracle.clauses = append(oracle.clauses, []int{oracle.gate(t, m, clause)})
			}
			for _, arg := range args {
				oracle.gate(t, m, arg)
			}
			for assignment := 0; assignment < 8; assignment++ {
				weights := []int64{1, 3, 5}
				var sum int64
				values := make([]int8, oracle.count+1)
				for i, arg := range args {
					values[oracle.ids[arg]] = -1
					if assignment&(1<<i) != 0 {
						values[oracle.ids[arg]] = 1
						sum += weights[i]
					}
				}
				want := sum <= k
				if equality {
					want = sum == k
				}
				positive := append([]int8(nil), values...)
				positive[rid] = 1
				if got := pb2BVOracleSAT(oracle.clauses, positive); got != want {
					t.Fatal("sorting truth", equality, k, assignment, sum, got, want)
				}
				negative := append([]int8(nil), values...)
				negative[rid] = -1
				if got := pb2BVOracleSAT(oracle.clauses, negative); got == want {
					t.Fatal("sorting complement", equality, k, assignment, sum, got, want)
				}
			}
			for _, clause := range clauses {
				astTestMust(t, m.DecRef(clause))
			}
		}
	}
}
