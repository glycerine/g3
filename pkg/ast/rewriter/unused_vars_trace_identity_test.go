package rewriter

// Source: src/ast/rewriter/var_subst.cpp:108-127.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"strings"
	"testing"
)

func TestUnusedVarsAllUsedIdentityTrace(t *testing.T) {
	m, tags, output, _, _ := unusedTraceFixture(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	v, e := m.MkVar(0, b)
	astTestMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(2)}, Body: v})
	astTestMust(t, e)
	out, e := ElimUnusedVarsUFBV(m, q)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != q {
		t.Fatal("identity", got, q)
	}
	if tags.ShouldTrace("elim_unused_vars") && strings.Count(output.String(), "var_subst.cpp:") != 2 {
		t.Fatal("trace sites", output.String())
	}
}
