package qe

import "testing"

func TestArithmeticCompleteManyEmpty(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	r, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&r)
	result, e := f.plugin.ProjectVariablesNNFComplete(nil, f.model, r, f.r.config)
	fmMust(t, e)
	defer result.Close()
	n, e := result.Len()
	fmMust(t, e)
	if n != 0 {
		t.Fatal(n)
	}
}
