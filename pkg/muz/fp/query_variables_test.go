package fp

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestCompactQueryVariablesPreservesSortsAndBoundVariables(t *testing.T) {
	m, _ := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	zero, err := m.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	fragmentMust(t, err)
	integer, err := m.GetSort(zero)
	fragmentMust(t, err)
	name, err := m.Symbols.Intern("compact-query-vars")
	fragmentMust(t, err)
	relation, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	bound, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	freeBool, err := m.MkVar(3, boolean)
	fragmentMust(t, err)
	freeInt, err := m.MkVar(6, integer)
	fragmentMust(t, err)
	body, err := m.MkApp(relation, []ast.ASTHandle{bound, freeBool, freeInt})
	fragmentMust(t, err)
	query, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body})
	fragmentMust(t, err)
	actual, err := engine.compactQueryVariables(query)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&actual)) }()
	compactBool, err := m.MkVar(1, boolean)
	fragmentMust(t, err)
	compactInt, err := m.MkVar(2, integer)
	fragmentMust(t, err)
	wantBody, err := m.MkApp(relation, []ast.ASTHandle{bound, compactBool, compactInt})
	fragmentMust(t, err)
	want, err := m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: wantBody})
	fragmentMust(t, err)
	got, err := m.RefGet(actual)
	fragmentMust(t, err)
	if got != want {
		t.Fatal("compaction changed bound variables or reordered free-variable sorts")
	}
}
