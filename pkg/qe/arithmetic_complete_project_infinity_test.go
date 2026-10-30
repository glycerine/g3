package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticCompleteProjectInfinityWitness(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	r, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&r)
	d, e := f.r.manager.NewRef(ast.ASTHandle{})
	fmMust(t, e)
	defer f.r.manager.DelRef(&d)
	ok, e := f.plugin.ProjectModelFormulaNNFComplete(f.contains, f.model, r, &d, f.r.config)
	fmMust(t, e)
	if !ok {
		t.Fatal("declined")
	}
	h, e := f.r.manager.RefGet(d)
	fmMust(t, e)
	if h.IsNull() {
		t.Fatal("missing infinity witness")
	}
}
