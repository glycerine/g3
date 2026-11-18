package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFPAOrderedComparisonSourceSpecialAndNumericBranches(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	rw, err := NewFPARewriter(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	sort, err := m.MkFamilySort(rw.family, 3, nil)
	astTestMust(t, err)
	parameter, err := m.NewParameterAST(sort)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelParameter(&parameter)) }()
	value := func(kind int32) ast.ASTHandle {
		h, e := m.MkFamilyApp(rw.family, kind, []ast.ASTParameter{parameter}, nil)
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	nan, pinf, ninf, pzero, nzero := value(8), value(6), value(7), value(9), value(10)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	trueTerm, err := m.MkTrue()
	astTestMust(t, err)
	falseTerm, err := m.MkFalse()
	astTestMust(t, err)
	out := astTestPin(t, m, trueTerm)
	check := func(kind int32, left, right ast.ASTHandle, wantStatus RewriteStatus, want ast.ASTHandle) {
		t.Helper()
		astTestMust(t, m.SetRef(out, trueTerm))
		decl, e := m.MkFamilyFuncDecl(rw.family, kind, nil, []ast.ASTHandle{sort, sort}, ast.ASTHandle{})
		astTestMust(t, e)
		status, e := rw.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{left, right}), out)
		astTestMust(t, e)
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if status != wantStatus || got != want {
			t.Fatalf("comparison %d: status=%v got=%v want status=%v result=%v", kind, status, got, wantStatus, want)
		}
	}
	check(ast.FPAOpLt, nan, pzero, RewriteDone, falseTerm)
	check(ast.FPAOpLt, pzero, nan, RewriteDone, falseTerm)
	check(ast.FPAOpLt, pzero, nzero, RewriteDone, falseTerm)
	infinityRewrite := func(other, infinity ast.ASTHandle) ast.ASTHandle {
		eqInf, e := m.MkEq(other, infinity)
		astTestMust(t, e)
		neqInf, e := m.MkNot(eqInf)
		astTestMust(t, e)
		eqNaN, e := m.MkEq(other, nan)
		astTestMust(t, e)
		neqNaN, e := m.MkNot(eqNaN)
		astTestMust(t, e)
		answer, e := m.MkAnd(neqInf, neqNaN)
		astTestMust(t, e)
		return answer
	}
	check(ast.FPAOpLt, ninf, ninf, Rewrite3, infinityRewrite(ninf, ninf))
	check(ast.FPAOpLt, ninf, variable, Rewrite3, infinityRewrite(variable, ninf))
	check(ast.FPAOpLt, variable, pinf, Rewrite3, infinityRewrite(variable, pinf))
	check(ast.FPAOpLt, nzero, pzero, RewriteDone, falseTerm)
	check(ast.FPAOpLt, pinf, pzero, RewriteDone, falseTerm)
	check(ast.FPAOpLt, pzero, ninf, RewriteDone, falseTerm)
	check(ast.FPAOpLe, nzero, pzero, RewriteDone, trueTerm)
	check(ast.FPAOpLe, nan, pzero, RewriteDone, falseTerm)
	check(ast.FPAOpLe, pinf, pinf, RewriteDone, trueTerm)
	check(ast.FPAOpLe, variable, pzero, RewriteFailed, trueTerm)
	gt, err := m.MkFamilyApp(rw.family, ast.FPAOpLt, nil, []ast.ASTHandle{pinf, pzero})
	astTestMust(t, err)
	check(ast.FPAOpGt, pzero, pinf, Rewrite1, gt)
	ge, err := m.MkFamilyApp(rw.family, ast.FPAOpLe, nil, []ast.ASTHandle{pinf, pzero})
	astTestMust(t, err)
	check(ast.FPAOpGe, pzero, pinf, Rewrite1, ge)
}

// fpa_rewriter::mk_lt(-oo, x) is mk_and(mk_not(mk_eq(x, -oo)),
// mk_neq_nan(x)); GCC evaluates the arguments right to left, so the NaN
// equality is created before the infinity equality.
func TestFPALessInfinityConstructionOrder(t *testing.T) {
	m := astTestManager(t)
	_, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	astTestMust(t, err)
	rw, err := NewFPARewriter(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	sort, err := m.MkFamilySort(rw.family, 3, nil)
	astTestMust(t, err)
	parameter, err := m.NewParameterAST(sort)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelParameter(&parameter)) }()
	ninf, err := m.MkFamilyApp(rw.family, 7, []ast.ASTParameter{parameter}, nil)
	astTestMust(t, err)
	astTestPin(t, m, ninf)
	variable, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, variable)
	out := astTestPin(t, m, ast.ASTHandle{})
	decl, err := m.MkFamilyFuncDecl(rw.family, ast.FPAOpLt, nil, []ast.ASTHandle{sort, sort}, ast.ASTHandle{})
	astTestMust(t, err)
	astTestPin(t, m, decl)
	if _, err = rw.MkAppCore(decl, ast.SliceArguments([]ast.ASTHandle{ninf, variable}), out); err != nil {
		t.Fatal(err)
	}
	got, err := m.RefGet(out)
	astTestMust(t, err)
	id := func(h ast.ASTHandle, path ...uint32) uint32 {
		t.Helper()
		for _, i := range path {
			h, err = m.GetArg(h, i)
			astTestMust(t, err)
		}
		v, e := m.GetASTID(h)
		astTestMust(t, e)
		return v
	}
	if nanEq, infEq := id(got, 1, 0), id(got, 0, 0); nanEq >= infEq {
		t.Fatal("NaN equality must precede the infinity equality", nanEq, infEq)
	}
}
