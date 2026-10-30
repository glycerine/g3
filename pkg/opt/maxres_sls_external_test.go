package opt_test

// Source: src/opt/opt_context.cpp enable_sls and src/opt/pb_sls.cpp.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func runMaxResSLSSwitch(t *testing.T, enabled bool) opt.MaxSMTStatistics {
	t.Helper()
	manager, evaluator := sortMaxFixture(t)
	key := "enable_sls"
	if err := evaluator.Context.SetBoolName(evaluator.Params, &key, enabled); err != nil {
		t.Fatal(err)
	}
	p := sortMaxBool(t, manager, "maxres-sls-p")
	q := sortMaxBool(t, manager, "maxres-sls-q")
	for _, expression := range []ast.ASTHandle{p, q} {
		if err := manager.IncRef(expression); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(expression)
	}
	both, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	mutex, err := manager.MkNot(both)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []ast.ASTHandle{both, mutex} {
		if err = manager.IncRef(expression); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(expression)
	}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(mutex); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q}, []*big.Rat{big.NewRat(5, 1), big.NewRat(3, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal || len(result.MaxSMT) != 1 || result.Values[0].SoftCost.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("MaxRes SLS result", result.Status, result.Optimal, result.Values, result.MaxSMT)
	}
	return result.MaxSMT[0]
}

func TestMaxResEnableSLSRunsValidatedCandidateSearch(t *testing.T) {
	statistics := runMaxResSLSSwitch(t, true)
	if statistics.Engine != "maxres" || statistics.SLSAttempts != 1 || statistics.SLSChecks != 1 {
		t.Fatal("enabled SLS statistics", statistics)
	}
}

func TestMaxResDisableSLSDoesNotRunCandidateSearch(t *testing.T) {
	statistics := runMaxResSLSSwitch(t, false)
	if statistics.SLSAttempts != 0 || statistics.SLSChecks != 0 || statistics.SLSFlips != 0 || statistics.SLSImprovements != 0 {
		t.Fatal("disabled SLS statistics", statistics)
	}
}
