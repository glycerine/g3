package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"encoding/binary"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/insdict"
)

const (
	monomialTemporaryKey         uint32 = 1
	monomialObjectKey            uint32 = 2
	monomialBorrowedTemporaryKey uint32 = 3
)

type monomialKey struct{ kind, slot uint32 }
type monomialTableEntry struct {
	key  monomialKey
	hash uint32
}

// Entries borrow their handles. Stable entry IDs preserve a temporary's factory
// publication until it is replaced by the completed object. Dictionary order
// makes rename deterministic without reproducing C++ hash bucket geometry.
type monomialTable struct {
	entries insdict.Dict[uint32, monomialTableEntry]
	buckets map[uint32][]uint32
	nextID  uint32
}

func (t *monomialTable) key(i uint32) (monomialKey, error) {
	e, ok := t.entries.Get2(i)
	if !ok {
		return monomialKey{}, arena.ErrBounds
	}
	return e.key, nil
}
func (t *monomialTable) setKey(i uint32, k monomialKey) error {
	e, ok := t.entries.Get2(i)
	if !ok {
		return arena.ErrBounds
	}
	e.key = k
	t.entries.Set(i, e)
	return nil
}
func (t *monomialTable) equal(m *MonomialManager, a, b monomialKey) (bool, error) {
	ra, rb := monomialKeyReader{k: a}, monomialKeyReader{k: b}
	na, e := ra.word(m, 3)
	if e != nil {
		return false, e
	}
	nb, e := rb.word(m, 3)
	if e != nil {
		return false, e
	}
	if na != nb {
		return false, nil
	}
	ha, e := ra.word(m, 4)
	if e != nil {
		return false, e
	}
	hb, e := rb.word(m, 4)
	if e != nil {
		return false, e
	}
	if ha != hb {
		return false, nil
	}
	// Source deliberately excludes total_degree: temporary values do not update it.
	if !polynomialSourceDebug {
		if same, ok := equalObjectTempPowers(m, &ra, &rb, na); ok {
			return same, nil
		}
	}
	for i := uint32(0); i < na; i++ {
		pa, e := ra.power(m, i)
		if e != nil {
			return false, e
		}
		pb, e := rb.power(m, i)
		if e != nil {
			return false, e
		}
		if pa != pb {
			return false, nil
		}
	}
	return true, nil
}

// monomialKeyReader is keyWord/keyPower for one key. An object key's
// checks and allocator object lookup are made once, at its first read, and
// later reads go through one reader of the object, which nothing changes
// while equal runs (temporary keys never touch the allocator).
type monomialKeyReader struct {
	k     monomialKey
	rd    umemory.SmallObjectReader
	ready bool
}

func (r *monomialKeyReader) word(m *MonomialManager, word uint32) (uint32, error) {
	k := r.k
	if k.kind != monomialObjectKey {
		return m.keyWord(k, word)
	}
	if !r.ready {
		// keyWord's object-key tests, then ReadUint32's object lookup.
		if k.slot == 0 || uint64(k.slot) > uint64(len(m.locations)) {
			return 0, ErrMonomialOwner
		}
		l := &m.locations[k.slot-1]
		if !l.live {
			return 0, ErrMonomialOwner
		}
		if e := m.allocator.ReaderInto(l.raw, &r.rd); e != nil {
			return 0, e
		}
		r.ready = true
	}
	return r.rd.ReadUint32(uint64(word) * 4)
}
func (r *monomialKeyReader) power(m *MonomialManager, index uint32) (MonomialPower, error) {
	k := r.k
	if k.kind != monomialObjectKey || !r.ready {
		return m.keyPower(k, index)
	}
	// keyPower -> powerCore: the key and location tests already passed at
	// the first word read and nothing changed them since.
	if e := m.powerSourceIndexAssert(Monomial{m.origin, k.slot, m.locations[k.slot-1].generation}, index); e != nil {
		return MonomialPower{}, e
	}
	var out MonomialPower
	var e error
	out.Var, e = r.rd.ReadUint32(20 + uint64(index)*8)
	if e != nil {
		return out, e
	}
	out.Degree, e = r.rd.ReadUint32(24 + uint64(index)*8)
	return out, e
}

func (t *monomialTable) find(m *MonomialManager, k monomialKey) (uint32, bool, error) {
	h, e := m.keyWord(k, 4)
	if e != nil {
		return 0, false, e
	}
	id, _, found, e := t.findHashed(m, k, h)
	return id, found, e
}

// findHashed also returns the found entry's key (what key(id) returns).
func (t *monomialTable) findHashed(m *MonomialManager, k monomialKey, h uint32) (uint32, monomialKey, bool, error) {
	for _, id := range t.buckets[h] {
		entry := t.entries.Get(id)
		same, e := t.equal(m, entry.key, k)
		if e != nil {
			return 0, monomialKey{}, false, e
		}
		if same {
			return id, entry.key, true, nil
		}
	}
	return 0, monomialKey{}, false, nil
}

// Source monomial_manager::rename uses chashtable::insert, which replaces an
// equal key. Factory interning instead keeps the prior canonical object.
func (t *monomialTable) insertRenamed(m *MonomialManager, k monomialKey) error {
	id, inserted, e := t.insert(m, k)
	if e != nil || inserted {
		return e
	}
	return t.setKey(id, k)
}

func (t *monomialTable) insert(m *MonomialManager, k monomialKey) (uint32, bool, error) {
	id, _, inserted, e := t.insertFound(m, k)
	return id, inserted, e
}

// insertFound is insert that also returns, when k was already present, the
// present entry's key, so the caller need not look it up again with key.
func (t *monomialTable) insertFound(m *MonomialManager, k monomialKey) (uint32, monomialKey, bool, error) {
	h, e := m.keyWord(k, 4)
	if e != nil {
		return 0, monomialKey{}, false, e
	}
	id, old, found, e := t.findHashed(m, k, h)
	if e != nil || found {
		return id, old, false, e
	}
	if t.nextID == ^uint32(0) {
		return 0, monomialKey{}, false, arena.ErrSlotLimit
	}
	id = t.nextID
	t.nextID++
	if t.buckets == nil {
		t.buckets = make(map[uint32][]uint32)
	}
	t.entries.Set(id, monomialTableEntry{k, h})
	t.buckets[h] = append(t.buckets[h], id)
	return id, monomialKey{}, true, nil
}
func (t *monomialTable) removeID(id uint32) {
	entry, ok := t.entries.Get2(id)
	if !ok {
		return
	}
	bucket := t.buckets[entry.hash]
	for i, existing := range bucket {
		if existing == id {
			copy(bucket[i:], bucket[i+1:])
			bucket = bucket[:len(bucket)-1]
			break
		}
	}
	if len(bucket) == 0 {
		delete(t.buckets, entry.hash)
	} else {
		t.buckets[entry.hash] = bucket
	}
	t.entries.Del(id)
}
func (t *monomialTable) erase(m *MonomialManager, k monomialKey) error {
	id, found, e := t.find(m, k)
	if e != nil {
		return e
	}
	if found {
		t.removeID(id)
	}
	return nil
}

// Owning abort removes the named token using its stored hash even if a partial
// source rename changed the object. It does not repair that source mutation.
func (t *monomialTable) removeToken(k monomialKey) {
	for id, entry := range t.entries.All() {
		if entry.key == k {
			t.removeID(id)
			return
		}
	}
}
func (t *monomialTable) cleanup() { *t = monomialTable{} }

// equalObjectTempPowers is equal's power loop for an object key against a
// temporary key (interning's lookup), when it cannot fail: the object's
// power words are all readable (SmallObjectReader.Bytes) and the temporary
// holds na powers. The loop's object reads are then plain reads (their
// index assertion is off without polynomialSourceDebug) and the
// temporary's are slice reads, so comparing in place gives the loop's
// result. ok is false otherwise, and equal runs its loop.
func equalObjectTempPowers(m *MonomialManager, ra, rb *monomialKeyReader, na uint32) (same, ok bool) {
	if ra.k.kind != monomialObjectKey {
		ra, rb = rb, ra
	}
	if ra.k.kind != monomialObjectKey || !ra.ready || na == 0 {
		return false, false
	}
	var t *monomialTemp
	switch k := rb.k; k.kind {
	case monomialTemporaryKey:
		if k.slot >= uint32(len(m.temps)) {
			return false, false
		}
		t = &m.temps[k.slot]
	case monomialBorrowedTemporaryKey:
		if m.borrowedTemp == nil || k.slot != 0 {
			return false, false
		}
		t = m.borrowedTemp
	default:
		return false, false
	}
	if t.powers == nil || na > t.capacity || uint64(na) > uint64(len(t.powers)) {
		return false, false
	}
	b, ok := ra.rd.Bytes(20, uint64(na)*8)
	if !ok {
		return false, false
	}
	for i, p := range t.powers[:na] {
		w := b[8*i : 8*i+8]
		if binary.LittleEndian.Uint32(w) != p.Var || binary.LittleEndian.Uint32(w[4:]) != p.Degree {
			return false, true
		}
	}
	return true, true
}
