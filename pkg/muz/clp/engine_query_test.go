package clp

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCLPBoundedRecursiveQuery(t *testing.T) {
	f := newFixture(t)
	e, x := NewEngine(f.m, f.set, f.solver, 20)
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
	if s.Unfold != 2 {
		t.Fatal(s)
	}
}
