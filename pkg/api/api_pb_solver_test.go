package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextPseudoBooleanConstraintReachesSolver(t *testing.T) {
	f, a, b := pbAPIFixture(t)
	atLeast := f.pin(f.c.PseudoBooleanAtLeast([]ast.ASTHandle{a, b}, 1))
	notA := f.pin(f.m.MkNot(a))
	notB := f.pin(f.m.MkNot(b))
	astTestMust(t, f.c.Assert(atLeast))
	astTestMust(t, f.c.Assert(notA))
	astTestMust(t, f.c.Assert(notB))
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatalf("PB contradiction status: %v", status)
	}
}
