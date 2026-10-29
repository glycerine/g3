package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextPseudoBooleanIntegerRelations(t *testing.T) {
	f, a, b := pbAPIFixture(t)
	args := []ast.ASTHandle{a, b}
	for _, test := range []struct {
		want int32
		make func([]ast.ASTHandle, []int32, int32) (ast.ASTHandle, error)
	}{
		{ast.ASTPBOpLE, f.c.PseudoBooleanLE},
		{ast.ASTPBOpGE, f.c.PseudoBooleanGE},
		{ast.ASTPBOpEQ, f.c.PseudoBooleanEQ},
	} {
		formula := f.pin(test.make(args, []int32{-3, 7}, -2))
		if kind := pbAPIKind(t, f.m, formula); kind != test.want {
			t.Fatalf("relation kind: got %d want %d", kind, test.want)
		}
	}
}
