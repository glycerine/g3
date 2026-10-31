package qe_test

// Source: src/qe/qe_arith_plugin.cpp mk_resolve/mk_atom.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticQEResolveUnitRealAndIntegerGate(t *testing.T) {
	f := newArithmeticQEFixture(t)
	resolved, handled, err := f.util.ResolveUnitOrReal(f.rx, false, big.NewRat(2, 1), f.ry, big.NewRat(-3, 1), f.rx)
	pluginMust(t, err)
	if !handled {
		t.Fatal("real nonunit resolution refused")
	}
	h, err := f.manager.RefGet(resolved)
	pluginMust(t, err)
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != 2 {
		t.Fatal("resolved real bound kind", kind, err)
	}
	pluginMust(t, f.manager.DelRef(&resolved))
	resolved, handled, err = f.util.ResolveUnitOrReal(f.x, false, big.NewRat(4, 1), f.y, big.NewRat(-5, 1), f.x)
	pluginMust(t, err)
	if handled || resolved != (ast.ASTRef{}) {
		t.Fatal("nonunit integer resolution bypassed Cooper gate")
	}
	resolved, handled, err = f.util.ResolveUnitOrReal(f.x, true, big.NewRat(1, 1), f.y, big.NewRat(-1, 1), f.x)
	pluginMust(t, err)
	if !handled {
		t.Fatal("unit strict integer resolution refused")
	}
	pluginMust(t, f.manager.DelRef(&resolved))
}

func TestArithmeticQENormalizeAtomPolarities(t *testing.T) {
	f := newArithmeticQEFixture(t)
	equality := f.pin(f.manager.MkEq(f.x, f.y))
	positive, handled, err := f.util.NormalizeAtom(equality, true)
	pluginMust(t, err)
	if !handled {
		t.Fatal("positive equality normalization refused")
	}
	h, err := f.manager.RefGet(positive)
	pluginMust(t, err)
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != ast.ASTOpAnd {
		t.Fatal("positive equality normal form", kind, err)
	}
	pluginMust(t, f.manager.DelRef(&positive))
	negative, handled, err := f.util.NormalizeAtom(equality, false)
	pluginMust(t, err)
	if !handled {
		t.Fatal("negative equality normalization refused")
	}
	h, err = f.manager.RefGet(negative)
	pluginMust(t, err)
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != ast.ASTOpOr {
		t.Fatal("negative equality normal form", kind, err)
	}
	pluginMust(t, f.manager.DelRef(&negative))
	le := f.pin(f.manager.MkArithmeticLE(f.x, f.y))
	positive, handled, err = f.util.NormalizeAtom(le, true)
	pluginMust(t, err)
	ph, err := f.manager.RefGet(positive)
	pluginMust(t, err)
	if !handled || ph != le {
		t.Fatal("positive <= atom was not retained")
	}
	pluginMust(t, f.manager.DelRef(&positive))
	lt := f.pin(f.manager.MkArithmeticLT(f.x, f.y))
	negative, handled, err = f.util.NormalizeAtom(lt, false)
	pluginMust(t, err)
	h, err = f.manager.RefGet(negative)
	pluginMust(t, err)
	if !handled {
		t.Fatal("negative < normalization refused")
	}
	if kind, err := f.manager.GetDeclKind(h); err != nil || kind != 2 {
		t.Fatal("negative < normal form", kind, err)
	}
	pluginMust(t, f.manager.DelRef(&negative))
}
