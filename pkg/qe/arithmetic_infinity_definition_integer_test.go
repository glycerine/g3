package qe_test

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"testing"
)

func TestArithmeticInfinityDefinitionIntegerOppositeBounds(t *testing.T) {
	f := newArithmeticPluginFixture(t, false)
	z := f.integerNumeral(t, "0")
	n := f.integerNumeral(t, "-1")
	l := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticMul(n, f.x)), z))
	u0 := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticAdd(f.x, n)), z))
	u1 := f.pin(f.manager.MkArithmeticLE(f.pin(f.manager.MkArithmeticAdd(f.x, f.integerNumeral(t, "-2"))), z))
	form := f.pin(f.manager.MkAnd(l, f.pin(f.manager.MkAnd(u0, u1))))
	ok, e := f.plugin.Prepare(f.contains, form, []ast.ASTHandle{l, u0, u1}, nil)
	pluginMust(t, e)
	if !ok {
		t.Fatal("prepare")
	}
	d, e := f.plugin.InfinityDefinition(f.contains, form)
	pluginMust(t, e)
	defer f.manager.DelRef(&d)
	h, e := f.manager.RefGet(d)
	pluginMust(t, e)
	if found, e := f.contains.Check(h); e != nil || found {
		t.Fatal(found, e)
	}
	dir, e := f.plugin.InfinityDefinitionDirection(f.contains, form)
	pluginMust(t, e)
	if dir != qe.ArithmeticUpperBound {
		t.Fatal(dir)
	}
}
