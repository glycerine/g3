package api

import "testing"

func TestAPIOptimizeModelBeforeCheckIsEmpty(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	model, err := f.opt.Model()
	if err != nil {
		t.Fatal(err)
	}
	defer model.Close()
	constants, err := model.NumConstants()
	if err != nil {
		t.Fatal(err)
	}
	functions, err := model.NumFunctions()
	if err != nil {
		t.Fatal(err)
	}
	sorts, err := model.NumSorts()
	if err != nil {
		t.Fatal(err)
	}
	if constants != 0 || functions != 0 || sorts != 0 {
		t.Fatal(constants, functions, sorts)
	}
}
