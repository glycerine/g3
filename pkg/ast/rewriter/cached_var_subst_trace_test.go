package rewriter

// Source: src/ast/rewriter/cached_var_subst.cpp:68-76.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestCachedVarSubstitutionCacheHitTrace(t *testing.T) {
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	tags.Enable("bindings")
	tags.Enable("coming_from_quant")
	m := cachedSubstTraceManager(t, tags)
	bindBVTraceGlobals(t, m)
	q, left, right := cachedVarSubstitutionFixture(t, m)
	c, err := NewCachedVarSubstitution(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, c.Begin(q, 2))
	astTestMust(t, c.SetBinding(0, left))
	astTestMust(t, c.SetBinding(1, right))
	_, err = c.Instantiate(out)
	astTestMust(t, err)
	output.Reset()
	cached, err := c.Instantiate(out)
	astTestMust(t, err)
	if !cached {
		t.Fatal("expected cache hit")
	}
	if !tags.ShouldTrace("bindings") {
		return
	}
	trace := output.String()
	for _, want := range []string{"cached_var_subst.cpp:68", "(cache)", "0:", "1:", "cached-left", "cached-right"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("missing cache trace %q in %q", want, trace)
		}
	}
}

func cachedSubstTraceManager(t *testing.T, tags *diagnostic.TraceTags) *ast.ASTManager {
	t.Helper()
	symbols, numerals, budget := astTestOwners(t)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{
		Symbols:   symbols,
		Numerals:  numerals,
		Budget:    budget,
		TraceTags: tags,
	})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, m.Close()) })
	return m
}
