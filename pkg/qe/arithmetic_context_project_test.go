package qe

import "testing"

func TestArithmeticProjectContextVariablesComplete(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	formula, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&formula)
	result, e := f.plugin.ProjectContextVariablesNNFComplete(f.model, formula, f.r.config)
	fmMust(t, e)
	defer result.Close()
	n, e := result.Len()
	fmMust(t, e)
	if n != 1 {
		t.Fatal(n)
	}
	variable, e := result.Variable(0)
	fmMust(t, e)
	if variable != f.x {
		t.Fatal(variable, f.x)
	}
}
