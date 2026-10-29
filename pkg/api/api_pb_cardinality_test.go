package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextPseudoBooleanCardinalityConstructors(t *testing.T) {
	f, a, b := pbAPIFixture(t)
	atMost := f.pin(f.c.PseudoBooleanAtMost([]ast.ASTHandle{a, b}, 1))
	atLeast := f.pin(f.c.PseudoBooleanAtLeast([]ast.ASTHandle{a, b}, 1))
	if kind := pbAPIKind(t, f.m, atMost); kind != ast.ASTPBOpAtMost {
		t.Fatalf("at-most kind: %d", kind)
	}
	if kind := pbAPIKind(t, f.m, atLeast); kind != ast.ASTPBOpAtLeast {
		t.Fatalf("at-least kind: %d", kind)
	}
}
