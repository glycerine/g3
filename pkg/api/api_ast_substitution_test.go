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

// AUDIT api_01-F1: Z3_substitute uses expr_safe_replace, which shifts the
// sources and destinations through quantifier binders. Expected strings are
// what build/libz3.so prints for the same calls.
func TestSubstituteDoesNotCaptureBoundVariable(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	intSort := f.pin(f.c.IntegerSort())
	v0 := f.pin(f.c.BoundVariable(0, intSort))
	v1 := f.pin(f.c.BoundVariable(1, intSort))
	zero := f.pin(f.c.Numeral("0", intSort))
	d := f.pin(f.c.Constant("d", intSort))
	substituted := func(term ast.ASTHandle, from, to []ast.ASTHandle) (ast.ASTHandle, string) {
		t.Helper()
		ref, err := f.c.Substitute(term, from, to)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, f.m.DelRef(&ref)) })
		h, err := f.m.RefGet(ref)
		astTestMust(t, err)
		s, err := f.c.ASTString(h, APIASTPrintSMT2)
		astTestMust(t, err)
		return h, s
	}

	body := f.pin(f.c.ArithmeticGT(v0, zero))
	q := f.pin(f.c.Forall([]ast.ASTHandle{intSort}, []string{"x"}, body))
	h, s := substituted(q, []ast.ASTHandle{v0}, []ast.ASTHandle{d})
	if want := "(forall ((x Int)) (> x 0))"; s != want {
		t.Fatalf("bound variable captured: got %q want %q", s, want)
	}
	if h != q {
		t.Fatal("unchanged quantifier should hash-cons back to the input")
	}

	// (:var 1) under one binder is the free (:var 0) shifted by one.
	body2 := f.pin(f.c.ArithmeticGT(v0, v1))
	q2 := f.pin(f.c.Forall([]ast.ASTHandle{intSort}, []string{"x"}, body2))
	if _, s = substituted(q2, []ast.ASTHandle{v0}, []ast.ASTHandle{d}); s != "(forall ((x Int)) (> x d))" {
		t.Fatalf("free variable under binder: got %q", s)
	}

	a := f.pin(f.c.Constant("a", intSort))
	b := f.pin(f.c.Constant("b", intSort))
	fd := f.pin(f.c.Function("f", []ast.ASTHandle{intSort, intSort, intSort}, intSort))
	gd := f.pin(f.c.Function("g", []ast.ASTHandle{intSort}, intSort))
	ga := f.pin(f.c.Apply(gd, a))
	term := f.pin(f.c.Apply(fd, a, b, ga))
	if _, s = substituted(term, []ast.ASTHandle{a, b}, []ast.ASTHandle{b, a}); s != "(f b a (g b))" {
		t.Fatalf("ground substitution: got %q", s)
	}
}
