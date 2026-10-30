package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticCompleteManyReverseOrder(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	y := f.constant("arith-many-y", f.integer)
	truth := f.own(f.r.manager.MkTrue())
	r, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&r)
	result, e := f.plugin.ProjectVariablesNNFComplete([]ast.ASTHandle{f.x, y}, f.model, r, f.r.config)
	fmMust(t, e)
	defer result.Close()
	n, e := result.Len()
	fmMust(t, e)
	if n != 2 {
		t.Fatal(n)
	}
	v0, e := result.Variable(0)
	fmMust(t, e)
	v1, e := result.Variable(1)
	fmMust(t, e)
	if v0 != y || v1 != f.x {
		t.Fatal(v0, v1)
	}
	for i := uint32(0); i < n; i++ {
		d, e := result.Definition(i)
		fmMust(t, e)
		if d.IsNull() {
			t.Fatal("missing definition", i)
		}
	}
}
