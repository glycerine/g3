package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDRExpressionQueryBorrowsOriginalFixedpointRewriterSnapshot(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	m, e := f.manager, f.engine
	if e.pdr != nil {
		t.Fatal("fixture already created a PDR context")
	}
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	p, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	q, err := m.MkVar(1, boolean)
	fragmentMust(t, err)
	input, err := m.MkAnd(p, q)
	fragmentMust(t, err)
	inputRef, err := m.NewRef(input)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&inputRef)) }()
	before, err := m.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&before)) }()
	fragmentMust(t, e.guard.Rewrite(input, before))
	oldResult, err := m.RefGet(before)
	fragmentMust(t, err)
	fragmentMust(t, e.globals.SourceSetName("rewriter.elim_and", "true"))
	status, err := e.QueryExpression(f.applications["q"])
	fragmentMust(t, err)
	if status != lbool.True || e.queryClone == nil || e.queryClone.pdr == nil {
		t.Fatal("expression query did not construct a PDR clone", status)
	}
	clone := e.queryClone
	if clone.pdrTheory != e.guard || clone.guard == e.guard {
		t.Fatal("PDR clone did not borrow original fixedpoint rewriter")
	}
	after, err := m.NewRef(ast.ASTHandle{})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, m.DelRef(&after)) }()
	fragmentMust(t, clone.guard.Rewrite(input, after))
	newResult, err := m.RefGet(after)
	fragmentMust(t, err)
	if newResult == oldResult {
		t.Fatal("test did not establish different rewriter parameter snapshots")
	}
	// Closing the clone's PDR context must not close the parent rewriter.
	fragmentMust(t, clone.pdr.Close())
	clone.pdr = nil
	if e.guard.Manager() != m {
		t.Fatal("closing cloned PDR closed original fixedpoint rewriter")
	}
	fragmentMust(t, e.guard.Rewrite(input, before))
}
