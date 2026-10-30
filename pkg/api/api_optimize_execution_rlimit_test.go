package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeLocalResourceLimitScopesRepeatedChecks(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, parameters.Close()) }()
	astTestMust(t, parameters.SetUInt("rlimit", 100000))
	astTestMust(t, f.opt.SetParameters(parameters))
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	_, err = f.opt.Maximize(f.x)
	astTestMust(t, err)
	for i := 0; i < 2; i++ {
		status, checkErr := f.opt.Check()
		if checkErr != nil || status != lbool.True {
			t.Fatal(i, status, checkErr)
		}
	}
}
