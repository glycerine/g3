package ast

// Source: src/ast/ast_smt2_pp.cpp:722-758,1081.

import "testing"

func TestASTSMT2PrinterParamsDriveAssociativeFlattening(t *testing.T) {
	m, c, g, local, budget := smt2ProfileFixture(t)
	b, err := m.MkBoolSort()
	paramsMust(t, err)
	a, err := m.MkConst(astTestSymbol(t, m, "fa"), b)
	paramsMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "fx"), b)
	paramsMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "fy"), b)
	paramsMust(t, err)
	inner, err := m.MkAnd(x, y)
	paramsMust(t, err)
	root, err := m.MkAnd(a, inner)
	paramsMust(t, err)
	key := "flat_assoc"
	paramsMust(t, c.SetBoolName(local, &key, false))
	format, err := m.SMT2PPBooleanTreeWithParams(root, nil, c, g, local)
	paramsMust(t, err)
	if got := smt2ProfileRender(t, m, format, c, g, local, budget); got != "(and fa (and fx fy))" {
		t.Fatal("parameter-backed non-flat tree", got)
	}
}
