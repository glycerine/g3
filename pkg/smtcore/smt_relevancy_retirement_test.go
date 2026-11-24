package smtcore

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTRelevancyBaseMarksSurviveScopedInternalization(t *testing.T) {
	for _, baseBoolean := range []bool{false, true} {
		t.Run(fmt.Sprint(baseBoolean), func(t *testing.T) {
			m, _ := smtModelGeneratorFixture(t)
			_, p, s := smtInternalizerFixture(t, m, nil)
			expr := smtRelevancyBool(t, m, "base-mark-scoped-owner")
			root, err := m.NewRef(expr)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&root)) })
			if baseBoolean {
				astTestMust(t, s.Internalize(expr, true))
			}
			astTestMust(t, p.MarkRelevant(expr))
			astTestMust(t, p.Propagate())
			limit := len(p.relevant)
			astTestMust(t, s.PushScope())
			astTestMust(t, s.Internalize(expr, false))
			if s.context.enodes.get(expr) == nil {
				t.Fatal("scoped enode not created")
			}
			if !p.IsRelevant(expr) {
				t.Fatal("native owner creation reset an earlier expression relevancy mark")
			}
			astTestMust(t, s.PopScopes(1))
			if s.context.enodes.get(expr) != nil {
				t.Fatal("scoped enode survived")
			}
			_, exists := s.BoolVar(expr)
			if exists != baseBoolean {
				t.Fatal("Boolean lifetime did not follow creation scope")
			}
			if !p.IsRelevant(expr) || len(p.relevant) != limit || p.relevant[limit-1] != expr {
				t.Fatal("native owner retirement erased an earlier expression relevancy mark")
			}
			astTestMust(t, s.Internalize(expr, false))
			if !p.IsRelevant(expr) {
				t.Fatal("recreated owner lost base relevance")
			}
		})
	}
}

// A scoped batch installs one simple watch on each of many distinct terms.
// z3's region allocates their list cells together; the table's cells come
// from shared pages, so the batch needs few allocations.
func TestSMTRelevancyScopedWatchAllocations(t *testing.T) {
	m := astTestManager(t)
	var owners [512]ast.ASTHandle
	for i := range owners {
		owners[i] = smtRelevancyPinnedBool(t, m, fmt.Sprint("batch-watch-", i))
	}
	allocs := testing.AllocsPerRun(10, func() {
		var table smtRelevancyTable
		for i, h := range owners {
			table.push(h, uint32(i+1))
		}
		for i := len(owners) - 1; i >= 0; i-- {
			table.pop(owners[i])
		}
	})
	if allocs > 32 {
		t.Fatalf("512 scoped watches required %.0f allocations", allocs)
	}
}

func TestSMTRelevancyPopRetriesOnlyUnconsumedOwners(t *testing.T) {
	m := astTestManager(t)
	context, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err := NewSMTRelevancyPropagator(context, true)
	astTestMust(t, err)
	outer := smtRelevancyPinnedBool(t, m, "outer")
	first := smtRelevancyPinnedBool(t, m, "first")
	middle := smtRelevancyBool(t, m, "refused")
	last := smtRelevancyPinnedBool(t, m, "last")
	astTestMust(t, p.PushScope())
	astTestMust(t, p.MarkRelevant(outer))
	astTestMust(t, p.PushScope())
	astTestMust(t, p.MarkRelevant(first))
	// The middle handle is valid at RC zero. Its refused decrement forces
	// the same unconsumed result as an unavailable AST manager, after first.
	astTestMust(t, context.SetRelevant(middle, true))
	p.relevant = append(p.relevant, middle)
	astTestMust(t, p.MarkRelevant(last))
	if err := p.PopScopes(1); err != ast.ErrASTDomain {
		t.Fatalf("middle decrement returned %v", err)
	}
	refs := func(h ast.ASTHandle) uint32 { n, e := m.GetRefCount(h); astTestMust(t, e); return n }
	if refs(first) != 1 || refs(last) != 2 {
		t.Fatal("wrong ownership after refused decrement", refs(first), refs(last))
	}
	visible := p.RelevantExpressions()
	if len(visible) != 1 || visible[0] != outer || p.CanPropagate() || len(p.DrainNewlyRelevant()) != 0 {
		t.Fatal("interrupted pop exposed retired relevance marks", visible)
	}
	// Callers can re-mark a live scoped handle directly through the checker
	// while the propagator is waiting to retry the refused decrement.
	astTestMust(t, context.SetRelevant(last, true))
	astTestMust(t, m.IncRef(middle))
	// Closing several scopes must finish the interrupted inner one first.
	astTestMust(t, p.PopScopes(2))
	if !m.IsLiveHandle(first) || refs(first) != 1 || refs(last) != 1 || refs(outer) != 1 || m.IsLiveHandle(middle) {
		t.Fatal("retry consumed an external reference or retained a popped reference")
	}
	if len(p.scopes) != 0 || len(p.relevant) != 0 || p.IsRelevant(outer) || p.CanPropagate() {
		t.Fatal("retry left a nonempty scope")
	}
	if got := p.DrainNewlyRelevant(); !slices.Equal(got, []ast.ASTHandle{last}) {
		t.Fatalf("lost reactivated notification after retry: %v", got)
	}
}

func TestSMTRelevancyPendingQueueMatchesMarkTable(t *testing.T) {
	m := astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err := NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	var handles [12]ast.ASTHandle
	for i := range handles {
		handles[i] = smtRelevancyPinnedBool(t, m, fmt.Sprintf("pending-%d", i))
	}
	// Replay against an independent trail and filter the entire reference
	// queue after each pop, including duplicate equal-class notifications.
	rng := rand.New(rand.NewSource(240))
	live := make(map[ast.ASTHandle]bool)
	var trail, queue []ast.ASTHandle
	var limits []int
	for step := 0; step < 12000; step++ {
		h := handles[rng.Intn(len(handles))]
		switch rng.Intn(10) {
		case 0, 1:
			if len(limits) < 8 {
				astTestMust(t, p.PushScope())
				limits = append(limits, len(trail))
			}
		case 2, 3:
			if len(limits) > 0 {
				n := 1 + rng.Intn(len(limits))
				astTestMust(t, p.PopScopes(uint32(n)))
				level := len(limits) - n
				base := limits[level]
				for _, term := range trail[base:] {
					delete(live, term)
				}
				limits, trail = limits[:level], trail[:base]
				kept := queue[:0]
				for _, term := range queue {
					if live[term] {
						kept = append(kept, term)
					}
				}
				queue = kept
			}
		case 4:
			value := rng.Intn(2) == 0
			astTestMust(t, ctx.SetRelevant(h, value))
			live[h] = value
		case 5:
			if got := p.DrainNewlyRelevant(); !slices.Equal(got, queue) {
				t.Fatalf("drain at step %d: got %v, want %v", step, got, queue)
			}
			queue = nil
		case 6:
			p.discardNewlyRelevant()
			queue = nil
		case 7:
			astTestMust(t, p.MarkRelevant(h))
			if live[h] {
				break
			}
			live[h] = true
			trail = append(trail, h)
			queue = append(queue, h)
		default:
			astTestMust(t, p.setRelevant(h))
			live[h] = true
			trail = append(trail, h)
			queue = append(queue, h)
		}
		if !slices.Equal(p.newlyRelevant, queue) {
			t.Fatalf("step %d: got %v, want %v", step, p.newlyRelevant, queue)
		}
	}
	astTestMust(t, p.release())
}

func TestSMTRelevancyPendingQueueAfterCallback(t *testing.T) {
	for _, action := range []string{"drain", "discard", "reactivate"} {
		t.Run(action, func(t *testing.T) {
			m := astTestManager(t)
			ctx, err := NewSMTCheckerContext(m)
			astTestMust(t, err)
			p, err := NewSMTRelevancyPropagator(ctx, true)
			astTestMust(t, err)
			base := smtRelevancyPinnedBool(t, m, "base")
			scoped := smtRelevancyPinnedBool(t, m, "scoped")
			astTestMust(t, p.MarkRelevant(base))
			astTestMust(t, p.PushScope())
			astTestMust(t, p.MarkRelevant(scoped))
			// Interleave the callback at the unmark/release boundary in PopScopes.
			additions := ctx.exprs.relevanceAdditions
			ctx.relevant.del(scoped)
			var want []ast.ASTHandle
			switch action {
			case "drain":
				if got := p.DrainNewlyRelevant(); !slices.Equal(got, []ast.ASTHandle{base, scoped}) {
					t.Fatalf("drain %v", got)
				}
			case "discard":
				p.discardNewlyRelevant()
			case "reactivate":
				astTestMust(t, ctx.SetRelevant(scoped, true))
				want = []ast.ASTHandle{base, scoped}
			}
			astTestMust(t, p.finishPop(0, 1, additions, true))
			if got := p.DrainNewlyRelevant(); !slices.Equal(got, want) {
				t.Fatalf("after callback: got %v, want %v", got, want)
			}
			astTestMust(t, p.release())
		})
	}
}

func TestSMTRelevancyTrailKeepsRefusedOwnerAfterUnlink(t *testing.T) {
	for _, flush := range []bool{false, true} {
		t.Run(fmt.Sprint("flush=", flush), func(t *testing.T) {
			var p *SMTRelevancyPropagator
			var refused error
			m, _, _, trigger := smtQuantifierRefusalFixture(t, func() {
				if flush {
					refused = p.release()
				} else {
					refused = p.PopScopes(1)
				}
			})
			context, e := NewSMTCheckerContext(m)
			astTestMust(t, e)
			p, e = NewSMTRelevancyPropagator(context, true)
			astTestMust(t, e)
			source := smtRelevancyPinnedBool(t, m, "refused-watch-source")
			target := smtRelevancyPinnedBool(t, m, "refused-watch-target")
			astTestMust(t, p.AddDependency(source, target))
			outer := p.handlers.head(source)
			astTestMust(t, p.PushScope())
			astTestMust(t, p.AddDependency(source, target))
			before, e := m.GetRefCount(source)
			astTestMust(t, e)
			trigger()
			if refused != ast.ErrASTPending || p.handlers.head(source) != outer || len(p.trail) != 2 {
				t.Fatalf("refused unlink %v, head %d, trail %d", refused, p.handlers.head(source), len(p.trail))
			}
			if flush {
				astTestMust(t, p.release())
			} else {
				astTestMust(t, p.PopScopes(1))
				after, e := m.GetRefCount(source)
				astTestMust(t, e)
				if p.handlers.head(source) != outer || len(p.trail) != 1 || after+1 != before {
					t.Fatal("retry unlinked or released the outer watch")
				}
				astTestMust(t, p.release())
			}
			final, e := m.GetRefCount(source)
			astTestMust(t, e)
			if p.handlers.head(source) != 0 || final+2 != before {
				t.Fatal("flush lost the retained watch reference", final, before)
			}
		})
	}
}

func TestSMTRelevancyPopOfRepeatedUndrainedMark(t *testing.T) {
	m := astTestManager(t)
	ctx, err := NewSMTCheckerContext(m)
	astTestMust(t, err)
	p, err := NewSMTRelevancyPropagator(ctx, true)
	astTestMust(t, err)
	a := smtRelevancyPinnedBool(t, m, "a")
	b := smtRelevancyPinnedBool(t, m, "b")
	c := smtRelevancyPinnedBool(t, m, "c")
	astTestMust(t, p.MarkRelevant(a))
	astTestMust(t, p.MarkRelevant(b))
	// Leave two base notifications pending across many sibling scopes.
	for range 10 {
		astTestMust(t, p.PushScope())
		astTestMust(t, p.MarkRelevant(c))
		astTestMust(t, p.PopScopes(1))
	}
	astTestMust(t, p.PushScope())
	// Equal-class events can mark a member a second time.
	astTestMust(t, p.setRelevant(a))
	astTestMust(t, p.MarkRelevant(c))
	astTestMust(t, p.PopScopes(1))
	if got := p.DrainNewlyRelevant(); !slices.Equal(got, []ast.ASTHandle{b}) {
		t.Fatalf("pending notifications after duplicate retires: %v", got)
	}
	astTestMust(t, p.release())
}
