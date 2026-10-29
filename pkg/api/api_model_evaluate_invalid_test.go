package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIModelEvaluateRejectsForeignAndClosedOwners(t *testing.T) {
	left := newAPIModelEvaluateFixture(t)
	right := newSMTAPIFixture(t, "ALL", nil)
	foreign := right.pin(right.c.Bool(true))
	if _, err := left.model.Evaluate(foreign, false); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("foreign expression", err)
	}
	md, err := left.c.NewAPIModel()
	astTestMust(t, err)
	astTestMust(t, md.Close())
	if _, err = md.Evaluate(left.x, false); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed model", err)
	}
}
