package clp

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCLPDepthFrontierUnknown(t *testing.T) {
	f := newFixture(t)
	e, x := NewEngine(f.m, f.set, f.solver, 1)
	if x != nil {
		t.Fatal(x)
	}
	defer e.Close()
	r, x := e.Query(f.app["Q"])
	if x != nil || r != lbool.Undef {
		t.Fatal(r, x)
	}
	s, _ := e.Statistics()
	if s.NoUnfold != 1 {
		t.Fatal(s)
	}
}
