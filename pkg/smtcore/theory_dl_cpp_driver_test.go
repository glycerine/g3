package smtcore

// Source: src/test/theory_dl.cpp, successive strict finite-domain constraints.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogTheorySourceIncrementalOrder(t *testing.T) {
	m, modelConfig := smtModelGeneratorFixture(t)
	sort, err := m.MkDatalogFiniteSort(astTestSymbol(t, m, "source-S"), 111)
	astTestMust(t, err)
	solver, err := NewSMTSolver(m, nil, "QF_FD", SMTSolverConfig{Params: modelConfig.Context, ModelEvaluator: modelConfig})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.SetProduceModels(true))
	var vars [3]ast.ASTHandle
	for i, name := range []string{"a", "b", "c"} {
		vars[i], err = m.MkConst(astTestSymbol(t, m, "source-dl-"+name), sort)
		astTestMust(t, err)
	}
	for stage, pair := range [][2]int{{0, 1}, {1, 2}, {2, 0}} {
		less, err := m.MkDatalogLT(vars[pair[0]], vars[pair[1]])
		astTestMust(t, err)
		astTestMust(t, solver.Assert(less))
		status, err := solver.CheckSat(nil)
		astTestMust(t, err)
		if stage == 2 {
			if status != lbool.False {
				t.Fatal("cyclic order", status)
			}
			continue
		}
		if status != lbool.True {
			t.Fatal("incremental order", stage, status)
		}
		model, err := solver.AcquireModel()
		astTestMust(t, err)
		if model == nil {
			t.Fatal("missing model", stage)
		}
		values := make([]uint64, stage+2)
		for i := range values {
			declaration, err := m.GetDecl(vars[i])
			astTestMust(t, err)
			value, err := model.ConstantInterpretation(declaration)
			astTestMust(t, err)
			n, ok, err := m.DatalogNumeralValue(value)
			astTestMust(t, err)
			if !ok || n >= 111 {
				t.Fatal("invalid model value", stage, i, n, ok)
			}
			values[i] = n
		}
		astTestMust(t, model.DecRef())
		for i := 1; i < len(values); i++ {
			if values[i-1] >= values[i] {
				t.Fatal("model violates strict ordering", stage, values)
			}
		}
	}
}
