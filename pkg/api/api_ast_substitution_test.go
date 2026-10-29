package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTSubstituteIsSimultaneousAndOwning(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	x := f.pin(f.c.Constant("subst-x", boolean))
	y := f.pin(f.c.Constant("subst-y", boolean))
	term := f.pin(f.c.Equal(x, y))

	resultRef, err := f.c.Substitute(term, []ast.ASTHandle{x, y}, []ast.ASTHandle{y, x})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&resultRef)) })
	result, err := f.m.RefGet(resultRef)
	astTestMust(t, err)
	if left, e := f.m.GetArg(result, 0); e != nil || left != y {
		t.Fatal("simultaneous left", left, y, e)
	}
	if right, e := f.m.GetArg(result, 1); e != nil || right != x {
		t.Fatal("simultaneous right", right, x, e)
	}
	// The replacer and its substitution have already been destroyed. The
	// returned owning reference must keep the rewritten result live.
	if _, err = f.m.GetKind(result); err != nil {
		t.Fatal("result did not survive temporary replacer", err)
	}
}

func TestSMTContextASTSubstituteVariablesUsesAPIOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	zero := f.pin(f.m.MkVar(0, boolean))
	one := f.pin(f.m.MkVar(1, boolean))
	term := f.pin(f.c.Equal(zero, one))
	x := f.pin(f.c.Constant("var-subst-x", boolean))
	y := f.pin(f.c.Constant("var-subst-y", boolean))

	resultRef, err := f.c.SubstituteVariables(term, []ast.ASTHandle{x, y})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&resultRef)) })
	result, err := f.m.RefGet(resultRef)
	astTestMust(t, err)
	if left, e := f.m.GetArg(result, 0); e != nil || left != x {
		t.Fatal("variable zero", left, x, e)
	}
	if right, e := f.m.GetArg(result, 1); e != nil || right != y {
		t.Fatal("variable one", right, y, e)
	}
}

func TestSMTContextASTSubstituteRejectsShapeAndSortErrors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	x := f.pin(f.c.Constant("subst-error-x", boolean))
	if _, err := f.c.Substitute(x, []ast.ASTHandle{x}, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("mismatched vectors", err)
	}
	integer := f.arithmeticVariable("subst-error-i", true)
	if _, err := f.c.Substitute(x, []ast.ASTHandle{x}, []ast.ASTHandle{integer}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("mismatched sorts", err)
	}
}
