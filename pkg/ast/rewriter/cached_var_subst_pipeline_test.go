package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:47-103.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func cachedVarSubstitutionFixture(t *testing.T, m *ast.ASTManager) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	v0, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	v1, err := m.MkVar(1, boolean)
	astTestMust(t, err)
	body, err := m.MkAndArgs([]ast.ASTHandle{v0, v1})
	astTestMust(t, err)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{boolean, boolean},
		Names:  []symbol.Symbol{astTestSymbol(t, m, "cached-x"), astTestSymbol(t, m, "cached-y")},
		Body:   body,
	})
	astTestMust(t, err)
	left, err := m.MkConst(astTestSymbol(t, m, "cached-left"), boolean)
	astTestMust(t, err)
	right, err := m.MkConst(astTestSymbol(t, m, "cached-right"), boolean)
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{q, left, right} {
		astTestPin(t, m, h)
	}
	return q, left, right
}

func TestCachedVarSubstitutionSourceOrderAndCache(t *testing.T) {
	m, _ := theoryRewriterFixture(t)
	q, left, right := cachedVarSubstitutionFixture(t, m)
	c, err := NewCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, c.Begin(q, 2))
	astTestMust(t, c.SetBinding(0, left))
	astTestMust(t, c.SetBinding(1, right))
	cached, err := c.Instantiate(out)
	astTestMust(t, err)
	if cached || c.Size() != 1 {
		t.Fatal("first insertion", cached, c.Size())
	}
	value, err := m.RefGet(out)
	astTestMust(t, err)
	first, err := m.GetArg(value, 0)
	astTestMust(t, err)
	second, err := m.GetArg(value, 1)
	astTestMust(t, err)
	if first != right || second != left {
		t.Fatal("standard binding order", first, second, right, left)
	}
	astTestMust(t, m.SetRef(out, left))
	cached, err = c.Instantiate(out)
	astTestMust(t, err)
	reused, err := m.RefGet(out)
	astTestMust(t, err)
	if !cached || reused != value || c.Size() != 1 {
		t.Fatal("cache hit", cached, reused, value, c.Size())
	}
}
