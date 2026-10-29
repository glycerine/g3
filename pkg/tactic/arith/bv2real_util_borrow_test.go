package arith

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBV2RealUtilityWaitsForRewriteAndEliminationBorrowers(t *testing.T) {
	u, _ := bv2RealFixture(t, 64)
	rewrite, err := NewBV2RealRewriter(u)
	equationMust(t, err)
	eliminate, err := NewBV2RealElimRewriter(u)
	equationMust(t, err)
	if err = u.Close(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("utility closed across live traversals", err)
	}
	if u.borrowers != 2 {
		t.Fatal("borrow count", u.borrowers)
	}
	equationMust(t, eliminate.Close())
	if err = u.Close(); !errors.Is(err, ast.ErrASTPending) {
		t.Fatal("utility ignored rewrite traversal", err)
	}
	equationMust(t, rewrite.Close())
	equationMust(t, u.Close())
}
