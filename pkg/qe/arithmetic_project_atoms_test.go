package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticProjectModelAtomsEndToEnd(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	z := f.number("0")
	n := f.number("-1")
	l0 := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticMul(n, f.x)), z))
	l1 := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.own(m.MkArithmeticMul(n, f.x)), n)), z))
	u := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.x, f.number("-5"))), z))
	form := f.own(m.MkAnd(f.own(m.MkAnd(l0, l1)), u))
	atoms, e := NewArithmeticAtomSet(m)
	fmMust(t, e)
	defer atoms.Close()
	for _, a := range []ast.ASTHandle{l0, l1, u} {
		fmMust(t, atoms.AddPositive(a))
	}
	r, e := m.NewRef(form)
	fmMust(t, e)
	defer m.DelRef(&r)
	ok, e := f.plugin.ProjectModelAtoms(f.contains, f.model, r, atoms, f.r.config)
	fmMust(t, e)
	if !ok {
		t.Fatal("projection declined")
	}
	h, e := m.RefGet(r)
	fmMust(t, e)
	if found, e := f.contains.Check(h); e != nil || found {
		t.Fatal(found, e)
	}
}
