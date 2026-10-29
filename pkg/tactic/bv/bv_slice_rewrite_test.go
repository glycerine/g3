package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVSliceSourceRebuildsEveryMarkedOccurrence(t *testing.T) {
	f := newBVSliceFixture(t)
	frame := newBVSliceFrame(f.tactic)
	defer frame.Close()
	sharingMust(t, frame.register(0, 15, f.x))
	sum, e := f.m.MkBitVectorAdd(f.x, f.x)
	sharingMust(t, e)
	got, e := frame.rewrite(sum)
	sharingMust(t, e)
	for i := uint32(0); i < 2; i++ {
		arg, e := f.m.GetArg(got, i)
		sharingMust(t, e)
		yes, e := frame.isConcat(arg)
		sharingMust(t, e)
		if !yes {
			d, _ := f.m.GetDecl(arg)
			op, _ := f.m.GetDeclKind(d)
			t.Fatal("unsliced occurrence", i, op)
		}
	}
	if s, e := f.m.GetSort(got); e != nil || s != f.sort {
		t.Fatal(s, e)
	}
	_ = ast.NullFamilyID
}
