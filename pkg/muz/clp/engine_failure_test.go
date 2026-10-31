package clp_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCLPSolverFailureBalancesScope(t *testing.T) {
	f := newFixture(t)
	stop := errors.New("stop")
	f.solver.fail = stop
	e, x := clp.NewEngine(f.m, f.set, f.solver, 20)
	if x != nil {
		t.Fatal(x)
	}
	defer e.Close()
	_, x = e.Query(f.app["Q"])
	if !errors.Is(x, stop) {
		t.Fatal(x)
	}
	if f.solver.pushes != 1 {
		t.Fatal(f.solver.pushes)
	}
}

func TestCLPSolverUnknownRaisesSourceErrorAndRestoresScope(t *testing.T) {
	f := newFixture(t)
	f.solver.checks = []lbool.LBool{lbool.Undef}
	engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	_, err = engine.Query(f.app["Q"])
	if !errors.Is(err, clp.ErrSolverUnknown) || err.Error() != "undef" {
		t.Fatal("solver unknown", err)
	}
	if f.solver.pushes != 1 || f.solver.pops != 1 {
		t.Fatal("unbalanced solver scope", f.solver.pushes, f.solver.pops)
	}
	result, err := engine.Query(f.app["Q"])
	if err != nil || result != lbool.True {
		t.Fatal("retry", result, err)
	}
}
