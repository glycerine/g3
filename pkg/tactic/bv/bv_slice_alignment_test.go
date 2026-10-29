package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVSliceSourceAlignsUnequalConcatWidths(t *testing.T) {
	f := newBVSliceFixture(t)
	frame := newBVSliceFrame(f.tactic)
	defer frame.Close()
	a := f.extract(t, 31, 24, f.x)
	b := f.extract(t, 23, 0, f.x)
	c := f.extract(t, 31, 16, f.y)
	d := f.extract(t, 15, 0, f.y)
	left, e := f.m.MkBitVectorConcat([]ast.ASTHandle{a, b})
	sharingMust(t, e)
	right, e := f.m.MkBitVectorConcat([]ast.ASTHandle{c, d})
	sharingMust(t, e)
	sharingMust(t, frame.processEquality(f.eq(t, left, right)))
	if len(frame.boundaries[f.x]) == 0 || len(frame.boundaries[f.y]) == 0 {
		t.Fatal(frame.boundaries)
	}
}
