package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeObjectsShareParentInterruptSlot(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	other, err := f.c.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	scope, err := newAPIOptimizeInterruptScope(f.opt)
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	status, err := other.Check()
	if status != lbool.Undef || !errors.Is(err, errSMTContextInterruptScope) {
		t.Fatal(status, err)
	}
}
