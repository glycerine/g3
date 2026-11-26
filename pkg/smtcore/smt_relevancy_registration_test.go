package smtcore

import (
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTRelevancyRegistrationRefusal(t *testing.T) {
	for _, name := range []string{"dependency", "positive watch", "negative watch"} {
		t.Run(name, func(t *testing.T) {
			var register func(ast.ASTHandle) error
			var inner ast.ASTHandle
			var refused error
			m, _, _, trigger := smtQuantifierRefusalFixture(t, func() { refused = register(inner) })
			ctx, err := NewSMTCheckerContext(m)
			astTestMust(t, err)
			p, err := NewSMTRelevancyPropagator(ctx, true)
			astTestMust(t, err)
			source := smtRelevancyPinnedBool(t, m, "source")
			outer := smtRelevancyPinnedBool(t, m, "outer")
			inner = smtRelevancyPinnedBool(t, m, "inner")
			table := &p.handlers
			register = func(target ast.ASTHandle) error { return p.AddDependency(source, target) }
			positive := name == "positive watch"
			if name != "dependency" {
				table = &p.negativeWatch
				if positive {
					table = &p.positiveWatch
				}
				// The internalizer also installs watches directly on known-live children.
				register = func(target ast.ASTHandle) error {
					return p.addWatch(source, positive, &smtRelevancyHandler{kind: smtRelevancySimple, target: target})
				}
			}
			refs := func() uint32 {
				n, err := m.GetRefCount(source)
				astTestMust(t, err)
				return n
			}
			base := refs()
			astTestMust(t, register(outer))
			head := table.head(source)
			astTestMust(t, p.PushScope())
			trigger()
			if refused != ast.ErrASTPending || table.head(source) != head || len(p.ehs) != 1 || len(p.trail) != 1 || refs() != base+1 {
				t.Fatalf("refused registration: err=%v head=%d (outer %d) handlers=%d trail=%d refs=%d (want %d)",
					refused, table.head(source), head, len(p.ehs), len(p.trail), refs(), base+1)
			}
			astTestMust(t, register(inner))
			if table.link(table.head(source)).previous != head || len(p.ehs) != 2 || len(p.trail) != 2 || refs() != base+2 {
				t.Fatal("retry must install exactly one link and retain its source once")
			}
			want := []ast.ASTHandle{inner, outer}
			if name == "dependency" {
				astTestMust(t, p.MarkRelevant(source))
				astTestMust(t, p.Propagate())
				want = []ast.ASTHandle{source, inner, outer}
			} else {
				astTestMust(t, ctx.SetAssignment(source, lbool.FromBool(positive)))
				astTestMust(t, p.AssignmentEvent(source, positive))
			}
			if got := p.DrainNewlyRelevant(); !slices.Equal(got, want) {
				t.Fatalf("delivery after retry %v, want %v", got, want)
			}
			astTestMust(t, p.PopScopes(1))
			if table.head(source) != head || refs() != base+1 {
				t.Fatal("pop did not restore the outer registration")
			}
			astTestMust(t, p.release())
			if table.head(source) != 0 || refs() != base {
				t.Fatal("release did not retire the outer registration")
			}
		})
	}
}

func TestSMTRelevancyMarkRefusal(t *testing.T) {
	var p *SMTRelevancyPropagator
	var source ast.ASTHandle
	var refused error
	var mark func(ast.ASTHandle) error
	m, _, _, trigger := smtQuantifierRefusalFixture(t, func() { refused = mark(source) })
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err = NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	source = smtRelevancyPinnedBool(t, m, "refused-mark")
	mark = p.MarkRelevant
	before, err := m.GetRefCount(source)
	astTestMust(t, err)
	astTestMust(t, p.PushScope())
	trigger()
	n, err := m.GetRefCount(source)
	astTestMust(t, err)
	if refused != ast.ErrASTPending || p.IsRelevant(source) || len(p.relevant) != 0 || len(p.newlyRelevant) != 0 || ctx.exprs.relevanceAdditions != 0 || n != before {
		t.Fatalf("refused mark: err=%v marked=%t owned=%d queued=%d additions=%d refs=%d",
			refused, p.IsRelevant(source), len(p.relevant), len(p.newlyRelevant), ctx.exprs.relevanceAdditions, n)
	}
	astTestMust(t, p.MarkRelevant(source))
	// Equality-class traversal can visit an already marked member. A refused
	// duplicate must leave the notification cursor and addition count alone.
	mark = p.setRelevant
	refused = nil
	trigger()
	if refused != ast.ErrASTPending || p.newlyRelevantBase != 0 || ctx.exprs.relevanceAdditions != 1 || len(p.relevant) != 1 {
		t.Fatalf("refused duplicate mark: err=%v base=%d additions=%d owned=%d",
			refused, p.newlyRelevantBase, ctx.exprs.relevanceAdditions, len(p.relevant))
	}
	if got := p.DrainNewlyRelevant(); !slices.Equal(got, []ast.ASTHandle{source}) {
		t.Fatalf("retry delivered %v", got)
	}
	astTestMust(t, p.PopScopes(1))
	n, err = m.GetRefCount(source)
	astTestMust(t, err)
	if n != before || p.IsRelevant(source) {
		t.Fatal("pop did not release the retried mark")
	}
}

func TestSMTRelevancyPropagationRetriesRefusedDependency(t *testing.T) {
	var p *SMTRelevancyPropagator
	var refused error
	m, _, _, trigger := smtQuantifierRefusalFixture(t, func() { refused = p.Propagate() })
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err = NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.release()) }()
	source := smtRelevancyPinnedBool(t, m, "propagation-source")
	target := smtRelevancyPinnedBool(t, m, "propagation-target")
	child := smtRelevancyPinnedBool(t, m, "propagation-child")
	astTestMust(t, p.AddDependency(source, target))
	astTestMust(t, p.AddDependency(target, child))
	astTestMust(t, p.PushScope())
	astTestMust(t, p.MarkRelevant(source))

	trigger()
	if refused != ast.ErrASTPending || p.IsRelevant(target) || p.IsRelevant(child) {
		t.Fatalf("refused propagation: err=%v target=%t child=%t", refused, p.IsRelevant(target), p.IsRelevant(child))
	}
	if !p.CanPropagate() {
		t.Error("refused propagation lost the pending source")
	}
	astTestMust(t, p.Propagate())
	if p.CanPropagate() || !p.IsRelevant(target) || !p.IsRelevant(child) {
		t.Error("retry did not mark both reachable dependencies")
	}
	if got, want := p.DrainNewlyRelevant(), []ast.ASTHandle{source, target, child}; !slices.Equal(got, want) {
		t.Errorf("notifications after retry %v, want %v", got, want)
	}
	astTestMust(t, p.PopScopes(1))
	if len(p.RelevantExpressions()) != 0 {
		t.Fatal("pop did not remove the retried marks")
	}
}

func TestSMTRelevancyPairRejectsForeignSecondSource(t *testing.T) {
	m, other := astTestManager(t), astTestManager(t)
	a := smtRelevancyPinnedBool(t, m, "first")
	b := smtRelevancyPinnedBool(t, other, "foreign")
	target := smtRelevancyPinnedBool(t, m, "target")
	for _, name := range []string{"single target", "multiple targets"} {
		t.Run(name, func(t *testing.T) {
			ctx, err := NewSMTCheckerContext(m)
			astTestMust(t, err)
			p, err := NewSMTRelevancyPropagator(ctx, true)
			astTestMust(t, err)
			defer func() { astTestMust(t, p.release()) }()
			before, err := m.GetRefCount(a)
			astTestMust(t, err)
			if name == "single target" {
				err = p.AddPairDependency(a, b, target)
			} else {
				err = p.AddPairTargetsDependency(a, b, a, target)
			}
			if err != ast.ErrASTHandle {
				t.Fatalf("foreign source returned %v", err)
			}
			after, err := m.GetRefCount(a)
			astTestMust(t, err)
			if after != before || p.handlers.head(a) != 0 || p.handlers.head(b) != 0 || len(p.ehs) != 0 || len(p.trail) != 0 {
				t.Fatalf("failed pair retained %d extra references, %d handlers and %d trail entries", after-before, len(p.ehs), len(p.trail))
			}
		})
	}
}

func TestSMTRelevancyDependencyRejectsRecycledSource(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyPinnedBool(t, m, "old-source-operand")
	b := smtRelevancyPinnedBool(t, m, "live-source-operand")
	target := smtRelevancyPinnedBool(t, m, "live-target")
	refusedTarget := smtRelevancyPinnedBool(t, m, "refused-target")
	old, err := m.MkNot(a)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(old))
	astTestMust(t, m.DecRef(old))
	live, err := m.MkNot(b)
	astTestMust(t, err)
	astTestPin(t, m, live)
	if live.Slot() != old.Slot() || live.Generation() == old.Generation() {
		t.Fatal("fixture requires a recycled source slot")
	}

	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err := NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	before, err := m.GetRefCount(live)
	astTestMust(t, err)
	astTestMust(t, p.AddDependency(live, target))
	head := p.handlers.head(live)
	astTestMust(t, p.PushScope())
	refused := p.AddDependency(old, refusedTarget)
	if got := p.handlers.head(live); refused != ast.ErrASTHandle || got != head || p.handlers.head(old) != 0 || len(p.ehs) != 1 || len(p.trail) != 1 {
		t.Fatalf("stale dependency: err=%v live head=%d (want %d) handlers=%d trail=%d", refused, got, head, len(p.ehs), len(p.trail))
	}
	astTestMust(t, p.MarkRelevant(live))
	astTestMust(t, p.Propagate())
	if !p.IsRelevant(target) || p.IsRelevant(refusedTarget) {
		t.Fatal("the live dependency must fire after stale registration is refused")
	}
	astTestMust(t, p.PopScopes(1))
	astTestMust(t, p.release())
	after, err := m.GetRefCount(live)
	astTestMust(t, err)
	if p.handlers.head(live) != 0 || after != before {
		t.Fatalf("release left a handler or source reference: head=%d refs=%d (want %d)", p.handlers.head(live), after, before)
	}
}

func TestSMTRelevancyWatchRejectsRecycledNot(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyPinnedBool(t, m, "old-operand")
	b := smtRelevancyPinnedBool(t, m, "new-operand")
	target := smtRelevancyPinnedBool(t, m, "target")
	old, err := m.MkNot(a)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(old))
	astTestMust(t, m.DecRef(old))
	live, err := m.MkNot(b)
	astTestMust(t, err)
	astTestPin(t, m, live)
	if live.Slot() != old.Slot() || old.Generation() == live.Generation() {
		t.Fatal("fixture requires a recycled NOT slot")
	}
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err := NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.release()) }()
	astTestMust(t, ctx.SetAssignment(b, lbool.False))
	if err := p.AddWatchTarget(old, true, target); err != ast.ErrASTHandle || p.IsRelevant(target) {
		t.Fatalf("stale NOT watch: err=%v target marked=%t", err, p.IsRelevant(target))
	}
	astTestMust(t, p.AddWatchTarget(live, true, target))
	if !p.IsRelevant(target) || len(p.trail) != 0 {
		t.Fatal("live satisfied watch must fire immediately")
	}
}

func TestSMTRelevancyGateWatchesRejectRecycledParent(t *testing.T) {
	m := astTestManager(t)
	a := smtRelevancyPinnedBool(t, m, "old-child")
	b := smtRelevancyPinnedBool(t, m, "shared-child")
	c := smtRelevancyPinnedBool(t, m, "new-child")
	old, err := m.MkOr(a, b)
	astTestMust(t, err)
	astTestMust(t, m.IncRef(old))
	astTestMust(t, m.DecRef(old))
	live, err := m.MkOr(c, b)
	astTestMust(t, err)
	astTestPin(t, m, live)
	if live.Slot() != old.Slot() || old.Generation() == live.Generation() {
		t.Fatal("fixture requires a recycled OR slot")
	}
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err := NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.release()) }()
	if err := p.AddOrWatches(old); err != ast.ErrASTHandle || len(p.trail) != 0 {
		t.Fatalf("stale OR parent: err=%v registered watches=%d", err, len(p.trail))
	}
	astTestMust(t, p.AddOrWatches(live))
	if p.positiveWatch.head(c) == 0 || p.positiveWatch.head(b) == 0 || len(p.trail) != 2 {
		t.Fatal("live OR must install one watch per child")
	}
}
