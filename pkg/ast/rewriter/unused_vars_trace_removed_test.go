package rewriter

// Source: src/ast/rewriter/var_subst.cpp:139-178.

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"strings"
	"testing"
)

func TestUnusedVarsAllRemovedExternalShiftTrace(t *testing.T) {
	m, tags, output, _, _ := unusedTraceFixture(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	outer, e := m.MkVar(1, b)
	astTestMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(3)}, Body: outer})
	astTestMust(t, e)
	out, e := ElimUnusedVarsUFBV(m, q)
	astTestMust(t, e)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if idx, e := m.GetVarIndex(got); e != nil || idx != 0 {
		t.Fatal("shift", idx, e)
	}
	if tags.ShouldTrace("elim_unused_vars") && !strings.Contains(output.String(), " -> (:var 0)") {
		t.Fatal("result trace", output.String())
	}
}
