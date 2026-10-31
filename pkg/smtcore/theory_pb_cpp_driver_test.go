package smtcore

// Source: src/test/theory_pb.cpp, constant-literal cardinality matrix.

import (
	"math/bits"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestPseudoBooleanTheorySourceConstantCardinalities(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterPseudoBooleanPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FD", SMTSolverConfig{Params: params})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, solver.Close())
		astTestMust(t, params.Close())
	})
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	for n := 4; n < 11; n++ {
		args := make([]ast.ASTHandle, n)
		for mask := 0; mask < 1<<n; mask++ {
			for i := range args {
				args[i] = falsity
				if mask&(1<<i) != 0 {
					args[i] = truth
				}
			}
			count := uint32(bits.OnesCount(uint(mask)))
			for offset := uint32(0); offset <= 1; offset++ {
				constraint, err := m.MkPseudoBooleanAtLeast(args, count+offset)
				astTestMust(t, err)
				astTestMust(t, solver.Push())
				astTestMust(t, solver.Assert(constraint))
				status, err := solver.CheckSat(nil)
				astTestMust(t, err)
				want := lbool.True
				if offset == 1 {
					want = lbool.False
				}
				if status != want {
					t.Fatalf("n=%d mask=%d bound=%d: got %v want %v", n, mask, count+offset, status, want)
				}
				astTestMust(t, solver.Pop(1))
			}
		}
	}
}
