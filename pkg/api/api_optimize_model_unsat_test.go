package api

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestAPIOptimizeModelAfterUnsatIsEmpty(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	falsity := f.pin(f.m.MkFalse())
	if err := f.opt.Assert(falsity); err != nil {
		t.Fatal(err)
	}
	if _, err := f.opt.Minimize(f.x); err != nil {
		t.Fatal(err)
	}
	status, err := f.opt.Check()
	if err != nil || status != lbool.False {
		t.Fatal(status, err)
	}
	model, err := f.opt.Model()
	if err != nil {
		t.Fatal(err)
	}
	defer model.Close()
	if count, countErr := model.NumConstants(); countErr != nil || count != 0 {
		t.Fatal(count, countErr)
	}
}
