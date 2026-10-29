package arith

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBV2IntPartialRewriterRetainsContextBorrow(t *testing.T) {
	base, m := fixDLFixture(t, false)
	ctx, err := NewBV2IntContext(m, base.context, nil)
	equationMust(t, err)
	m.Budget().FailAfter(1) // rewriter header succeeds; traversal owner refuses.
	r, err := NewBV2IntRewriter(ctx)
	m.Budget().FailAfter(-1)
	if err == nil || r == nil {
		t.Fatal("expected partial rewriter", r, err)
	}
	if err = ctx.Close(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("partial rewriter lost context borrow", err)
	}
	equationMust(t, r.Close())
	equationMust(t, ctx.Close())
}
