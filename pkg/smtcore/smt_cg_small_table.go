package smtcore

// smtCgSmallTable is the arity one and two congruence table of one
// declaration: an open-addressing hash table from a pair of class roots to
// the stored enode, with the semantics of map[[2]*SMTEnode]*SMTEnode (keys
// are captured at insertion and compared by pointer). It replaces the Go
// map, whose generic hashing dominated congruence-table operations; the
// source chashtable is likewise a custom open table. Hashes use the root
// owners' AST slots, unique among live ASTs, so the table never depends on
// pointer addresses.
type smtCgSmallTable struct {
	entries []smtCgSmallEntry // len is zero or a power of two; node == nil is empty
	count   int
	shift   uint8 // 64 - log2(len(entries))
}

type smtCgSmallEntry struct {
	k0, k1, node *SMTEnode
}

func smtCgSmallSlot(k0, k1 *SMTEnode, shift uint8) int {
	x := uint64(k0.owner.Slot()) << 32
	if k1 != nil {
		x |= uint64(k1.owner.Slot())
	}
	x ^= x >> 29
	return int((x * 0x9E3779B97F4A7C15) >> shift)
}

func (t *smtCgSmallTable) get(k0, k1 *SMTEnode) *SMTEnode {
	if t.count == 0 {
		return nil
	}
	mask := len(t.entries) - 1
	for i := smtCgSmallSlot(k0, k1, t.shift); ; i = (i + 1) & mask {
		e := &t.entries[i]
		if e.node == nil {
			return nil
		}
		if e.k0 == k0 && e.k1 == k1 {
			return e.node
		}
	}
}

// set is m[key] = node.
func (t *smtCgSmallTable) set(k0, k1, node *SMTEnode) {
	if 2*(t.count+1) > len(t.entries) {
		t.grow()
	}
	mask := len(t.entries) - 1
	for i := smtCgSmallSlot(k0, k1, t.shift); ; i = (i + 1) & mask {
		e := &t.entries[i]
		if e.node == nil {
			*e = smtCgSmallEntry{k0, k1, node}
			t.count++
			return
		}
		if e.k0 == k0 && e.k1 == k1 {
			e.node = node
			return
		}
	}
}

// slot finds an entry for deletion. Read-only probes return the node directly.
func (t *smtCgSmallTable) slot(k0, k1 *SMTEnode) int {
	if t.count == 0 {
		return -1
	}
	mask := len(t.entries) - 1
	for i := smtCgSmallSlot(k0, k1, t.shift); ; i = (i + 1) & mask {
		e := &t.entries[i]
		if e.node == nil {
			return -1
		}
		if e.k0 == k0 && e.k1 == k1 {
			return i
		}
	}
}

func (t *smtCgSmallTable) del(k0, k1 *SMTEnode) {
	if i := t.slot(k0, k1); i >= 0 {
		t.deleteSlot(i)
	}
}

func (t *smtCgSmallTable) eraseCongruent(key [2]*SMTEnode, query *SMTEnode) (bool, error) {
	i := t.slot(key[0], key[1])
	if i < 0 {
		return false, nil
	}
	node := t.entries[i].node
	// A representative is congruent to itself in either probe order.
	// Malformed commutative applications still go through the arity check.
	if node != query || (node.commutative && len(node.arguments) != 2) {
		equal, _, err := SMTEnodesCongruent(node, query)
		if err != nil || !equal {
			return false, err
		}
	}
	// A merge may have changed the representative's roots. Erase uses its
	// current signature, even if the successful probe found an older one.
	if current := smtCongruenceSmallKey(node); current != key {
		i = t.slot(current[0], current[1])
		if i < 0 {
			return true, nil
		}
	}
	t.deleteSlot(i)
	return true, nil
}

// deleteSlot closes the hole by moving displaced entries back toward home.
func (t *smtCgSmallTable) deleteSlot(i int) {
	mask := len(t.entries) - 1
	t.count--
	for j := (i + 1) & mask; ; j = (j + 1) & mask {
		e := &t.entries[j]
		if e.node == nil {
			break
		}
		home := smtCgSmallSlot(e.k0, e.k1, t.shift)
		// Move e back to the hole at i unless its home lies cyclically in
		// (i, j].
		if (j-home)&mask >= (j-i)&mask {
			t.entries[i] = *e
			i = j
		}
	}
	t.entries[i] = smtCgSmallEntry{}
}

func (t *smtCgSmallTable) grow() {
	old := t.entries
	size := 8
	if len(old) != 0 {
		size = 2 * len(old)
	}
	shift := uint8(64)
	for n := size; n > 1; n >>= 1 {
		shift--
	}
	t.entries, t.shift, t.count = make([]smtCgSmallEntry, size), shift, 0
	for i := range old {
		if e := &old[i]; e.node != nil {
			t.set(e.k0, e.k1, e.node)
		}
	}
}

// forEach visits the stored entries (in table order).
func (t *smtCgSmallTable) forEach(visit func(k0, k1, node *SMTEnode)) {
	for i := range t.entries {
		if e := &t.entries[i]; e.node != nil {
			visit(e.k0, e.k1, e.node)
		}
	}
}
