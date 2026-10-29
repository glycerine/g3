package arith

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBV2RealPartialTraversalRetainsUtilityBorrow(t *testing.T) {
	_, f := bv2RealFixture(t, 64)
	u, err := NewBV2RealUtil(f.m, 2, 2, 64)
	equationMust(t, err)
	f.m.Budget().FailAfter(1) // rewriter header succeeds; traversal owner refuses.
	r, err := NewBV2RealRewriter(u)
	f.m.Budget().FailAfter(-1)
	if err == nil || r == nil {
		t.Fatal("expected partial rewriter", r, err)
	}
	if err = u.Close(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("partial traversal lost utility borrow", err)
	}
	equationMust(t, r.Close())
	equationMust(t, u.Close())
}
