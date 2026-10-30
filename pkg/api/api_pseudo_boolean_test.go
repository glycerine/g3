package api

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextPseudoBooleanAPI(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	a := f.pin(f.m.MkTrue())
	b := f.pin(f.m.MkFalse())
	args := []ast.ASTHandle{a, b}
	atMost := f.pin(f.c.PseudoBooleanAtMost(args, 1))
	if yes, err := f.m.IsPseudoBooleanAtMost(atMost); err != nil || !yes {
		t.Fatal("at-most", yes, err)
	}
	atLeast := f.pin(f.c.PseudoBooleanAtLeast(args, 1))
	if yes, err := f.m.IsPseudoBooleanAtLeast(atLeast); err != nil || !yes {
		t.Fatal("at-least", yes, err)
	}
	checks := []struct {
		name  string
		term  ast.ASTHandle
		check func(ast.ASTHandle) (bool, error)
	}{
		{"le", f.pin(f.c.PseudoBooleanLE(args, []int64{2, 3}, 4)), f.m.IsPseudoBooleanLE},
		{"ge", f.pin(f.c.PseudoBooleanGE(args, []int64{2, 3}, 4)), f.m.IsPseudoBooleanGE},
		{"eq", f.pin(f.c.PseudoBooleanEQ(args, []int64{2, 3}, 4)), f.m.IsPseudoBooleanEQ},
	}
	for _, check := range checks {
		if yes, err := check.check(check.term); err != nil || !yes {
			t.Fatal(check.name, yes, err)
		}
	}
}

func TestSMTContextPseudoBooleanValidationAndSignedCoefficients(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	truth := f.pin(f.m.MkTrue())
	falsehood := f.pin(f.m.MkFalse())
	arguments := []ast.ASTHandle{truth, falsehood}
	for _, build := range []func([]ast.ASTHandle, []int64, int64) (ast.ASTHandle, error){
		f.c.PseudoBooleanLE, f.c.PseudoBooleanGE, f.c.PseudoBooleanEQ,
	} {
		term, err := build(arguments, []int64{-3, 2}, -1)
		if err != nil || term.IsNull() {
			t.Fatal("signed pseudo-Boolean coefficients", term, err)
		}
		if _, err = build(arguments, []int64{1}, 0); !errors.Is(err, ast.ErrASTDomain) {
			t.Fatal("coefficient count mismatch", err)
		}
	}
	text := f.pin(f.c.String("not-boolean"))
	if _, err := f.c.PseudoBooleanAtMost([]ast.ASTHandle{text}, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("at-most non-Boolean argument", err)
	}
	if _, err := f.c.PseudoBooleanAtLeast([]ast.ASTHandle{text}, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("at-least non-Boolean argument", err)
	}
}
