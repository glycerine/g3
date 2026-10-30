package api

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestAPIOptimizeModelSnapshotSurvivesLaterOptimization(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if err := f.opt.Assert(f.lower); err != nil {
		t.Fatal(err)
	}
	if err := f.opt.Assert(f.upper); err != nil {
		t.Fatal(err)
	}
	if _, err := f.opt.Maximize(f.x); err != nil {
		t.Fatal(err)
	}
	if status, err := f.opt.Check(); err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	first, err := f.opt.Model()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if err = f.opt.Push(); err != nil {
		t.Fatal(err)
	}
	if err = f.opt.Assert(f.lower); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := f.opt.Check(); checkErr != nil || status != lbool.True {
		t.Fatal(status, checkErr)
	}
	declaration, err := f.m.GetDecl(f.x)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.ConstantInterpretation(declaration); err != nil {
		t.Fatal("snapshot retired", err)
	}
}
