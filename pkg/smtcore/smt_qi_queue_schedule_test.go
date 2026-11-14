package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func smtQIQueueFingerprint(t *testing.T, m *ast.ASTManager, name string) *SMTFingerprint {
	t.Helper()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	quantifier, err := m.MkConst(astTestSymbol(t, m, name), boolean)
	astTestMust(t, err)
	return &SMTFingerprint{data: quantifier}
}

func TestSMTQIQueueScheduleEagerPromotionAndLazyThreshold(t *testing.T) {
	m := astTestManager(t)
	eager := smtQIQueueFingerprint(t, m, "qi-eager")
	delayed := smtQIQueueFingerprint(t, m, "qi-delayed")
	promoted := smtQIQueueFingerprint(t, m, "qi-promoted")
	queue := NewSMTQIQueueSchedule(m, 1, 2.5, false)
	astTestMust(t, queue.Insert(eager, .5, 1))
	astTestMust(t, queue.Insert(delayed, 2, 2))
	astTestMust(t, queue.Insert(promoted, 9, 3))
	if !queue.HasWork() {
		t.Fatal("new entry work flag")
	}
	ready, err := queue.ScheduleNew(map[*SMTFingerprint]bool{promoted: true})
	astTestMust(t, err)
	if queue.HasWork() || len(ready) != 2 || ready[0].Fingerprint != eager || ready[1].Fingerprint != promoted || len(queue.delayedEntries) != 1 {
		t.Fatal("eager and promoted scheduling", ready, queue.delayedEntries)
	}
	done, lazy, err := queue.FinalCheck()
	astTestMust(t, err)
	if done || len(lazy) != 1 || lazy[0].Fingerprint != delayed || !lazy[0].Instantiated {
		t.Fatal("lazy threshold scheduling", done, lazy)
	}
	done, lazy, err = queue.FinalCheck()
	astTestMust(t, err)
	if !done || len(lazy) != 0 {
		t.Fatal("instantiated delayed entry repeated", done, lazy)
	}
	stats := queue.Stats()
	if stats.LazyInstances != 1 || stats.MissedInstances != 1 || stats.MinimumMissedCost != 0 || stats.MaximumMissedCost != 0 {
		t.Fatal("queue statistics", stats)
	}
}

func TestSMTQIQueueScheduleConservativeCostBatches(t *testing.T) {
	m := astTestManager(t)
	first := smtQIQueueFingerprint(t, m, "qi-first")
	second := smtQIQueueFingerprint(t, m, "qi-second")
	third := smtQIQueueFingerprint(t, m, "qi-third")
	queue := NewSMTQIQueueSchedule(m, 0, 5, true)
	for _, item := range []struct {
		fingerprint *SMTFingerprint
		cost        float32
	}{{first, 2}, {second, 2}, {third, 3}} {
		astTestMust(t, queue.Insert(item.fingerprint, item.cost, 0))
	}
	_, err := queue.ScheduleNew(nil)
	astTestMust(t, err)
	done, batch, err := queue.FinalCheck()
	astTestMust(t, err)
	if done || len(batch) != 2 || batch[0].Cost != 2 || batch[1].Cost != 2 {
		t.Fatal("minimum conservative batch", done, batch)
	}
	done, batch, err = queue.FinalCheck()
	astTestMust(t, err)
	if done || len(batch) != 1 || batch[0].Fingerprint != third {
		t.Fatal("next conservative batch", done, batch)
	}
	done, batch, err = queue.FinalCheck()
	astTestMust(t, err)
	if !done || len(batch) != 0 {
		t.Fatal("conservative completion", done, batch)
	}
}

func TestSMTQIQueueScheduleScopeRollbackAndDelayedSummaries(t *testing.T) {
	m := astTestManager(t)
	quantifier := smtQIQueueFingerprint(t, m, "qi-summary")
	other := smtQIQueueFingerprint(t, m, "qi-other")
	first := &SMTFingerprint{data: quantifier.data}
	second := &SMTFingerprint{data: quantifier.data, dataHash: 1}
	queue := NewSMTQIQueueSchedule(m, 0, 10, false)
	astTestMust(t, queue.Insert(first, 4, 0))
	astTestMust(t, queue.Insert(second, 2, 0))
	_, err := queue.ScheduleNew(nil)
	astTestMust(t, err)
	summaries := queue.DelayedSummaries()
	if len(summaries) != 1 || summaries[0].Count != 2 || summaries[0].MinimumCost != 2 || summaries[0].MaximumCost != 2 {
		t.Fatal("source delayed summary aggregation", summaries)
	}
	stats := queue.Stats()
	if stats.MinimumMissedCost != 2 || stats.MaximumMissedCost != 4 || stats.MissedInstances != 2 {
		t.Fatal("missed cost range", stats)
	}
	astTestMust(t, queue.PushScope())
	astTestMust(t, queue.Insert(other, 6, 0))
	_, err = queue.ScheduleNew(nil)
	astTestMust(t, err)
	lemma, err := m.MkTrue()
	astTestMust(t, err)
	astTestMust(t, queue.RecordInstance(lemma))
	done, selected, err := queue.FinalCheck()
	astTestMust(t, err)
	if done || len(selected) != 3 {
		t.Fatal("scoped delayed selection", done, selected)
	}
	astTestMust(t, queue.PopScopes(1))
	if len(queue.delayedEntries) != 2 || len(queue.instances) != 0 || queue.delayedEntries[0].Instantiated || queue.delayedEntries[1].Instantiated {
		t.Fatal("scope rollback", queue.delayedEntries, queue.instances)
	}
	if err := queue.PopScopes(1); err != ast.ErrASTDomain {
		t.Fatal("scope underflow guard", err)
	}
}

func TestSMTQIQueueOwnsRecordedLemmasThroughScopeAndReset(t *testing.T) {
	m := astTestManager(t)
	queue := NewSMTQIQueueSchedule(m, 1, 2, false)
	t.Cleanup(func() { astTestMust(t, queue.Reset()) })
	atom := smtRelevancyBool(t, m, "owned-lemma-atom")
	base, err := m.MkNot(atom)
	astTestMust(t, err)
	baseRef, err := m.NewRef(base)
	astTestMust(t, err)
	astTestMust(t, queue.RecordInstance(base))
	astTestMust(t, m.DelRef(&baseRef))
	if _, err = m.GetKind(base); err != nil {
		t.Fatal("recorded lemma did not outlive its producer", err)
	}
	astTestMust(t, queue.PushScope())
	other := smtRelevancyBool(t, m, "owned-lemma-other")
	scoped, err := m.MkOr(base, other)
	astTestMust(t, err)
	scopedRef, err := m.NewRef(scoped)
	astTestMust(t, err)
	astTestMust(t, queue.RecordInstance(scoped))
	astTestMust(t, m.DelRef(&scopedRef))
	if _, err = m.GetKind(scoped); err != nil {
		t.Fatal("scoped lemma did not outlive its producer", err)
	}
	astTestMust(t, queue.PopScopes(1))
	if m.IsLiveHandle(scoped) {
		t.Fatal("popped lemma retained its AST ownership")
	}
	if _, err = m.GetKind(base); err != nil {
		t.Fatal("scope rollback released the base lemma", err)
	}
	astTestMust(t, queue.Reset())
	if m.IsLiveHandle(base) {
		t.Fatal("reset retained the base lemma")
	}
}
