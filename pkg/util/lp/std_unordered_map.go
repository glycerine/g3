package lp

// Source: libstdc++ (GCC 13) bits/hashtable.h and
// src/c++11/hashtable_c++0x.cc, std::unordered_map<unsigned, V> with
// std::hash<unsigned> (identity), _Mod_range_hashing and
// _Prime_rehash_policy, unique keys, hash codes not cached. The node list
// and bucket machinery (stdHashtable) is shared with
// std_unordered_string_map.go.
//
// User-approved exception to the no-STL-port rule (2026-11-10): lp's
// lar_term::m_coeffs is a std::unordered_map<unsigned, mpq>, and z3's
// pivot choices depend on its iteration order, so LARTerm reproduces that
// order exactly. Kept in its own file so it can be generalized to other
// unordered_map/unordered_set uses. Positive control:
// src/test/stl_controls/stl_unordered_map_order_control.cpp, whose output is
// test_vectors/libstdcxx_unordered_map_order.txt.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// Node links: stdNil is nullptr; stdBeforeBegin is &_M_before_begin.
const (
	stdNil         int32 = -1
	stdBeforeBegin int32 = -2
)

// stdFastBucket is _M_next_bkt's __fast_bkt table for hints below 14.
var stdFastBucket = [...]uint64{2, 2, 2, 3, 5, 5, 7, 7, 11, 11, 11, 11, 13, 13}

// stdPrimeList is __prime_list from __prime_list+6 (17) on, as reported by
// the control's _M_next_bkt dump.
var stdPrimeList = [...]uint64{
	17, 19, 23, 29, 31, 37, 41, 43,
	47, 53, 59, 61, 67, 71, 73, 79,
	83, 89, 97, 103, 109, 113, 127, 137,
	139, 149, 157, 167, 179, 193, 199, 211,
	227, 241, 257, 277, 293, 313, 337, 359,
	383, 409, 439, 467, 503, 541, 577, 619,
	661, 709, 761, 823, 887, 953, 1031, 1109,
	1193, 1289, 1381, 1493, 1613, 1741, 1879, 2029,
	2179, 2357, 2549, 2753, 2971, 3209, 3469, 3739,
	4027, 4349, 4703, 5087, 5503, 5953, 6427, 6949,
	7517, 8123, 8783, 9497, 10273, 11113, 12011, 12983,
	14033, 15173, 16411, 17749, 19183, 20753, 22447, 24281,
	26267, 28411, 30727, 33223, 35933, 38873, 42043, 45481,
	49201, 53201, 57557, 62233, 67307, 72817, 78779, 85229,
	92203, 99733, 107897, 116731, 126271, 136607, 147793, 159871,
	172933, 187091, 202409, 218971, 236897, 256279, 277261, 299951,
	324503, 351061, 379787, 410857, 444487, 480881, 520241, 562841,
	608903, 658753, 712697, 771049, 834181, 902483, 976369, 1056323,
	1142821, 1236397, 1337629, 1447153, 1565659, 1693859, 1832561, 1982627,
	2144977, 2320627, 2510653, 2716249, 2938679, 3179303, 3439651, 3721303,
	4026031, 4355707, 4712381, 5098259, 5515729, 5967347, 6456007, 6984629,
	7556579, 8175383, 8844859, 9569143, 10352717, 11200489, 12117689, 13109983,
	14183539, 15345007, 16601593, 17961079, 19431899, 21023161, 22744717, 24607243,
	26622317, 28802401, 31160981, 33712729, 36473443, 39460231, 42691603, 46187573,
	49969847, 54061849, 58488943, 63278561, 68460391, 74066549, 80131819, 86693767,
	93793069, 101473717, 109783337, 118773397, 128499677, 139022417, 150406843, 162723577,
	176048909, 190465427, 206062531, 222936881, 241193053, 260944219, 282312799, 305431229,
	330442829, 357502601, 386778277, 418451333, 452718089, 489790921, 529899637, 573292817,
	620239453, 671030513, 725980837, 785430967, 849749479, 919334987, 994618837, 1076067617,
	1164186217, 1259520799, 1362662261, 1474249943, 1594975441, 1725587117, 1866894511, 2019773507,
	2185171673, 2364114217, 2557710269, 2767159799, 2993761039, 3238918481, 3504151727, 3791104843,
	4101556399, 4294967291, 6442450933, 8589934583, 12884901857,
}

// stdPrimeRehashPolicy is _Prime_rehash_policy with max_load_factor 1.
type stdPrimeRehashPolicy struct {
	nextResize uint64
}

func (p *stdPrimeRehashPolicy) nextBucket(n uint64) uint64 {
	if n < uint64(len(stdFastBucket)) {
		if n == 0 {
			return 1
		}
		p.nextResize = uint64(math.Floor(float64(stdFastBucket[n])))
		return stdFastBucket[n]
	}
	last := len(stdPrimeList) - 1
	i := 0
	for i < last && stdPrimeList[i] < n {
		i++
	}
	if i == last {
		p.nextResize = math.MaxUint64
	} else {
		p.nextResize = uint64(math.Floor(float64(stdPrimeList[i])))
	}
	return stdPrimeList[i]
}

func (p *stdPrimeRehashPolicy) needRehash(buckets, elements, inserts uint64) (bool, uint64) {
	if elements+inserts <= p.nextResize {
		return false, 0
	}
	floor := uint64(0)
	if p.nextResize == 0 {
		floor = 11
	}
	minBuckets := float64(max(elements+inserts, floor))
	if minBuckets >= float64(buckets) {
		return true, p.nextBucket(max(uint64(math.Floor(minBuckets))+1, buckets*2))
	}
	p.nextResize = uint64(math.Floor(float64(buckets)))
	return false, 0
}

// stdHashtable is the node list and bucket array of libstdc++'s
// _Hashtable for unique keys. Node n's hash code is codes[n]: the key
// itself for std::hash<unsigned> (not cached; recomputing it gives the same
// value), the cached _Hash_bytes code for std::string. The wrapping
// container keeps keys and values at the same node index.
type stdHashtable struct {
	codes       []uint64
	next        []int32
	free        []int32
	buckets     []int32 // node before the bucket's first node, or stdNil
	beforeBegin int32   // _M_before_begin._M_nxt
	count       uint64
	policy      stdPrimeRehashPolicy
}

func newStdHashtable() stdHashtable {
	return stdHashtable{buckets: []int32{stdNil}, beforeBegin: stdNil}
}

func (h *stdHashtable) nxt(p int32) int32 {
	if p == stdBeforeBegin {
		return h.beforeBegin
	}
	return h.next[p]
}

func (h *stdHashtable) setNxt(p, v int32) {
	if p == stdBeforeBegin {
		h.beforeBegin = v
		return
	}
	h.next[p] = v
}

func (h *stdHashtable) bucketOf(n int32, count uint64) uint64 {
	return h.codes[n] % count
}

// allocNode takes a node for a new element with the given hash code,
// reusing an erased node first. appended reports a fresh index equal to
// the old node count, which the caller must append its key and value at.
func (h *stdHashtable) allocNode(code uint64) (n int32, appended bool) {
	if k := len(h.free); k > 0 {
		n = h.free[k-1]
		h.free = h.free[:k-1]
		h.codes[n], h.next[n] = code, stdNil
		return n, false
	}
	n = int32(len(h.codes))
	h.codes = append(h.codes, code)
	h.next = append(h.next, stdNil)
	return n, true
}

// linkNode is _M_insert_unique_node: rehash when the policy asks for it,
// then insert at the beginning of the node's bucket.
func (h *stdHashtable) linkNode(n int32) {
	if do, count := h.policy.needRehash(uint64(len(h.buckets)), h.count, 1); do {
		h.rehash(count)
	}
	h.insertBucketBegin(h.bucketOf(n, uint64(len(h.buckets))), n)
	h.count++
}

// insertBucketBegin is _M_insert_bucket_begin.
func (h *stdHashtable) insertBucketBegin(bkt uint64, n int32) {
	if b := h.buckets[bkt]; b != stdNil {
		h.next[n] = h.nxt(b)
		h.setNxt(b, n)
		return
	}
	h.next[n] = h.beforeBegin
	h.beforeBegin = n
	if h.next[n] != stdNil {
		h.buckets[h.bucketOf(h.next[n], uint64(len(h.buckets)))] = n
	}
	h.buckets[bkt] = stdBeforeBegin
}

// rehash is _M_rehash_aux for unique keys.
func (h *stdHashtable) rehash(count uint64) {
	buckets := make([]int32, count)
	for i := range buckets {
		buckets[i] = stdNil
	}
	p := h.beforeBegin
	h.beforeBegin = stdNil
	beginBucket := uint64(0)
	for p != stdNil {
		next := h.next[p]
		bkt := h.bucketOf(p, count)
		if buckets[bkt] == stdNil {
			h.next[p] = h.beforeBegin
			h.beforeBegin = p
			buckets[bkt] = stdBeforeBegin
			if h.next[p] != stdNil {
				buckets[beginBucket] = p
			}
			beginBucket = bkt
		} else {
			h.next[p] = h.nxt(buckets[bkt])
			h.setNxt(buckets[bkt], p)
		}
		p = next
	}
	h.buckets = buckets
}

// unlinkNode is _M_erase(bkt, prev, n) for a linked node n; the node goes
// on the free list.
func (h *stdHashtable) unlinkNode(n int32) {
	count := uint64(len(h.buckets))
	bkt := h.bucketOf(n, count)
	prev := h.buckets[bkt]
	for h.nxt(prev) != n {
		prev = h.nxt(prev)
	}
	next := h.next[n]
	if prev == h.buckets[bkt] {
		nextBkt := uint64(0)
		if next != stdNil {
			nextBkt = h.bucketOf(next, count)
		}
		if next == stdNil || nextBkt != bkt {
			if next != stdNil {
				h.buckets[nextBkt] = h.buckets[bkt]
			}
			if h.buckets[bkt] == stdBeforeBegin {
				h.beforeBegin = next
			}
			h.buckets[bkt] = stdNil
		}
	} else if next != stdNil {
		if nextBkt := h.bucketOf(next, count); nextBkt != bkt {
			h.buckets[nextBkt] = prev
		}
	}
	h.setNxt(prev, next)
	h.free = append(h.free, n)
	h.count--
}

// clear is clear(): the bucket array and rehash state stay.
func (h *stdHashtable) clear() {
	for i := range h.buckets {
		h.buckets[i] = stdNil
	}
	h.codes, h.next, h.free = h.codes[:0], h.next[:0], h.free[:0]
	h.beforeBegin = stdNil
	h.count = 0
}

// StdUnorderedRationalMap is std::unordered_map<unsigned, mpq> as
// lar_term::m_coeffs uses it: iteration follows the singly linked node list
// that libstdc++ maintains, so insertion, erase and rehash place nodes as
// the C++ container does. The Go map only speeds up key lookup.
type StdUnorderedRationalMap struct {
	table  stdHashtable
	keys   []uint32
	values []*numeral.Rational
	lookup map[uint32]int32
}

func NewStdUnorderedRationalMap() *StdUnorderedRationalMap {
	return &StdUnorderedRationalMap{table: newStdHashtable(), lookup: map[uint32]int32{}}
}

func (m *StdUnorderedRationalMap) Len() int { return int(m.table.count) }

// BucketCount is bucket_count().
func (m *StdUnorderedRationalMap) BucketCount() uint64 { return uint64(len(m.table.buckets)) }

func (m *StdUnorderedRationalMap) Get2(key uint32) (*numeral.Rational, bool) {
	n, ok := m.lookup[key]
	if !ok {
		return nil, false
	}
	return m.values[n], true
}

// Get is find(key)->second, or nil when key is absent.
func (m *StdUnorderedRationalMap) Get(key uint32) *numeral.Rational {
	v, _ := m.Get2(key)
	return v
}

// Put stores value for key; a new key is inserted as _M_insert_unique_node
// does, and an existing key keeps its node.
func (m *StdUnorderedRationalMap) Put(key uint32, value *numeral.Rational) {
	if n, ok := m.lookup[key]; ok {
		m.values[n] = value
		return
	}
	// std::hash<unsigned> is the identity.
	n, appended := m.table.allocNode(uint64(key))
	if appended {
		m.keys = append(m.keys, key)
		m.values = append(m.values, value)
	} else {
		m.keys[n], m.values[n] = key, value
	}
	m.lookup[key] = n
	m.table.linkNode(n)
}

// Del is erase(const key_type&).
func (m *StdUnorderedRationalMap) Del(key uint32) {
	n, ok := m.lookup[key]
	if !ok {
		return
	}
	m.table.unlinkNode(n)
	delete(m.lookup, key)
	m.values[n] = nil
}

// Clear is clear(): the bucket array and rehash state stay.
func (m *StdUnorderedRationalMap) Clear() {
	m.table.clear()
	m.keys, m.values = m.keys[:0], m.values[:0]
	clear(m.lookup)
}

// All iterates in begin()..end() order. The yielded entry may be deleted
// during iteration; other mutations invalidate the walk.
func (m *StdUnorderedRationalMap) All() func(yield func(uint32, *numeral.Rational) bool) {
	return func(yield func(uint32, *numeral.Rational) bool) {
		for p := m.table.beforeBegin; p != stdNil; {
			next := m.table.next[p]
			if !yield(m.keys[p], m.values[p]) {
				return
			}
			p = next
		}
	}
}

// Keys returns the keys in iteration order.
func (m *StdUnorderedRationalMap) Keys() []uint32 {
	out := make([]uint32, 0, m.table.count)
	for p := m.table.beforeBegin; p != stdNil; p = m.table.next[p] {
		out = append(out, m.keys[p])
	}
	return out
}
