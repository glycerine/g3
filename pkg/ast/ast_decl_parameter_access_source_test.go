package ast

// Source: src/ast/ast.h declaration parameter access consumed by api_ast.cpp.

import (
	"errors"
	"testing"
)

func TestASTDeclParameterAccessMetadataAndBorrowedASTPayload(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	leaf, err := m.MkConst(astTestSymbol(t, m, "parameter-owned-leaf"), boolean)
	astTestMust(t, err)
	integer, err := m.NewParameterInt(19)
	astTestMust(t, err)
	value, err := m.NewParameterAST(leaf)
	astTestMust(t, err)
	info, err := m.NewFuncDeclInfo(NullFamilyID, 917, []ASTParameter{integer, value}, ASTFuncFlags{}, false)
	astTestMust(t, err)
	decl, err := m.MkFuncDeclWithInfo(astTestSymbol(t, m, "parameter-owning-decl"), nil, boolean, info)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(decl))
	astTestMust(t, m.DelFuncDeclInfo(&info))
	astTestMust(t, m.DelParameter(&integer))
	astTestMust(t, m.DelParameter(&value))
	m.Budget().FailAfter(0)
	defer m.Budget().FailAfter(-1)
	count, err := m.GetDeclNumParameters(decl)
	if err != nil || count != 2 {
		t.Fatal("metadata count must not allocate", count, err)
	}
	if _, err := m.GetDeclParameterCopy(decl, 2); !errors.Is(err, ErrASTDomain) {
		t.Fatal("parameter bounds before allocation", err)
	}
	m.Budget().FailAfter(-1)
	before, err := m.GetRefCount(leaf)
	astTestMust(t, err)
	copy, err := m.GetDeclParameterCopy(decl, 1)
	astTestMust(t, err)
	selected, err := m.ParameterAST(copy)
	if err != nil || selected != leaf {
		t.Fatal("selected parameter value", selected, leaf, err)
	}
	if after, err := m.GetRefCount(leaf); err != nil || after != before {
		t.Fatal("parameter copy must borrow its AST payload", before, after, err)
	}
	astTestMust(t, m.DecRef(decl))
	if m.IsLiveHandle(leaf) {
		t.Fatal("parameter copy incorrectly owns its AST payload")
	}
	astTestMust(t, m.DelParameter(&copy))
	empty, err := m.MkFuncDecl(astTestSymbol(t, m, "parameter-empty"), nil, boolean, ASTFuncDeclInfo{})
	astTestMust(t, err)
	if count, err := m.GetDeclNumParameters(empty); err != nil || count != 0 {
		t.Fatal("uninterpreted declaration without info", count, err)
	}
	if _, err := m.GetDeclParameterCopy(empty, 0); !errors.Is(err, ErrASTDomain) {
		t.Fatal("empty parameter bounds", err)
	}
	if _, err := m.GetDeclNumParameters(boolean); !errors.Is(err, ErrASTDomain) {
		t.Fatal("sort accepted as declaration", err)
	}
	if _, err := m.GetDeclParameterCopy(ASTHandle{}, 0); err == nil {
		t.Fatal("null declaration accepted")
	}
}
