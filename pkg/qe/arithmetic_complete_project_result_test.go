package qe

import "testing"

func TestArithmeticCompleteProjectOptionalDefinition(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	r, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&r)
	ok, e := f.plugin.ProjectModelFormulaNNFComplete(f.contains, f.model, r, nil, f.r.config)
	fmMust(t, e)
	if !ok {
		t.Fatal("declined")
	}
	h, e := f.r.manager.RefGet(r)
	fmMust(t, e)
	if found, e := f.contains.Check(h); e != nil || found {
		t.Fatal(found, e)
	}
}
