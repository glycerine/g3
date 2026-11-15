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

type smt2UsesStubTable map[string]bool

func (t smt2UsesStubTable) SMT2SortInfo(ASTHandle) (SMT2SortInfo, bool) {
	return SMT2SortInfo{}, false
}

func (t smt2UsesStubTable) SMT2Uses(name string) bool { return t[name] }

// AUDIT.md ast_07-F1: smt2_printer::next_name skips names that the
// environment uses (cmd_context pp_env::uses), so an alias never captures a
// declared symbol.
func TestASTSMT2AliasSkipsEnvSymbols(t *testing.T) {
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
	previous := m.SetSMT2SortInfoTable(smt2UsesStubTable{"a!1": true})
	defer m.SetSMT2SortInfoTable(previous)
	format, err := m.SMT2PPBooleanTreeWithParams(root, nil, c, g, local)
	paramsMust(t, err)
	if got := smt2ProfileRender(t, m, format, c, g, local, budget); got != "(let ((a!2 (and pa px)))\n  (or a!2 a!2))" {
		t.Fatal("alias skips env symbol", got)
	}
}
