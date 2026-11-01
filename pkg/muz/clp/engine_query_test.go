package clp_test

import (
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCLPBoundedRecursiveQuery(t *testing.T) {
	f := newFixture(t)
	e, x := clp.NewEngine(f.m, f.set, f.solver, 20)
	if x != nil {
		t.Fatal(x)
	}
	defer e.Close()
	r, x := e.Query(f.app["Q"])
	if x != nil || r != lbool.True {
		t.Fatal(r, x)
	}
	if f.solver.pushes != 2 || f.solver.pops != 2 {
		t.Fatal(f.solver.pushes, f.solver.pops)
	}
	s, _ := e.Statistics()
	if s != (clp.Statistics{}) {
		t.Fatal(s)
	}
}

func TestCLPStatisticsRemainZeroAfterRejectedRule(t *testing.T) {
	f := newFixture(t)
	f.solver.checks = []lbool.LBool{lbool.False}
	engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	result, err := engine.Query(f.app["Q"])
	if err != nil || result != lbool.False {
		t.Fatal("rejected rule", result, err)
	}
	stats, err := engine.Statistics()
	if err != nil || stats != (clp.Statistics{}) {
		t.Fatal("source CLP counters", stats, err)
	}
	if err = engine.ResetStatistics(); err != nil {
		t.Fatal(err)
	}
	stats, err = engine.Statistics()
	if err != nil || stats != (clp.Statistics{}) {
		t.Fatal("reset source counters", stats, err)
	}
}
