package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestPDRContextBorrowsFixedpointRewriterSnapshotAndLifetime(t *testing.T) {
	m, owner, old, _, closeBase := closureFixture(t)
	globals := owner.contexts.config.Globals
	shared, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	closureMust(t, err)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	p, err := m.MkVar(0, boolean)
	closureMust(t, err)
	q, err := m.MkVar(1, boolean)
	closureMust(t, err)
	and, err := m.MkAnd(p, q)
	closureMust(t, err)
	input, err := m.MkNot(and)
	closureMust(t, err)
	inputRef, err := m.NewRef(input)
	closureMust(t, err)
	expected, err := m.NewRef(ast.ASTHandle{})
	closureMust(t, err)
	closureMust(t, shared.Rewrite(input, expected))
	oldValue, err := m.RefGet(expected)
	closureMust(t, err)
	closureMust(t, globals.SourceSetName("rewriter.elim_and", "true"))
	fresh, err := rewriter.NewTheoryRewriter(m, globals.Context, globals, nil)
	closureMust(t, err)
	freshResult, err := m.NewRef(ast.ASTHandle{})
	closureMust(t, err)
	closureMust(t, fresh.Rewrite(input, freshResult))
	newValue, err := m.RefGet(freshResult)
	closureMust(t, err)
	if newValue == oldValue {
		t.Fatal("test did not establish a changed global rewrite snapshot")
	}
	c, err := NewContextWithTheory(m, nil, 2, ReachableNoCache, owner.contexts.config, shared)
	closureMust(t, err)
	if c.projectionTheory != shared || !c.projectionTheoryBorrowed {
		t.Fatal("PDR did not borrow fixedpoint rewriter")
	}
	vars := []ast.ASTRef{}
	result, err := c.normalizeProjectedPhi([]ast.ASTHandle{input}, &vars, nil)
	closureMust(t, err)
	got, err := m.RefGet(result)
	closureMust(t, err)
	if got != oldValue {
		t.Fatal("projection used changed global settings", got, oldValue)
	}
	closureMust(t, m.DelRef(&result))
	closureMust(t, c.Close())
	if shared.Manager() != m {
		t.Fatal("PDR context closed its borrowed rewriter")
	}
	closureMust(t, shared.Rewrite(input, expected))
	closureMust(t, m.DelRef(&freshResult))
	closureMust(t, fresh.Close())
	closureMust(t, m.DelRef(&expected))
	closureMust(t, m.DelRef(&inputRef))
	closureMust(t, shared.Close())
	closureMust(t, old.Close())
	closureMust(t, owner.Close())
	closeBase()
}

func TestPDRContextBorrowedRewriterConstructorFailuresRecover(t *testing.T) {
	m, owner, old, _, closeBase := closureFixture(t)
	cfg := owner.contexts.config
	shared, err := rewriter.NewTheoryRewriter(m, cfg.Params, cfg.Globals, nil)
	closureMust(t, err)
	defer func() {
		closureMust(t, shared.Close())
		closureMust(t, old.Close())
		closureMust(t, owner.Close())
		closeBase()
	}()
	// Warm plugin and reference storage before auditing constructor cleanup.
	warm, err := NewContextWithTheory(m, nil, 2, ReachableNoCache, cfg, shared)
	closureMust(t, err)
	closureMust(t, warm.Close())
	baseline := m.Stats().LiveReferences
	failures := 0
	for n := int64(0); n < 200; n++ {
		m.Budget().FailAfter(n)
		context, failure := NewContextWithTheory(m, nil, 2, ReachableNoCache, cfg, shared)
		m.Budget().FailAfter(-1)
		if failure == nil {
			closureMust(t, context.Close())
			break
		}
		failures++
		if !errors.Is(failure, arena.ErrInjectedFailure) {
			t.Fatalf("allocation point %d: %v", n, failure)
		}
		if context != nil {
			closureMust(t, context.Close())
		}
		if shared.Manager() != m {
			t.Fatal("constructor cleanup closed shared theory")
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatalf("point %d leaked AST references: %d != %d", n, got, baseline)
		}
	}
	if failures == 0 {
		t.Fatal("no constructor failure was reached")
	}
	retry, err := NewContextWithTheory(m, nil, 2, ReachableNoCache, cfg, shared)
	closureMust(t, err)
	closureMust(t, retry.Close())
}

func TestPDRContextOwnedRewriterConstructorFailuresRecover(t *testing.T) {
	m, owner, old, _, closeBase := closureFixture(t)
	cfg := owner.contexts.config
	defer func() { closureMust(t, old.Close()); closureMust(t, owner.Close()); closeBase() }()
	warm, err := NewContext(m, nil, 2, ReachableNoCache, cfg)
	closureMust(t, err)
	closureMust(t, warm.Close())
	baseline := m.Stats().LiveReferences
	failures := 0
	for n := int64(0); n < 300; n++ {
		m.Budget().FailAfter(n)
		context, failure := NewContext(m, nil, 2, ReachableNoCache, cfg)
		m.Budget().FailAfter(-1)
		if failure == nil {
			closureMust(t, context.Close())
			break
		}
		failures++
		if !errors.Is(failure, arena.ErrInjectedFailure) {
			t.Fatalf("allocation point %d: %v", n, failure)
		}
		if context != nil {
			closureMust(t, context.Close())
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatalf("point %d leaked AST references: %d != %d", n, got, baseline)
		}
	}
	if failures == 0 {
		t.Fatal("no owned constructor failure was reached")
	}
	retry, err := NewContext(m, nil, 2, ReachableNoCache, cfg)
	closureMust(t, err)
	closureMust(t, retry.Close())
}
