package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVBoundsSourceLeadingZeroPatchAndTopExtract(t *testing.T) {
	f := newBVBoundsFixture(t)
	upper := f.ule(t, f.x, f.num(t, 15))
	ok, e := f.simp.AssertExpression(upper, false)
	sharingMust(t, e)
	if !ok {
		t.Fatal()
	}
	sum, e := f.m.MkBitVectorAdd(f.x, f.num(t, 1))
	sharingMust(t, e)
	out, e := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, e)
	defer f.m.DelRef(&out)
	done, e := f.simp.Simplify(sum, out)
	sharingMust(t, e)
	if !done {
		t.Fatal("leading-zero patch absent")
	}
	got, e := f.m.RefGet(out)
	sharingMust(t, e)
	arg, e := f.m.GetArg(got, 0)
	sharingMust(t, e)
	d, e := f.m.GetDecl(arg)
	sharingMust(t, e)
	if op, e := f.m.GetDeclKind(d); e != nil || op != 37 {
		t.Fatal("expected concat patch", op, e)
	}
	top, e := f.m.MkBitVectorExtract(7, 4, f.x)
	sharingMust(t, e)
	zero4, e := f.m.BitVectorNumeralUnsigned(0, 4)
	sharingMust(t, e)
	eq, e := f.m.MkEq(zero4, top)
	sharingMust(t, e)
	key, b, found, e := f.simp.recognize(eq)
	sharingMust(t, e)
	if !found || key != f.x || len(b.ranges) != 1 || b.ranges[0].hi.Int64() != 15 {
		t.Fatal(found, key, b)
	}
}
