package bv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVBoundsSimplifyFullWithoutContext(t *testing.T) {
	f := newBVBoundsFixture(t)
	full := f.ule(t, f.x, f.num(t, 255))
	out, err := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, err)
	defer f.m.DelRef(&out)
	done, err := f.simp.Simplify(full, out)
	sharingMust(t, err)
	value, err := f.m.RefGet(out)
	sharingMust(t, err)
	if !done || !f.m.IsTrue(value) {
		t.Fatal("tight full bound was not folded to true")
	}

	notFull, err := f.m.MkNot(full)
	sharingMust(t, err)
	done, err = f.simp.Simplify(notFull, out)
	sharingMust(t, err)
	value, err = f.m.RefGet(out)
	sharingMust(t, err)
	if !done || !f.m.IsFalse(value) {
		t.Fatal("negated tight full bound was not folded to false")
	}
}
