package cmd_context

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestParserContextBorrowsManagerAndOwnsSessionState(t *testing.T) {
	parent := contextFixture(t)
	manager, err := parent.Manager()
	contextMust(t, err)
	child, err := parent.NewParserContext()
	contextMust(t, err)
	if !child.borrowedManager || child.manager != manager {
		t.Fatal("parser context did not borrow parent manager")
	}
	boolean, err := manager.MkBoolSort()
	contextMust(t, err)
	if err = child.InsertSort("ExternalBool", boolean); err != nil {
		t.Fatal("insert parser sort", err)
	}
	contextMust(t, child.Close())
	if _, err = manager.MkTrue(); err != nil {
		t.Fatal("parser close retired borrowed manager", err)
	}
}

func TestParserContextExistingFunctionBindingUsesExactDeclaration(t *testing.T) {
	parent := contextFixture(t)
	manager, err := parent.Manager()
	contextMust(t, err)
	boolean, err := manager.MkBoolSort()
	contextMust(t, err)
	name, err := manager.Symbols.Intern("parser-bound")
	contextMust(t, err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	contextMust(t, err)
	child, err := parent.NewParserContext()
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, child.Close()) })
	inserted, err := child.InsertFunction("bound", declaration)
	contextMust(t, err)
	if inserted != declaration {
		t.Fatal("binding declaration identity", inserted, declaration)
	}
}
