package bv

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBVBoundsZeroPatchConsumesNonWrappedModularBounds(t *testing.T) {
	f := newBVBoundsFixture(t)
	sum, err := f.m.MkBitVectorAdd(f.x, f.num(t, 1))
	sharingMust(t, err)
	out, err := f.m.NewRef(ast.ASTHandle{})
	sharingMust(t, err)
	defer f.m.DelRef(&out)

	f.simp.bounds[f.x] = bvBoundsSet{
		width: 8,
		ranges: []bvBoundsRange{
			{lo: big.NewInt(0), hi: big.NewInt(3)},
			{lo: big.NewInt(240), hi: big.NewInt(255)},
		},
		tight: true,
	}
	if done, err := f.simp.zeroPatch(sum, out); err != nil || done {
		t.Fatalf("wrapped interval was patched: done=%v err=%v", done, err)
	}

	f.simp.bounds[f.x] = bvBoundsLooseRangeSet(8, big.NewInt(0), big.NewInt(15))
	if done, err := f.simp.zeroPatch(sum, out); err != nil || !done {
		t.Fatalf("non-wrapped loose interval was not patched: done=%v err=%v", done, err)
	}
}
