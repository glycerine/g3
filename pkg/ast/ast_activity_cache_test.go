package ast

// Source: src/ast/act_cache.cpp:56-225; src/ast/act_cache.h:23-53.

import (
	"fmt"
	"testing"
)

func TestASTActivityCacheSourceOwnershipReplacementAndReset(t *testing.T) {
	m := astTestManager(t)
	limit := uint32(1)
	c, e := NewActivityCache(m, &limit)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	if c.maxUnused != 1024 || c.activityCacheEntries != nil {
		t.Fatal("new cache", c.maxUnused, c.activityCacheEntries)
	}
	size, e := c.Size()
	astTestMust(t, e)
	empty, e := c.Empty()
	astTestMust(t, e)
	if size != 0 || !empty {
		t.Fatal("source cache queries", size, empty)
	}
	p, q, r := activityCacheTestAtom(t, m, "cache-p"), activityCacheTestAtom(t, m, "cache-q"), activityCacheTestAtom(t, m, "cache-r")
	if found, err := c.Find(p); err != nil || !found.IsNull() || c.activityCacheEntries != nil {
		t.Fatal("lookup in unused cache", found, err)
	}
	if _, err := c.Find(ASTHandle{}); err != ErrASTDomain {
		t.Fatal("null lookup in unused cache", err)
	}
	astTestMust(t, c.Reset())
	refs := func(h ASTHandle) uint32 { n, e := m.GetRefCount(h); astTestMust(t, e); return n }
	bp, bq, br := refs(p), refs(q), refs(r)
	astTestMust(t, c.Insert(p, q))
	if int(c.table.size) != 1 || c.unused != 1 || refs(p) != bp+1 || refs(q) != bq+1 {
		t.Fatal("source new entry ownership")
	}
	astTestMust(t, c.Insert(p, q))
	if c.unused != 1 || len(c.queue) != 1 || refs(q) != bq+1 {
		t.Fatal("same value insertion changed ownership/activity")
	}
	astTestMust(t, c.Insert(p, r))
	if c.unused != 1 || refs(q) != bq || refs(r) != br+1 {
		t.Fatal("source replacement INC before DEC")
	}
	found, e := c.Find(p)
	astTestMust(t, e)
	if found != r || c.unused != 0 {
		t.Fatal("first find marks active", found, c.unused)
	}
	found, e = c.Find(p)
	astTestMust(t, e)
	if found != r || c.unused != 0 {
		t.Fatal("repeat find")
	}
	astTestMust(t, c.Reset())
	if int(c.table.size) != 0 || c.unused != 0 || c.qhead != 0 || len(c.queue) != 0 || refs(p) != bp || refs(q) != bq || refs(r) != br {
		t.Fatal("source reset ownership/state")
	}
	// Null values are used by the original proof cache, but still have an entry.
	astTestMust(t, c.Insert(p, ASTHandle{}))
	found, e = c.Find(p)
	astTestMust(t, e)
	if !found.IsNull() || int(c.table.size) != 1 || c.unused != 0 {
		t.Fatal("null tagged value", found, int(c.table.size), c.unused)
	}
	astTestMust(t, c.Cleanup())
	if c.activityCacheEntries != nil {
		t.Fatal("cleanup reinitializes map and finalizes queue")
	}
	copyOwner := *c
	if e = copyOwner.Insert(p, q); e != ErrASTHandle {
		t.Fatal("copied cache owner", e)
	}
	astTestMust(t, c.Close())
	if _, e = c.Find(p); e != ErrASTHandle {
		t.Fatal("closed cache owner", e)
	}
}

func TestASTActivityCacheOldestUnusedEvictionAndQueueCompression(t *testing.T) {
	m := astTestManager(t)
	limit := uint32(0)
	c, e := NewActivityCache(m, &limit)
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Close()) }()
	value := activityCacheTestAtom(t, m, "cache-shared-value")
	var keys []ASTHandle
	for i := 0; i < 1024; i++ {
		k := activityCacheTestAtom(t, m, fmt.Sprintf("cache-key-%d", i))
		keys = append(keys, k)
		astTestMust(t, c.Insert(k, value))
	}
	if c.unused != 1024 || int(c.table.size) != 1024 {
		t.Fatal("source unused bound")
	}
	// The default overload captures the current canonical AST-table size.
	wantLimit := m.table.size
	large, e := NewActivityCache(m, nil)
	astTestMust(t, e)
	if wantLimit <= 1024 || large.maxUnused != wantLimit {
		t.Fatal("source default current AST count", large.maxUnused, wantLimit)
	}
	astTestMust(t, large.Close())
	for i := 0; i < 900; i++ {
		got, e := c.Find(keys[i])
		astTestMust(t, e)
		if got != value {
			t.Fatal("activity mark", i)
		}
	}
	// Reach the limit again while leaving used entries in the old queue prefix.
	for i := 0; i < 900; i++ {
		k := activityCacheTestAtom(t, m, fmt.Sprintf("cache-refill-%d", i))
		astTestMust(t, c.Insert(k, value))
	}
	fresh := activityCacheTestAtom(t, m, "cache-trigger")
	astTestMust(t, c.Insert(fresh, value))
	if c.unused != 1024 || int(c.table.size) != 1924 || c.qhead != 901 {
		t.Fatal("skip used keys then evict oldest unused", c.unused, int(c.table.size), c.qhead)
	}
	missing, e := c.Find(keys[900])
	astTestMust(t, e)
	if !missing.IsNull() {
		t.Fatal("oldest unused key survived")
	}
	for i := 0; i < 124; i++ {
		k := activityCacheTestAtom(t, m, fmt.Sprintf("cache-evict-%d", i))
		astTestMust(t, c.Insert(k, value))
	}
	if c.qhead != 0 || len(c.queue) != 1024 {
		t.Fatal("source compression threshold", c.qhead, len(c.queue))
	}
	got, e := c.Find(keys[0])
	astTestMust(t, e)
	if got != value {
		t.Fatal("used entry was evicted")
	}
	if !c.CheckInvariant() {
		t.Fatal("source invariant method")
	}
}

func TestASTActivityCacheUsedReplacementPreservesSourceCounterQuirk(t *testing.T) {
	m := astTestManager(t)
	c, e := NewActivityCache(m, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Close()) }()
	p, q, r := activityCacheTestAtom(t, m, "cache-used-p"), activityCacheTestAtom(t, m, "cache-used-q"), activityCacheTestAtom(t, m, "cache-used-r")
	if c.maxUnused != 1024 {
		t.Fatal("default max uses current AST table size and clamps")
	}
	astTestMust(t, c.Insert(p, q))
	_, e = c.Find(p)
	astTestMust(t, e)
	astTestMust(t, c.Insert(p, r))
	if c.unused != 0 || len(c.queue) != 1 {
		t.Fatal("source replacement does not increment unused or append queue")
	}
	got, e := c.Find(p)
	if ASTSourceDebug {
		if e != ErrASTDomain || c.unused != 0 {
			t.Fatal("source reached unused>0 assertion", e, c.unused)
		}
	} else {
		astTestMust(t, e)
		if got != r || c.unused != ^uint32(0) {
			t.Fatal("source counter underflow", got, c.unused)
		}
	}
}

func TestASTActivityCacheNativeGrowthOrderAndOwnership(t *testing.T) {
	m := astTestManager(t)
	c, e := NewActivityCache(m, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Close()) }()
	const count = 257
	value := activityCacheTestAtom(t, m, "native-cache-value")
	keys := make([]ASTHandle, count)
	for i := range keys {
		keys[i] = activityCacheTestAtom(t, m, fmt.Sprintf("native-cache-key-%d", i))
	}
	valueRefs, e := m.GetRefCount(value)
	astTestMust(t, e)
	used, storage := m.BudgetField.Used(), m.Stats()
	m.BudgetField.FailAfter(0)
	for _, key := range keys {
		astTestMust(t, c.Insert(key, value))
	}
	index := 0
	for _, key := range keys {
		entry := activityTestEntry(c, key)
		if entry == nil || m.external(entry.key) != key || m.external(entry.value) != value || !entry.keyHeld || !entry.valueHeld {
			t.Fatal("native growth changed ordered owning entry", index, key, entry)
		}
		index++
	}
	astTestMust(t, c.Insert(keys[1], value))
	if index != count || len(c.queue) != count || c.unused != count {
		t.Fatal("duplicate changed queue or acquisitions", index, len(c.queue), c.unused)
	}
	got, e := c.Find(keys[0])
	astTestMust(t, e)
	if got != value {
		t.Fatal("native lookup", got)
	}
	astTestMust(t, c.deleteUnused())
	if activityTestEntry(c, keys[0]) == nil || activityTestEntry(c, keys[1]) != nil || c.qhead != 2 || c.unused != count-2 {
		t.Fatal("oldest unused eviction changed", c.qhead, c.unused)
	}
	refs, e := m.GetRefCount(value)
	astTestMust(t, e)
	if refs != valueRefs+count-1 || m.BudgetField.Used() != used || m.Stats() != storage {
		t.Fatal("native containers changed actual object storage or references", refs, valueRefs)
	}
	astTestMust(t, c.Reset())
	refs, e = m.GetRefCount(value)
	astTestMust(t, e)
	if refs != valueRefs || int(c.table.size) != 0 || len(c.queue) != 0 || m.BudgetField.Used() != used {
		t.Fatal("matched reset retained references", refs, valueRefs)
	}
	for _, key := range keys {
		refs, e := m.GetRefCount(key)
		astTestMust(t, e)
		if refs != 1 {
			t.Fatal("reset changed caller-owned key", key, refs)
		}
	}
	m.BudgetField.FailAfter(-1)
}

func activityCacheTestAtom(t *testing.T, m *ASTManager, name string) ASTHandle {
	t.Helper()
	h := proofTestBool(t, m, name)
	astTestPin(t, m, h)
	return h
}

// activityTestEntry looks k up through the chained table.
func activityTestEntry(c *ActivityCache, k ASTHandle) *astActivityEntry {
	hash, e := c.manager.GetHash(k)
	if e != nil {
		return nil
	}
	return c.table.find(astNodeRef{k.slot, k.generation}, hash)
}

// chashtable::insert_if_not_there expands a table without free cells before
// looking the key up, so re-inserting a cached key into a full table grows it
// (AUDIT ast_01-F2).
func TestASTActivityCacheDuplicateInsertExpandsFullTable(t *testing.T) {
	m := astTestManager(t)
	c, e := NewActivityCache(m, nil)
	astTestMust(t, e)
	defer func() { astTestMust(t, c.Close()) }()
	value := activityCacheTestAtom(t, m, "full-cache-value")
	var keys []ASTHandle
	for i := 0; c.activityCacheEntries == nil || c.table.hasFreeCells(); i++ {
		if i > 1000 {
			t.Fatal("table never filled")
		}
		k := activityCacheTestAtom(t, m, fmt.Sprintf("full-cache-key-%d", i))
		keys = append(keys, k)
		astTestMust(t, c.Insert(k, value))
	}
	slots := c.table.slots
	astTestMust(t, c.Insert(keys[0], value))
	if c.table.slots != 2*slots {
		t.Fatal("duplicate insert into a full table must expand it", slots, c.table.slots)
	}
	for _, k := range keys {
		if entry := activityTestEntry(c, k); entry == nil || m.external(entry.value) != value {
			t.Fatal("expansion lost an entry", k)
		}
	}
}

func TestActivityCacheForeignAliases(t *testing.T) {
	m, foreign := astTestManager(t), astTestManager(t)
	variable := func(owner *ASTManager, index uint32) ASTHandle {
		b, err := owner.MkBoolSort()
		astTestMust(t, err)
		v, err := owner.MkVar(index, b)
		astTestMust(t, err)
		astTestPin(t, owner, v)
		return v
	}
	k, v := variable(m, 0), variable(m, 1)
	otherK, otherV := variable(foreign, 0), variable(foreign, 1)
	if k.slot != otherK.slot || k.generation != otherK.generation || v.slot != otherV.slot || v.generation != otherV.generation {
		t.Fatal("fixture must collide across managers")
	}
	refs := func(owner *ASTManager, h ASTHandle) uint32 {
		n, err := owner.GetRefCount(h)
		astTestMust(t, err)
		return n
	}
	c, err := NewActivityCache(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	keyCount, valueCount := refs(m, k), refs(m, v)
	astTestMust(t, c.Insert(k, v))
	got, err := c.Find(otherK)
	if !got.IsNull() || err != nil && err != ErrASTHandle {
		t.Fatal("foreign lookup resolved a local entry", got, err)
	}
	if err = c.Insert(otherK, v); err != ErrASTHandle {
		t.Fatal("foreign key inserted", err)
	}
	astTestMust(t, c.Reset())
	if refs(m, k) != keyCount || refs(m, v) != valueCount {
		t.Fatal("foreign key disturbed local ownership")
	}
	if err = c.Insert(k, otherV); err != ErrASTHandle {
		t.Fatal("foreign value inserted", err)
	}
	astTestMust(t, c.Reset())
	if refs(m, k) != keyCount || refs(m, v) != valueCount || refs(foreign, otherV) != 1 {
		t.Fatal("failed insertion leaked an acquisition")
	}
	astTestMust(t, c.Insert(k, v))
	got, err = c.Find(k)
	astTestMust(t, err)
	if got != v {
		t.Fatal("cache did not recover after reset", got)
	}
}

func TestActivityCacheEvictionRetiresIncomingOperand(t *testing.T) {
	for _, operand := range []string{"key", "value"} {
		t.Run(operand, func(t *testing.T) {
			m := astTestManager(t)
			sort, err := m.MkBoolSort()
			astTestMust(t, err)
			value := activityCacheTestAtom(t, m, "pinned value")
			key := activityCacheTestAtom(t, m, "pinned key")
			c, err := NewActivityCache(m, nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, c.Close()) })
			var oldest ASTHandle
			for i := uint32(0); i < c.maxUnused; i++ {
				v, err := m.MkVar(i, sort)
				astTestMust(t, err)
				astTestMust(t, c.Insert(v, value))
				if i == 0 {
					oldest = v
				}
			}
			// The cache holds the sole reference to its oldest entry's key.
			// It is live at entry and dies during eviction, before acquisition.
			if operand == "key" {
				err = c.Insert(oldest, value)
			} else {
				err = c.Insert(key, oldest)
			}
			if err != ErrASTHandle || m.IsLiveHandle(oldest) {
				t.Fatal("retired operand was acquired", err)
			}
			astTestMust(t, c.Reset())
			for _, h := range []ASTHandle{key, value} {
				refs, err := m.GetRefCount(h)
				astTestMust(t, err)
				if refs != 1 {
					t.Fatal("reset left an acquisition", h, refs)
				}
			}
		})
	}
}
