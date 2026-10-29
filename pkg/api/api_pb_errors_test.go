package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextPseudoBooleanRejectsCoefficientArityAndNonBoolean(t *testing.T) {
	f, a, _ := pbAPIFixture(t)
	if _, err := f.c.PseudoBooleanLE([]ast.ASTHandle{a}, nil, 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("coefficient arity: %v", err)
	}
	integer := f.arithmeticVariable("api-pb-integer", true)
	if _, err := f.c.PseudoBooleanAtMost([]ast.ASTHandle{integer}, 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("non-Boolean argument: %v", err)
	}
}
