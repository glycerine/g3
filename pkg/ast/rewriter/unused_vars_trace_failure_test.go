package rewriter

// Source: src/util/trace.h sticky writer behavior and result ownership.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"io"
	"testing"
)

type unusedVarsRefusingWriter struct{}

func (unusedVarsRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestUnusedVarsTraceFailurePreservesResultOwnership(t *testing.T) {
	tags := diagnostic.NewTraceTagsWithWriter(unusedVarsRefusingWriter{})
	tags.Enable("elim_unused_vars")
	m := unusedVarsTraceManager(t, tags)
	bindBVTraceGlobals(t, m)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	truth, e := m.MkTrue()
	astTestMust(t, e)
	q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(9)}, Body: truth})
	astTestMust(t, e)
	out, e := ElimUnusedVarsUFBV(m, q)
	astTestMust(t, e)
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != truth {
		t.Fatal("stream failure changed result", got, truth)
	}
	astTestMust(t, m.DelRef(&out))
}
