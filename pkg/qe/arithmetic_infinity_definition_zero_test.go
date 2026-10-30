package qe_test

import "testing"

func TestArithmeticInfinityDefinitionEmptyOppositeIsZero(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	truth := f.pin(f.manager.MkTrue())
	ok, e := f.plugin.Prepare(f.contains, truth, nil, nil)
	pluginMust(t, e)
	if !ok {
		t.Fatal("prepare")
	}
	d, e := f.plugin.InfinityDefinition(f.contains, truth)
	pluginMust(t, e)
	defer f.manager.DelRef(&d)
	h, e := f.manager.RefGet(d)
	pluginMust(t, e)
	text, _, matched, e := f.manager.ArithmeticNumeralText(h)
	pluginMust(t, e)
	if !matched || text != "0" {
		t.Fatal(text, matched)
	}
}
