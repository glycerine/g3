package interp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func interpolationCheckerFixture(t *testing.T) (*ast.ASTManager, *Manager, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m, manager := interpManagerFixture(t)
	boolSort, err := manager.BoolType()
	interpMust(t, err)
	makeConstant := func(name string) ast.ASTHandle {
		h, e := manager.MakeVariable(name, boolSort)
		interpMust(t, e)
		return interpPin(t, m, h)
	}
	return m, manager, makeConstant("checker-p"), makeConstant("checker-q"), makeConstant("checker-r")
}

func TestInterpolationCheckerSequenceSuccess(t *testing.T) {
	m, manager, p, _, _ := interpolationCheckerFixture(t)
	notP, err := manager.MkNot(p)
	interpMust(t, err)
	s, err := newInterpolationScriptSolver(m, lbool.False)
	interpMust(t, err)
	s.script = []lbool.LBool{lbool.False, lbool.False}
	defer func() { interpMust(t, s.Close()) }()
	var diagnostics bytes.Buffer
	ok, err := CheckInterpolants(manager, s, &diagnostics, []ast.ASTHandle{p, notP}, nil, []ast.ASTHandle{p}, nil)
	interpMust(t, err)
	if !ok || diagnostics.Len() != 0 || s.checks != 2 {
		t.Fatal(ok, diagnostics.String(), s.checks)
	}
}

func TestInterpolationCheckerSemanticAndSymbolFailures(t *testing.T) {
	m, manager, p, q, r := interpolationCheckerFixture(t)
	for _, test := range []struct {
		name   string
		script []lbool.LBool
		itp    ast.ASTHandle
		want   string
	}{{"semantic", []lbool.LBool{lbool.True, lbool.True}, p, "interpolant 0 is incorrect"}, {"symbol", []lbool.LBool{lbool.False, lbool.False}, r, "bad symbols in interpolant 0:checker-r,"}} {
		t.Run(test.name, func(t *testing.T) {
			s, e := newInterpolationScriptSolver(m, lbool.False)
			interpMust(t, e)
			s.script = append([]lbool.LBool(nil), test.script...)
			defer func() { interpMust(t, s.Close()) }()
			var diagnostics bytes.Buffer
			ok, e := CheckInterpolants(manager, s, &diagnostics, []ast.ASTHandle{p, q}, nil, []ast.ASTHandle{test.itp}, nil)
			interpMust(t, e)
			if ok || !strings.Contains(diagnostics.String(), test.want) {
				t.Fatal(ok, diagnostics.String())
			}
		})
	}
}

func TestInterpolationCheckerTreeConversion(t *testing.T) {
	m, manager, p, q, _ := interpolationCheckerFixture(t)
	marked, err := manager.Make(OperatorInterp, q)
	interpMust(t, err)
	tree, err := manager.Make(OperatorAnd, p, marked)
	interpMust(t, err)
	truth, err := manager.Make(OperatorTrue)
	interpMust(t, err)
	s, err := newInterpolationScriptSolver(m, lbool.False)
	interpMust(t, err)
	s.script = []lbool.LBool{lbool.False, lbool.False}
	defer func() { interpMust(t, s.Close()) }()
	var diagnostics bytes.Buffer
	ok, err := CheckTreeInterpolants(manager, s, &diagnostics, []ast.ASTHandle{p, q}, tree, []ast.ASTHandle{truth})
	interpMust(t, err)
	if !ok || diagnostics.Len() != 0 {
		t.Fatal(ok, diagnostics.String())
	}
}

// AUDIT.md interp_01-F17: Z3_check_interpolant's iz3checker has empty
// scopes::parents, so the closure re-check asserts every constraint j <= i
// even for a tree; constraint 0 is not below node 1 in the tree.
func TestCheckInterpolantsParentsAPIUsesLinearClosure(t *testing.T) {
	m, manager, p, q, r := interpolationCheckerFixture(t)
	s, err := newInterpolationScriptSolver(m, lbool.False)
	interpMust(t, err)
	s.script = []lbool.LBool{lbool.False, lbool.True, lbool.False, lbool.False}
	defer func() { interpMust(t, s.Close()) }()
	var diagnostics bytes.Buffer
	_, err = CheckInterpolants(manager, s, &diagnostics, []ast.ASTHandle{p, q, r}, []int{2, 2, ScopeTop}, []ast.ASTHandle{p, q}, nil)
	interpMust(t, err)
	count := 0
	for _, h := range s.assertions {
		if h == p {
			count++
		}
	}
	// constraint 0 at i=0 and in the closure for i=1 (the check stops at
	// the first incorrect interpolant).
	if count != 2 || !strings.Contains(diagnostics.String(), "interpolant 1 is incorrect") {
		t.Fatal(count, diagnostics.String())
	}
}
