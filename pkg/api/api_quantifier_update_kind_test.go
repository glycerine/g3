package api

import "testing"

func TestSMTContextUpdateQuantifierKindAndBody(t *testing.T) {
	f := newQuantifierUpdateFixture(t)
	ref, err := f.context.UpdateQuantifierKind(f.value, false, f.false)
	updated := f.own(t, ref, err)
	if forall, err := f.manager.IsForall(updated); err != nil || forall {
		t.Fatal(forall, err)
	}
	if body, err := f.manager.GetQuantifierBody(updated); err != nil || body != f.false {
		t.Fatal(body, err)
	}
	if patterns, err := f.manager.GetNumPatterns(updated); err != nil || patterns != 1 {
		t.Fatal(patterns, err)
	}
}
