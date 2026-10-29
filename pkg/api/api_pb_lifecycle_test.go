package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextPseudoBooleanConstructorsHonorClosedContext(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, c.Close())
	if _, err = c.PseudoBooleanAtMost(nil, 0); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatalf("at-most closed context: %v", err)
	}
	if _, err = c.PseudoBooleanAtLeast(nil, 0); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatalf("at-least closed context: %v", err)
	}
	for name, make := range map[string]func([]ast.ASTHandle, []int32, int32) (ast.ASTHandle, error){
		"le": c.PseudoBooleanLE,
		"ge": c.PseudoBooleanGE,
		"eq": c.PseudoBooleanEQ,
	} {
		if _, err = make(nil, nil, 0); !errors.Is(err, ErrSMTContextOwner) {
			t.Fatalf("%s closed context: %v", name, err)
		}
	}
}
