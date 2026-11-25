package smtcore

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExpressionBindingFullWidthValues(t *testing.T) {
	m := astTestManager(t)
	c, e := NewSMTCheckerContext(m)
	astTestMust(t, e)
	h := smtRelevancyBool(t, m, "full-width-value")
	b := smtBoolVarTable{c.exprs}
	cache := smtCacheGenerationTable{t: c.exprs}
	for _, v := range []int32{0, 1<<24 - 2, 1<<24 - 1, -1, math.MaxInt32, math.MinInt32, 37} {
		b.set(h, v)
		cache.set(h, uint32(v))
		c.relevant.set(h, true)
		if g, ok := cache.get(h); !ok || g != uint32(v) {
			t.Fatalf("cached %d found %t want %d", g, ok, uint32(v))
		}
		for _, assign := range []lbool.LBool{lbool.False, lbool.Undef, lbool.True} {
			c.assignments.set(h, assign)
			got, ok := c.assignments.lookup(h)
			if !ok || got != assign {
				t.Fatalf("assignment %d found %t, want %d", got, ok, assign)
			}
			if got, ok := b.lookup(h); !ok || got != v {
				t.Fatalf("binding %d found %t want %d", got, ok, v)
			}
			if !c.relevant.get(h) {
				t.Fatal("assignment changed relevance")
			}
		}
		cache.reset()
		if _, ok := cache.get(h); ok {
			t.Fatal("cached generation survived reset")
		}
		if got, ok := b.lookup(h); !ok || got != v {
			t.Fatal("cache reset changed variable", v, got, ok)
		}
	}
	b.del(h)
	if b.count() != 0 {
		t.Fatal("retired variable counted")
	}
}

func TestExpressionCacheColumnsAcrossSlotReuse(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	old, err := m.MkVar(231, sort)
	astTestMust(t, err)
	ref, err := m.NewRef(old)
	astTestMust(t, err)
	first := smtCacheGenerationTable{t: newSMTExprTable()}
	second := smtCacheGenerationTable{t: newSMTExprTable()}
	first.set(old, math.MaxUint32)
	first.visit(old)
	astTestMust(t, m.DelRef(&ref))
	live, err := m.MkVar(232, sort)
	astTestMust(t, err)
	if live.Slot() != old.Slot() || live.Generation() == old.Generation() {
		t.Fatal("fixture requires a recycled slot")
	}
	bindings := smtBoolVarTable{first.t}
	bindings.set(live, math.MaxInt32)
	second.set(live, 0)
	if _, ok := first.get(live); ok {
		t.Fatal("new generation inherited the retired cache entry")
	}
	first.reset()
	if got, ok := second.get(live); !ok || got != 0 {
		t.Fatal("one solver's reset changed another solver's entry")
	}
	if got, ok := bindings.lookup(live); !ok || got != math.MaxInt32 {
		t.Fatal("stale visit cleared the live variable binding")
	}
	first.set(live, math.MaxUint32)
	if first.visit(live) || !first.visit(live) {
		t.Fatal("visit did not start fresh after slot reuse")
	}
	if got, ok := first.get(live); !ok || got != math.MaxUint32 {
		t.Fatal("new cache entry lost its generation")
	}
}
