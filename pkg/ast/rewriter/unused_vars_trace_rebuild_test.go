package rewriter

// Source: src/ast/rewriter/var_subst.cpp:180-204.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"strings"
	"testing"
)

func TestUnusedVarsRebuiltQuantifierTrace(t *testing.T) {
	m, tags, output, _, _ := unusedTraceFixture(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(0, b)
	astTestMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b, b}, Names: []usymbol.Symbol{usymbol.Numerical(4), usymbol.Numerical(5)}, Body: v})
	astTestMust(t, e)
	out, e := ElimUnusedVarsUFBV(m, q)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	got, e := m.RefGet(out)
	astTestMust(t, e)
	n, e := m.GetNumDeclarations(got)
	astTestMust(t, e)
	if n != 1 {
		t.Fatal("decls", n)
	}
	if tags.ShouldTrace("elim_unused_vars") && !strings.Contains(output.String(), " -> (forall") {
		t.Fatal("rebuild trace", output.String())
	}
}
