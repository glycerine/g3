package qe_test

import "testing"

func TestArithmeticAssignSubstituteCompleteEliminatesVariable(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	truth := f.pin(f.manager.MkTrue())
	ok, e := f.plugin.Prepare(f.contains, truth, nil, nil)
	pluginMust(t, e)
	if !ok {
		t.Fatal("prepare")
	}
	s := f.manager.NumeralService()
	v, e := s.NewValue()
	pluginMust(t, e)
	defer s.Del(&v)
	pluginMust(t, s.SetUnsigned(&v, 0))
	r, e := f.manager.NewRef(truth)
	pluginMust(t, e)
	defer f.manager.DelRef(&r)
	pluginMust(t, f.plugin.AssignSubstituteComplete(f.contains, r, &v, nil))
	h, e := f.manager.RefGet(r)
	pluginMust(t, e)
	if found, e := f.contains.Check(h); e != nil || found {
		t.Fatal(found, e)
	}
}
