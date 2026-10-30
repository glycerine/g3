package qe

import "testing"

func TestArithmeticPrepareAtomsPositiveAndNegative(t *testing.T) {
	f := newArithmeticModelFixture(t, true, "0")
	m := f.r.manager
	z := f.own(m.ArithmeticRealNumeralText("0"))
	pos := f.own(m.MkArithmeticLE(f.x, z))
	neg := f.own(m.MkArithmeticLT(f.x, z))
	atoms, e := NewArithmeticAtomSet(m)
	fmMust(t, e)
	defer atoms.Close()
	fmMust(t, atoms.AddPositive(pos))
	negative, e := NewArithmeticAtomSet(m)
	fmMust(t, e)
	defer negative.Close()
	fmMust(t, negative.AddNegative(neg))
	if hs, cause := negative.Negative(); cause != nil || len(hs) != 1 || hs[0] != neg {
		t.Fatal(hs, cause)
	}
	form := pos
	ok, e := f.plugin.PrepareAtoms(f.contains, form, atoms)
	fmMust(t, e)
	if !ok {
		t.Fatal("indexed bounds rejected")
	}
}
