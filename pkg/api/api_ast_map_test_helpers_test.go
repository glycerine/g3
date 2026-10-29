package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func newPublicASTMapFixture(t *testing.T) (*SMTContext, *ast.ASTManager, *ASTMap) {
	t.Helper()
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	manager, err := c.Manager()
	astTestMust(t, err)
	mapping, err := c.NewASTMap()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, mapping.Close()) })
	return c, manager, mapping
}

func publicASTMapAtom(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	boolean, err := manager.MkBoolSort()
	astTestMust(t, err)
	value, err := manager.MkConst(astTestSymbol(t, manager, name), boolean)
	astTestMust(t, err)
	return value
}
