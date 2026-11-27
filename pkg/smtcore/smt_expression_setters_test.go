package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExpressionSettersPreserveLiveSlot(t *testing.T) {
	m, other := astTestManager(t), astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	relevance, err := NewSMTRelevantExprContext(m)
	astTestMust(t, err)
	_, _, solver := smtInternalizerFixture(t, m, nil)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	retired, err := m.MkVar(100, sort)
	astTestMust(t, err)
	hold, err := m.NewRef(retired)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&hold))
	live, err := m.MkVar(101, sort)
	astTestMust(t, err)
	if live.Slot() != retired.Slot() || live.Generation() == retired.Generation() {
		t.Fatal("fixture requires two generations of the same slot")
	}
	foreign := smtRelevancyBool(t, other, "other-manager")
	for _, c := range []struct {
		name string
		set  func(ast.ASTHandle) error
	}{
		{"checker assignment", func(h ast.ASTHandle) error { return ctx.SetAssignment(h, lbool.True) }},
		{"checker relevance", func(h ast.ASTHandle) error { return ctx.SetRelevant(h, true) }},
		{"cached generation", func(h ast.ASTHandle) error { return solver.SetCachedGeneration(h, 7) }},
		{"expression assignment", func(h ast.ASTHandle) error { return relevance.SetAssignment(h, lbool.True) }},
		{"expression relevance", func(h ast.ASTHandle) error { return relevance.SetRelevant(h, true) }},
		{"expression retirement", relevance.SetNotInternalized},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, bad := range []ast.ASTHandle{foreign, retired, {}} {
				if err := c.set(bad); !errors.Is(err, ast.ErrASTHandle) {
					t.Fatalf("invalid handle returned %v", err)
				}
			}
			astTestMust(t, c.set(live))
			if err := c.set(retired); !errors.Is(err, ast.ErrASTHandle) {
				t.Fatalf("writing retired generation over live slot returned %v", err)
			}
		})
	}
	if v, ok := ctx.assignments.lookup(live); !ok || v != lbool.True || !ctx.relevant.get(live) {
		t.Fatal("invalid write disturbed checker state")
	}
	if g, ok := solver.cachedGeneration.get(live); !ok || g != 7 {
		t.Fatal("invalid write disturbed cached generation")
	}
	if !relevance.relevant[live] || len(relevance.relevant) != 1 || len(relevance.assignments) != 0 || len(relevance.internalized) != 0 {
		t.Fatal("invalid write disturbed the standalone relevance context")
	}
}

func TestRegisterBorrowedEnodePreservesLiveSlot(t *testing.T) {
	m := astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p := smtRelevancyBool(t, m, "borrowed-enode")
	astTestPin(t, m, p)
	newNode := func() (ast.ASTHandle, *SMTEnode) {
		owner, err := m.MkNot(p)
		astTestMust(t, err)
		node, err := NewSMTEnode(m, owner, nil, SMTEnodeOptions{SuppressArguments: true})
		astTestMust(t, err)
		return owner, node
	}
	old, stale := newNode()
	hold, err := m.NewRef(old)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&hold))
	live, current := newNode()
	astTestPin(t, m, live)
	if old.Slot() != live.Slot() || old.Generation() == live.Generation() {
		t.Fatal("expected a new generation in the retired slot")
	}
	astTestMust(t, ctx.RegisterEnode(current, true))
	err = ctx.RegisterEnode(stale, false)
	if !errors.Is(err, ast.ErrASTHandle) || ctx.enodes.get(live) != current || !ctx.relevant.get(live) || len(ctx.registeredEnodes) != 1 {
		t.Fatalf("stale registration returned %v, live node preserved %t, relevant %t, registered %d",
			err, ctx.enodes.get(live) == current, ctx.relevant.get(live), len(ctx.registeredEnodes))
	}
}
