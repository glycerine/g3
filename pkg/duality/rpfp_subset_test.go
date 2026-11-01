package duality

import "testing"

func TestTransformerSubsetAlignsParametersAndPreservesSolverScopes(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	x, err := ctx.BoolConst("subset-x")
	must(err)
	closeExpr(t, x)
	y, err := ctx.BoolConst("subset-y")
	must(err)
	closeExpr(t, y)
	notY, err := y.Not()
	must(err)
	closeExpr(t, notY)
	truth, err := ctx.BoolVal(true)
	must(err)
	closeExpr(t, truth)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	for _, c := range []struct {
		name        string
		left, right *Expr
		want        bool
	}{
		{"aligned equality", x, y, true},
		{"opposite", x, notY, false},
		{"empty subset", falsity, y, true},
		{"full superset", x, truth, true},
		{"full is not constrained", truth, y, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			left, err := NewTransformer(ctx, nil, []*Expr{x}, c.left)
			must(err)
			defer left.Close()
			right, err := NewTransformer(ctx, nil, []*Expr{y}, c.right)
			must(err)
			defer right.Close()
			got, err := left.SubsetEq(right, aux)
			must(err)
			if got != c.want {
				t.Fatal("containment", got, c.want)
			}
			count, err := aux.NumAssertions()
			must(err)
			level, err := aux.ScopeLevel()
			must(err)
			if count != 0 || level != 0 {
				t.Fatal("containment left solver state", count, level)
			}
		})
	}
	// Existing auxiliary assertions act as background and survive the query.
	must(aux.Push())
	h, err := x.Raw()
	must(err)
	must(aux.Assert(h))
	left, err := NewTransformer(ctx, nil, []*Expr{x}, truth)
	must(err)
	defer left.Close()
	right, err := NewTransformer(ctx, nil, []*Expr{y}, y)
	must(err)
	defer right.Close()
	got, err := left.SubsetEq(right, aux)
	must(err)
	if !got {
		t.Fatal("background assertion ignored")
	}
	count, err := aux.NumAssertions()
	must(err)
	level, err := aux.ScopeLevel()
	must(err)
	if count != 1 || level != 1 {
		t.Fatal("background scope changed", count, level)
	}
	must(aux.Pop(1))
}
