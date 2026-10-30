package qe

import "testing"

func TestArithmeticPrepareFormulaNNF(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	z := f.number("0")
	n := f.number("-1")
	l := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticMul(n, f.x)), z))
	u := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.x, f.number("-5"))), z))
	form := f.own(m.MkAnd(l, u))
	ok, e := f.plugin.PrepareFormulaNNF(f.contains, form)
	fmMust(t, e)
	if !ok {
		t.Fatal("formula preparation declined")
	}
}
