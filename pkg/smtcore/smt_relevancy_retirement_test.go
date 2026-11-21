package smtcore

import (
	"fmt"
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
	astTestMust(t, m.IncRef(middle))
	// Closing several scopes must finish the interrupted inner one first.
	astTestMust(t, p.PopScopes(2))
	if !m.IsLiveHandle(first) || refs(first) != 1 || refs(last) != 1 || refs(outer) != 1 || m.IsLiveHandle(middle) {
		t.Fatal("retry consumed an external reference or retained a popped reference")
	}
	if len(p.scopes) != 0 || len(p.relevant) != 0 || p.IsRelevant(outer) || p.CanPropagate() {
		t.Fatal("retry left a nonempty scope")
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
