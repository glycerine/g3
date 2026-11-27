package pattern

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestInferenceQuantifierSubtreesFoldNestedITE(t *testing.T) {
	m := patternManager(t)
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		patternMust(t, err)
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	truth := pin(m.MkTrue())
	falsity := pin(m.MkFalse())
	b := pin(m.MkBoolSort())
	a := pin(m.MkFreshConst("a", b))
	// The outer condition becomes constant only after folding the inner ITE.
	inner := pin(m.MkIte(truth, falsity, a))
	outer := pin(m.MkIte(inner, truth, a))
	wrapped := pin(m.MkNot(outer))
	folded := pin(m.MkNot(a))
	q, _, _ := patternDatabaseStoreQuantifier(t, m, false)
	input := pin(m.MkAnd(wrapped, q))

	rewrite := func(root ast.ASTHandle, prune bool) ast.ASTHandle {
		t.Helper()
		r, err := NewInferenceRewriter(m)
		patternMust(t, err)
		t.Cleanup(func() { patternMust(t, r.Close()) })
		out, err := m.NewRef(ast.ASTHandle{})
		patternMust(t, err)
		t.Cleanup(func() { patternMust(t, m.DelRef(&out)) })
		if prune {
			patternMust(t, r.RewriteQuantifierSubtrees(root, out))
		} else {
			patternMust(t, r.Rewrite(root, out))
		}
		h, err := m.RefGet(out)
		patternMust(t, err)
		return h
	}
	if got := rewrite(wrapped, true); got != folded {
		t.Fatalf("constant ITE survived inside negation: got %v, want %v", got, folded)
	}
	want := rewrite(input, false)
	got := rewrite(input, true)
	if got != want {
		t.Fatalf("different rewritten conjunctions: got %v, want %v", got, want)
	}
	child, err := m.GetArg(got, 1)
	patternMust(t, err)
	count, err := m.GetNumPatterns(child)
	patternMust(t, err)
	if count == 0 {
		t.Fatal("nested quantifier received no pattern")
	}
}
