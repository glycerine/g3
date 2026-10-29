package arith

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBV2IntContextWaitsForAllBorrowingRewriters(t *testing.T) {
	ctx, _ := bv2IntContextFixture(t)
	first, err := NewBV2IntRewriter(ctx)
	equationMust(t, err)
	second, err := NewBV2IntRewriter(ctx)
	equationMust(t, err)
	if err = ctx.Close(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("context closed across live rewriters", err)
	}
	if ctx.borrowers != 2 {
		t.Fatal("borrow count", ctx.borrowers)
	}
	equationMust(t, first.Close())
	if err = ctx.Close(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("context ignored second rewriter", err)
	}
	equationMust(t, second.Close())
	equationMust(t, ctx.Close())
}
