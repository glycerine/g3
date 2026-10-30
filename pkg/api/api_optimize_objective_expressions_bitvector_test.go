package api

import "testing"

func TestAPIOptimizeObjectiveExpressionsNegateBitVectorMaximum(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_BV", nil)
	sort := f.pin(f.m.BitVectorSort(8))
	name, err := f.m.Symbols.Intern("api-opt-objective-bv")
	if err != nil {
		t.Fatal(err)
	}
	x := f.pin(f.m.MkConst(name, sort))
	o, err := f.c.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if _, err = o.Maximize(x); err != nil {
		t.Fatal(err)
	}
	refs, err := o.ObjectiveExpressions()
	if err != nil || len(refs) != 1 {
		t.Fatal(len(refs), err)
	}
	handle, err := f.m.RefGet(refs[0])
	if err != nil {
		t.Fatal(err)
	}
	if kind, e := f.m.GetDeclKind(handle); e != nil || kind != 3 {
		t.Fatal(kind, e)
	}
	argument, err := f.m.GetArg(handle, 0)
	if err != nil || argument != x {
		t.Fatal(argument, err)
	}
	if err = f.m.DelRef(&refs[0]); err != nil {
		t.Fatal(err)
	}
}
