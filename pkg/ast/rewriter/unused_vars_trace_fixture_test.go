package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func unusedVarsTraceManager(t *testing.T, tags *diagnostic.TraceTags) *ast.ASTManager {
	t.Helper()
	s, q, b := astTestOwners(t)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: s, Numerals: q, Budget: b, TraceTags: tags})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	return m
}
