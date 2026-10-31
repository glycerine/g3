package clp_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/muz/clp"
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
