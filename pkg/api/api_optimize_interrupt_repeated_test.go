package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeRepeatedChecksReinstallInterruptScope(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if err := f.opt.Assert(f.lower); err != nil {
		t.Fatal(err)
	}
	if _, err := f.opt.Minimize(f.x); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		status, err := f.opt.Check()
		if err != nil || status != lbool.True {
			t.Fatal(i, status, err)
		}
	}
}
