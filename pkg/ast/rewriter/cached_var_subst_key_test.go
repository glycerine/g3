package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:22-31,58-68.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCachedVarSubstitutionBindingIdentityAndOrderKeys(t *testing.T) {
	m, _ := theoryRewriterFixture(t)
	q, left, right := cachedVarSubstitutionFixture(t, m)
	c, err := NewCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	out := astTestPin(t, m, ast.ASTHandle{})
	insert := func(first, second ast.ASTHandle) bool {
		astTestMust(t, c.Begin(q, 2))
		astTestMust(t, c.SetBinding(0, first))
		astTestMust(t, c.SetBinding(1, second))
		cached, e := c.Instantiate(out)
		astTestMust(t, e)
		return cached
	}
	if insert(left, right) || insert(right, left) || !insert(left, right) {
		t.Fatal("ordered identity key behavior")
	}
	if c.Size() != 2 {
		t.Fatal("key count", c.Size())
	}
	leftKey := &cachedVarSubstitutionKey{quantifier: q, bindings: []ast.ASTHandle{left, right}}
	rightKey := &cachedVarSubstitutionKey{quantifier: q, bindings: []ast.ASTHandle{right, left}}
	if !cachedVarSubstitutionKeysEqual(leftKey, leftKey) || cachedVarSubstitutionKeysEqual(leftKey, rightKey) {
		t.Fatal("key equality")
	}
}
