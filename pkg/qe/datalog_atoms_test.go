package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestDatalogQEAtomCollectionAndBranches(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	x := f.constant("x", f.integer)
	y := f.constant("y", f.integer)
	z := f.constant("z", f.integer)
	a := f.own(m.MkEq(x, y))
	b := f.own(m.MkEq(z, x))
	unrelated := f.own(m.MkEq(y, z))
	atoms, ok, e := collectDatalogQEAtoms(m, x, []ast.ASTHandle{unrelated, a}, []ast.ASTHandle{b})
	fmMust(t, e)
	if !ok || len(atoms.positive) != 1 || len(atoms.negative) != 1 || atoms.positive[0].atom != a || atoms.positive[0].value != y || atoms.negative[0].value != z {
		t.Fatal(atoms, ok)
	}
	for _, tc := range []struct {
		domain, count uint64
		small         bool
	}{{0, 0, true}, {1, 1, true}, {2, 2, false}, {100, 2, false}} {
		count, small := datalogQEBranchCount(tc.domain, atoms)
		if count != tc.count || small != tc.small {
			t.Fatalf("domain %d: %d %v", tc.domain, count, small)
		}
	}
	nested := f.own(m.MkArithmeticAdd(x, f.number("1")))
	invalid := []ast.ASTHandle{f.own(m.MkEq(x, nested)), f.own(m.MkEq(nested, y)), f.own(m.MkArithmeticLE(x, y)), f.own(m.MkEq(x, x))}
	for _, atom := range invalid {
		_, ok, e = collectDatalogQEAtoms(m, x, []ast.ASTHandle{a, atom}, nil)
		fmMust(t, e)
		if ok {
			t.Fatal("accepted invalid occurrence", atom)
		}
	}
	empty, ok, e := collectDatalogQEAtoms(m, x, []ast.ASTHandle{unrelated}, nil)
	fmMust(t, e)
	if !ok || len(empty.positive) != 0 || len(empty.negative) != 0 {
		t.Fatal(empty, ok)
	}
	count, small := datalogQEBranchCount(10, empty)
	if count != 1 || small {
		t.Fatal(count, small)
	}
}
