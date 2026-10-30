package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:39-54,76-103.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestCachedVarSubstitutionResetAndGuards(t *testing.T) {
	m, _ := theoryRewriterFixture(t)
	q, left, right := cachedVarSubstitutionFixture(t, m)
	c, err := NewCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	out := astTestPin(t, m, ast.ASTHandle{})
	if _, err = c.Instantiate(out); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("instantiate without key", err)
	}
	truth, err := m.MkTrue()
	astTestMust(t, err)
	if err = c.Begin(truth, 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-quantifier key", err)
	}
	astTestMust(t, c.Begin(q, 2))
	astTestMust(t, c.SetBinding(0, left))
	if _, err = c.Instantiate(out); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("missing binding", err)
	}
	astTestMust(t, c.SetBinding(1, right))
	_, err = c.Instantiate(out)
	astTestMust(t, err)
	astTestMust(t, c.Reset())
	if c.Size() != 0 || c.current != nil {
		t.Fatal("reset retained cache state", c.Size(), c.current)
	}
	copyOwner := *c
	if err = copyOwner.Begin(q, 0); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("copied owner", err)
	}
	astTestMust(t, c.Close())
	if err = c.Begin(q, 0); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed owner", err)
	}
}
