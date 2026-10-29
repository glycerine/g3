package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVBoundsSourcePropagateEqualityAndTranslation(t *testing.T) {
	f := newBVBoundsFixture(t)
	p := f.base.context.EmptyValue()
	defer func() { sharingMust(t, f.base.context.Del(&p)) }()
	key := "propagate_eq"
	sharingMust(t, f.base.context.SetBoolName(&p, &key, true))
	sharingMust(t, f.simp.UpdateParams(&p))
	if !f.simp.propagateEq {
		t.Fatal("parameter")
	}
	translated, e := f.simp.Translate(f.m)
	sharingMust(t, e)
	t.Cleanup(func() { sharingMust(t, translated.Close()) })
	if !translated.(*BVBoundsSimplifier).propagateEq {
		t.Fatal("translated parameter")
	}
	out, e := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, e)
	defer f.m.DelRef(&out)
	if done, e := translated.Simplify(f.x, out); e != nil || done {
		t.Fatal(done, e)
	}
}
