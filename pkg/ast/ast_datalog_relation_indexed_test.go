package ast

import (
	"errors"
	"testing"
)

func datalogTestRelationColumns(t *testing.T, m *ASTManager, app ASTHandle, expected ...ASTHandle) {
	t.Helper()
	sort, e := m.GetSort(app)
	astTestMust(t, e)
	columns, e := m.DatalogRelationColumns(sort)
	astTestMust(t, e)
	if len(columns) != len(expected) {
		t.Fatal("relation result column count", columns, expected)
	}
	for i := range columns {
		if columns[i] != expected[i] {
			t.Fatal("relation result columns", i, columns, expected)
		}
	}
}

func TestASTDatalogOriginalProjectRenameJoinAndNegationFilter(t *testing.T) {
	m := astTestManager(t)
	a, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "indexed-a"), 2)
	astTestMust(t, e)
	b, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "indexed-b"), 3)
	astTestMust(t, e)
	c, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "indexed-c"), 5)
	astTestMust(t, e)
	leftSort, e := m.MkDatalogRelationSort([]ASTHandle{a, b, c})
	astTestMust(t, e)
	rightSort, e := m.MkDatalogRelationSort([]ASTHandle{b, a})
	astTestMust(t, e)
	left, e := m.MkDatalogEmpty(leftSort)
	astTestMust(t, e)
	right, e := m.MkDatalogEmpty(rightSort)
	astTestMust(t, e)
	project, e := m.MkDatalogProject(left, []uint32{1})
	astTestMust(t, e)
	astTestPin(t, m, project)
	datalogTestRelationColumns(t, m, project, a, c)
	rename, e := m.MkDatalogRename(left, []uint32{0, 2, 1})
	astTestMust(t, e)
	astTestPin(t, m, rename)
	datalogTestRelationColumns(t, m, rename, b, c, a)
	join, e := m.MkDatalogJoin(left, right, []uint32{1, 0})
	astTestMust(t, e)
	astTestPin(t, m, join)
	datalogTestRelationColumns(t, m, join, a, b, c, b, a)
	joinAll, e := m.MkDatalogJoin(left, right, nil)
	astTestMust(t, e)
	astTestPin(t, m, joinAll)
	datalogTestRelationColumns(t, m, joinAll, a, b, c, b, a)
	negation, e := m.MkDatalogNegationFilter(left, right, []uint32{1, 0})
	astTestMust(t, e)
	astTestPin(t, m, negation)
	datalogTestRelationColumns(t, m, negation, a, b, c)
}

func TestASTDatalogOriginalNullaryNegationFilter(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkDatalogRelationSort(nil)
	astTestMust(t, e)
	left, e := m.MkDatalogEmpty(sort)
	astTestMust(t, e)
	right, e := m.MkDatalogEmpty(sort)
	astTestMust(t, e)
	filter, e := m.MkDatalogNegationFilter(left, right, nil)
	astTestMust(t, e)
	astTestPin(t, m, filter)
	datalogTestRelationColumns(t, m, filter)
	declaration, e := m.GetDecl(filter)
	astTestMust(t, e)
	if kind, cause := m.GetDeclKind(declaration); cause != nil || kind != DatalogOpNegationFilter {
		t.Fatal("nullary negation declaration", kind, cause)
	}
	parameters, e := m.GetDeclParametersCopy(declaration)
	astTestMust(t, e)
	if len(parameters) != 0 {
		t.Fatal("nullary negation unexpectedly has column parameters", parameters)
	}
}

func TestASTDatalogOriginalIndexedRelationGuards(t *testing.T) {
	m := astTestManager(t)
	a, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "guard-a"), 2)
	astTestMust(t, e)
	b, e := m.MkDatalogFiniteSort(astTestSymbol(t, m, "guard-b"), 3)
	astTestMust(t, e)
	sort, e := m.MkDatalogRelationSort([]ASTHandle{a, b})
	astTestMust(t, e)
	relation, e := m.MkDatalogEmpty(sort)
	astTestMust(t, e)
	for name, call := range map[string]func() (ASTHandle, error){
		"empty-project":       func() (ASTHandle, error) { return m.MkDatalogProject(relation, nil) },
		"unordered-project":   func() (ASTHandle, error) { return m.MkDatalogProject(relation, []uint32{1, 0}) },
		"duplicate-project":   func() (ASTHandle, error) { return m.MkDatalogProject(relation, []uint32{0, 0}) },
		"short-rename":        func() (ASTHandle, error) { return m.MkDatalogRename(relation, []uint32{0}) },
		"rename-out-of-range": func() (ASTHandle, error) { return m.MkDatalogRename(relation, []uint32{0, 2}) },
		"odd-join":            func() (ASTHandle, error) { return m.MkDatalogJoin(relation, relation, []uint32{0}) },
		"join-sort-mismatch":  func() (ASTHandle, error) { return m.MkDatalogJoin(relation, relation, []uint32{0, 1}) },
		"odd-negation":        func() (ASTHandle, error) { return m.MkDatalogNegationFilter(relation, relation, []uint32{0}) },
		"negation-out-of-range": func() (ASTHandle, error) {
			return m.MkDatalogNegationFilter(relation, relation, []uint32{2, 0})
		},
		"negation-sort-mismatch": func() (ASTHandle, error) {
			return m.MkDatalogNegationFilter(relation, relation, []uint32{0, 1})
		},
	} {
		if _, e := call(); !errors.Is(e, ErrASTDomain) {
			t.Fatal(name, e)
		}
	}
}
