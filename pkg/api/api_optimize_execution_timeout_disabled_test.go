package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeZeroTimeoutRemainsDisabled(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, parameters.Close()) }()
	astTestMust(t, parameters.SetUInt("timeout", 0))
	astTestMust(t, f.opt.SetParameters(parameters))
	astTestMust(t, f.opt.Assert(f.lower))
	_, err = f.opt.Minimize(f.x)
	astTestMust(t, err)
	status, err := f.opt.Check()
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
}
