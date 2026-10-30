package ast

// Source: src/ast/ast_smt2_pp.cpp:693-716,1079-1080.

import "testing"

func TestASTSMT2PrinterParamsDriveAliasDepthAndWeight(t *testing.T) {
	m, c, g, local, budget := smt2ProfileFixture(t)
	b, err := m.MkBoolSort()
	paramsMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "pa"), b)
	paramsMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "px"), b)
	paramsMust(t, err)
	shared, err := m.MkAnd(a, x)
	paramsMust(t, err)
	root, err := m.MkOr(shared, shared)
	paramsMust(t, err)
	key := "max_depth"
	paramsMust(t, c.SetUIntName(local, &key, 100))
	key = "min_alias_size"
	paramsMust(t, c.SetUIntName(local, &key, 1))
	format, err := m.SMT2PPBooleanTreeWithParams(root, nil, c, g, local)
	paramsMust(t, err)
	if got := smt2ProfileRender(t, m, format, c, g, local, budget); got != "(let ((a!1 (and pa px)))\n  (or a!1 a!1))" {
		t.Fatal("parameter-backed alias", got)
	}
}
