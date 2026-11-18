package smtcore

// smtCheckerCache holds smt::checker's three per-query caches
// (m_is_true_cache[2] and m_to_enode_cache) in one open-addressed table
// keyed by smtHandleKey. clear is checker::is_sat's cache reset: it starts
// a new epoch, so entries of earlier epochs read as empty slots and no
// table is rewritten. The caches are lookups only; nothing iterates them.
type smtCheckerCache struct {
	slots []smtCheckerCacheSlot
	epoch uint32
	size  uint32 // entries of the current epoch
}

// smtCheckerCacheSlot is one key's entries; truth[v] is 0 (absent), 1
// (false) or 2 (true) for check(key, v); hasEnode marks enode as present.
type smtCheckerCacheSlot struct {
	key      uint64
	epoch    uint32
	truth    [2]uint8
	hasEnode bool
	enode    *SMTEnode
}

const (
	smtCheckerCacheMinSlots  = 64
	smtCheckerCacheKeepSlots = 1 << 14
)

func smtCheckerCacheHome(key uint64, mask uint32) uint32 {
	key *= 0x9e3779b97f4a7c15
	return uint32(key>>32) & mask
}

// clear empties the table. A table that grew very large is dropped so one
// large query does not pin its memory.
func (t *smtCheckerCache) clear() {
	if len(t.slots) > smtCheckerCacheKeepSlots {
		*t = smtCheckerCache{}
		return
	}
	if t.size == 0 {
		return
	}
	t.size = 0
	t.epoch++
	if t.epoch == 0 { // wrapped: stale epochs could read as current
		clear(t.slots)
		t.epoch = 1
	}
}

// find returns key's slot of the current epoch, or nil.
func (t *smtCheckerCache) find(key uint64) *smtCheckerCacheSlot {
	if t.size == 0 {
		return nil
	}
	mask := uint32(len(t.slots) - 1)
	for i := smtCheckerCacheHome(key, mask); ; i = (i + 1) & mask {
		s := &t.slots[i]
		if s.epoch != t.epoch {
			return nil
		}
		if s.key == key {
			return s
		}
	}
}

// slot returns key's slot of the current epoch, adding an empty one.
func (t *smtCheckerCache) slot(key uint64) *smtCheckerCacheSlot {
	if 4*(t.size+1) > 3*uint32(len(t.slots)) {
		t.grow()
	}
	mask := uint32(len(t.slots) - 1)
	for i := smtCheckerCacheHome(key, mask); ; i = (i + 1) & mask {
		s := &t.slots[i]
		if s.epoch != t.epoch {
			*s = smtCheckerCacheSlot{key: key, epoch: t.epoch}
			t.size++
			return s
		}
		if s.key == key {
			return s
		}
	}
}

func (t *smtCheckerCache) grow() {
	n := 2 * len(t.slots)
	if n < smtCheckerCacheMinSlots {
		n = smtCheckerCacheMinSlots
	}
	old, epoch := t.slots, t.epoch
	if epoch == 0 {
		epoch = 1
	}
	t.slots = make([]smtCheckerCacheSlot, n)
	t.epoch = epoch
	mask := uint32(n - 1)
	for k := range old {
		s := &old[k]
		if s.epoch != epoch {
			continue
		}
		i := smtCheckerCacheHome(s.key, mask)
		for t.slots[i].epoch == epoch {
			i = (i + 1) & mask
		}
		t.slots[i] = *s
	}
}
