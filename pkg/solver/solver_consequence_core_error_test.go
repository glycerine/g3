package solver

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var errFixtureCore = errors.New("fixture unsat core failure")

type coreFailingSolverFixture struct{ *solverFixture }

func (s coreFailingSolverFixture) UnsatCore() ([]ast.ASTHandle, error) {
	return nil, errFixtureCore
}

// AUDIT.md solver_misc_01-F2: an exception from get_unsat_core in the
// non-Boolean branch of get_consequences_core propagates.
func TestSolverDefaultConsequenceUnsatCoreErrorPropagates(t *testing.T) {
	m := solverTestManager(t)
	result, err := NewSimpleCheckSatResult(m)
	solverMust(t, err)
	base := &solverFixture{SimpleCheckSatResult: result, manager: m, statuses: []lbool.LBool{lbool.True, lbool.False}}
	t.Cleanup(func() { solverMust(t, result.Close()) })
	logic, err := m.Symbols.Intern("LIA")
	solverMust(t, err)
	_, err = m.RegisterArithmeticPlugin()
	solverMust(t, err)
	var names ast.ASTBuiltinNames
	defer names.Close()
	solverMust(t, m.GetArithmeticSortNames(&names, logic))
	value, err := m.ArithmeticNumeralInt64(int64(7), true)
	solverMust(t, err)
	integer, err := m.GetSort(value)
	solverMust(t, err)
	name, err := m.Symbols.Intern("solver-int")
	solverMust(t, err)
	variable, err := m.MkConst(name, integer)
	solverMust(t, err)
	md, err := model.NewModel(m)
	solverMust(t, err)
	solverMust(t, result.SetModel(md.Core()))
	status, _, err := DefaultGetConsequences(coreFailingSolverFixture{base}, nil, []ast.ASTHandle{variable}, func(*model.ModelCore) (ConsequenceEvaluator, error) {
		return &fixedConsequenceEvaluator{manager: m, value: value}, nil
	})
	if !errors.Is(err, errFixtureCore) || status != lbool.Undef {
		t.Fatalf("status %v, error %v; want the unsat core error", status, err)
	}
}
