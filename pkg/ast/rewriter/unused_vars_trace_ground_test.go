package rewriter

// Source: src/ast/rewriter/var_subst.cpp:98-106,210.

import (
	"bytes"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func unusedTraceFixture(t *testing.T) (*ast.ASTManager, *diagnostic.TraceTags, *bytes.Buffer, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	output := &bytes.Buffer{}
	tags := diagnostic.NewTraceTagsWithWriter(output)
	tags.Enable("elim_unused_vars")
	m := unusedVarsTraceManager(t, tags)
	bindBVTraceGlobals(t, m)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(1)}, Body: truth})
	astTestMust(t, e)
	return m, tags, output, q, truth
}

func TestUnusedVarsGroundBranchSourceTrace(t *testing.T) {
	m, tags, output, q, truth := unusedTraceFixture(t)
	out, e := ElimUnusedVarsUFBV(m, q)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != truth {
		t.Fatal("ground result", got)
	}
	// AUDIT ast_rewriter_05-F7: z3 has no elim_unused_vars trace tag.
	if tags.ShouldTrace("elim_unused_vars") && output.Len() != 0 {
		t.Fatalf("Go-only elim_unused_vars trace: %q", output.String())
	}
}
