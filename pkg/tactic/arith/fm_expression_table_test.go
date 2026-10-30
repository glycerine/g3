package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func fmBoolConst(t *testing.T, m *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, e := m.MkBoolSort()
	equationMust(t, e)
	symbol, e := m.Symbols.Intern(name)
	equationMust(t, e)
	h, e := m.MkConst(symbol, sort)
	equationMust(t, e)
	fixDLPin(t, m, h)
	return h
}

func TestFMExpressionTableOwnsMappings(t *testing.T) {
	_, m := fixDLFixture(t, false)
	variables := NewFMVariableTable()
	x := variables.Add(false, false)
	term, flag := fixDLConst(t, m, false, "fm_expr_x"), fmBoolConst(t, m, "fm_expr_p")
	table, e := NewFMExpressionTable(m, variables)
	equationMust(t, e)
	equationMust(t, table.BindVariable(x, term))
	p, e := table.AddBoolean(flag)
	equationMust(t, e)
	if p != 1 {
		t.Fatalf("first Boolean id = %d", p)
	}
	if _, e = table.Variable(x); e != nil {
		t.Fatal(e)
	}
	if e = table.BindVariable(x, term); e != ast.ErrASTDomain {
		t.Fatalf("duplicate binding error = %v", e)
	}
	equationMust(t, table.Close())
}
