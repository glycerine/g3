package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeSetParametersSelectsLocalParetoPriority(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, parameters.Close()) }()
	astTestMust(t, parameters.SetSymbol("priority", "pareto"))
	astTestMust(t, f.opt.SetParameters(parameters))
	astTestMust(t, f.opt.Assert(f.lower))
	one := f.pin(f.m.ArithmeticIntegerNumeralText("1"))
	atMostOne := f.pin(f.m.MkArithmeticLE(f.x, one))
	astTestMust(t, f.opt.Assert(atMostOne))
	_, err = f.opt.Maximize(f.x)
	astTestMust(t, err)
	_, err = f.opt.Minimize(f.x)
	astTestMust(t, err)
	for i, want := range []lbool.LBool{lbool.True, lbool.True, lbool.False} {
		if status, checkErr := f.opt.Check(); checkErr != nil || status != want {
			t.Fatal("pareto status", i, status, checkErr)
		}
	}
}
