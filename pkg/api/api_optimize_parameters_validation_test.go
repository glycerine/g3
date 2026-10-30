package api

import "testing"

func TestAPIOptimizeSetParametersValidatesCatalog(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, parameters.Close()) }()
	astTestMust(t, parameters.SetBool("not_an_optimize_parameter", true))
	if err = f.opt.SetParameters(parameters); err == nil {
		t.Fatal("unknown optimize parameter accepted")
	}
}
