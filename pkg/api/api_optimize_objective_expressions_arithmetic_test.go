package api

import "testing"

func TestAPIOptimizeObjectiveExpressionsNormalizeArithmeticDirection(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if _, err := f.opt.Minimize(f.x); err != nil {
		t.Fatal(err)
	}
	if _, err := f.opt.Maximize(f.x); err != nil {
		t.Fatal(err)
	}
	refs, err := f.opt.ObjectiveExpressions()
	if err != nil || len(refs) != 2 {
		t.Fatal(len(refs), err)
	}
	first, err := f.m.RefGet(refs[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.m.RefGet(refs[1])
	if err != nil {
		t.Fatal(err)
	}
	if first != f.x {
		t.Fatal(first)
	}
	if kind, e := f.m.GetDeclKind(second); e != nil || kind != 8 {
		t.Fatal(kind, e)
	}
	argument, err := f.m.GetArg(second, 0)
	if err != nil || argument != f.x {
		t.Fatal(argument, err)
	}
	for i := range refs {
		if err = f.m.DelRef(&refs[i]); err != nil {
			t.Fatal(err)
		}
	}
}
