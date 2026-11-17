package smtcore

import "testing"

// Removing a fingerprint must release both its ordered-table slot and its
// hash-bucket slot. Otherwise a popped binding keeps an entire enode graph
// alive even though it is no longer a member of the set.
func TestSMTQuantifierFingerprintPopReleasesBackingSlots(t *testing.T) {
	m := astTestManager(t)
	base := smtQIQueueFingerprint(t, m, "retained-fp-base")
	scoped := smtQIQueueFingerprint(t, m, "retained-fp-scoped")
	set := NewSMTFingerprintSet()
	first, e := set.Insert(base.data, 0, nil)
	astTestMust(t, e)
	astTestMust(t, set.PushScope())
	second, e := set.Insert(scoped.data, 0, nil)
	astTestMust(t, e)
	// Exercise the collision-bucket path deterministically. Finding an actual
	// 64-bit collision is irrelevant to the lifetime contract being checked.
	delete(set.byHash, second.hash)
	second.hash = first.hash
	set.byHash[first.hash] = append(set.byHash[first.hash], second)
	ordered, bucket := set.fingerprints, set.byHash[first.hash]
	astTestMust(t, set.PopScopes(1))
	if set.Size() != 1 || set.fingerprints[0] != first || len(set.byHash[first.hash]) != 1 || set.byHash[first.hash][0] != first {
		t.Fatal("pop damaged a surviving fingerprint")
	}
	if ordered[1] != nil {
		t.Error("popped fingerprint retained in ordered backing array")
	}
	if bucket[1] != nil {
		t.Error("popped fingerprint retained in collision-bucket backing array")
	}
}

func TestSMTQuantifierDelayedPopReleasesBackingSlots(t *testing.T) {
	m := astTestManager(t)
	base := smtQIQueueFingerprint(t, m, "retained-queue-base")
	scoped := smtQIQueueFingerprint(t, m, "retained-queue-scoped")
	q := NewSMTQIQueueSchedule(m, 0, 0, false)
	astTestMust(t, q.Insert(base, 2, 0))
	_, e := q.ScheduleNew(nil)
	astTestMust(t, e)
	astTestMust(t, q.PushScope())
	astTestMust(t, q.Insert(scoped, 3, 0))
	_, e = q.ScheduleNew(nil)
	astTestMust(t, e)
	backing := q.delayedEntries
	astTestMust(t, q.PopScopes(1))
	if len(q.delayedEntries) != 1 || q.delayedEntries[0].Fingerprint != base {
		t.Fatal("pop damaged the surviving delayed entry")
	}
	if backing[1].Fingerprint != nil {
		t.Fatal("popped delayed entry retains its fingerprint and bindings")
	}
}
