package api

import (
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
