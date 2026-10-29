package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
	"testing"
)

func TestBVBoundsSourceRecognizesUnsignedSignedAndEquality(t *testing.T) {
	f := newBVBoundsFixture(t)
	for _, tc := range []struct {
		term func() ast.ASTHandle
		lo   int64
	}{{func() ast.ASTHandle { return f.ule(t, f.num(t, 4), f.x) }, 4}, {func() ast.ASTHandle { return f.ule(t, f.x, f.num(t, 24)) }, 0}, {func() ast.ASTHandle { return f.sle(t, f.num(t, 128), f.x) }, 0}} {
		key, b, ok, e := f.simp.recognize(tc.term())
		sharingMust(t, e)
		if !ok || key != f.x || b.empty() {
			t.Fatal(ok, key, b)
		}
		if tc.lo == 4 && b.ranges[0].lo.Cmp(big.NewInt(tc.lo)) != 0 {
			t.Fatal(b)
		}
	}
}
