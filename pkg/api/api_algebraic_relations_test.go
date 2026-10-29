package api

import "testing"

func TestAPIAlgebraicRelationsMixedValues(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	lt, err := f.c.AlgebraicLT(f.one, root)
	astTestMust(t, err)
	gt, err := f.c.AlgebraicGT(root, f.one)
	astTestMust(t, err)
	le, err := f.c.AlgebraicLE(root, root)
	astTestMust(t, err)
	ge, err := f.c.AlgebraicGE(root, root)
	astTestMust(t, err)
	equal, err := f.c.AlgebraicEqual(root, root)
	astTestMust(t, err)
	notEqual, err := f.c.AlgebraicNotEqual(root, f.three)
	astTestMust(t, err)
	if !lt || !gt || !le || !ge || !equal || !notEqual {
		t.Fatal("relation results", lt, gt, le, ge, equal, notEqual)
	}
}
