package lp

// Source: libstdc++ (GCC 13) std::unordered_map<std::string, unsigned>:
// std::hash<std::string> is std::_Hash_bytes(data, size, 0xc70f6907)
// (libsupc++/hash_bytes.cc, the 64-bit MurmurHash2 variant), hash codes
// cached in the nodes, _Mod_range_hashing and _Prime_rehash_policy; node
// placement is the stdHashtable of std_unordered_map.go.

// =========================================================================
// !!! WARNING: THIS IS A DELIBERATE, USER-APPROVED PORT OF STL INTERNALS !!!
// =========================================================================
//
// Approved by the owner on 2026-11-14 for AUDIT.md util_lp_06-F1, as a
// generalization of the lar_term unordered_map exception
// (StdUnorderedRationalMap). DO NOT replace it with a Go map, a sorted
// slice, or insdict.
//
// WHY IT EXISTS:
//   The MPS front end prints its model by walking lp_solver's
//   std::unordered_map<std::string, unsigned> m_names_to_columns
//   (lp_solver.h:259-264 print_model), and fills that map while walking
//   mps_reader's std::unordered_map<std::string, column*> m_columns
//   (mps_reader.h:715 fill_solver_on_columns). Both iteration orders are
//   libstdc++ node-list orders, deterministic for a given build, and they
//   are z3's user-visible "column values" order.
//
// WHICH TESTS DEPEND ON IT (keep them green):
//   * TestStdUnorderedStringMapMatchesLibstdcxx
//     (std_unordered_string_map_test.go) compares hash codes and iteration
//     orders against the output of
//     src/test/stl_controls/stl_unordered_string_map_order_control.cpp
//     built with g++ 13.3 (test_vectors/libstdcxx_unordered_string_map_order.txt).
//     Regenerate the recorded output only by rerunning that program.
//   * TestLPFloatModelReportLibstdcxxNameOrder and the cmd/g3 MPS vector
//     lp_mps_print_model_order.mps.

import "encoding/binary"

// stdHashBytesMul is _Hash_bytes's mul.
const stdHashBytesMul = uint64(0xc6a4a793)<<32 + 0x5bd1e995

// stdStringHashSeed is std::hash<std::string>'s seed.
const stdStringHashSeed = 0xc70f6907

func stdHashShiftMix(v uint64) uint64 { return v ^ (v >> 47) }

// stdHashBytes is the 64-bit std::_Hash_bytes(ptr, len, seed).
func stdHashBytes(s string, seed uint64) uint64 {
	n := uint64(len(s))
	aligned := n &^ 7
	hash := seed ^ (n * stdHashBytesMul)
	for p := uint64(0); p != aligned; p += 8 {
		// unaligned_load: a little-endian 8-byte load.
		data := stdHashShiftMix(binary.LittleEndian.Uint64([]byte(s[p:p+8]))*stdHashBytesMul) * stdHashBytesMul
		hash ^= data
		hash *= stdHashBytesMul
	}
	if n&7 != 0 {
		// load_bytes(end, len & 7): bytes as unsigned char, last one first.
		var data uint64
		for i := int(n&7) - 1; i >= 0; i-- {
			data = data<<8 + uint64(s[aligned+uint64(i)])
		}
		hash ^= data
		hash *= stdHashBytesMul
	}
	hash = stdHashShiftMix(hash) * stdHashBytesMul
	return stdHashShiftMix(hash)
}

// StdUnorderedStringMap is std::unordered_map<std::string, unsigned>:
// iteration follows libstdc++'s node list. The Go map only speeds up key
// lookup.
type StdUnorderedStringMap struct {
	table  stdHashtable
	keys   []string
	values []uint32
	lookup map[string]int32
}

// NewStdUnorderedStringMap is a default-constructed map (one bucket).
func NewStdUnorderedStringMap() *StdUnorderedStringMap {
	return &StdUnorderedStringMap{table: newStdHashtable(), lookup: map[string]int32{}}
}

// Len is size().
func (m *StdUnorderedStringMap) Len() int { return int(m.table.count) }

// BucketCount is bucket_count().
func (m *StdUnorderedStringMap) BucketCount() uint64 { return uint64(len(m.table.buckets)) }

// Get2 is find(key).
func (m *StdUnorderedStringMap) Get2(key string) (uint32, bool) {
	n, ok := m.lookup[key]
	if !ok {
		return 0, false
	}
	return m.values[n], true
}

// Put is m[key] = value: a new key is inserted as _M_insert_unique_node
// does, and an existing key keeps its node.
func (m *StdUnorderedStringMap) Put(key string, value uint32) {
	if n, ok := m.lookup[key]; ok {
		m.values[n] = value
		return
	}
	n, appended := m.table.allocNode(stdHashBytes(key, stdStringHashSeed))
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
func (m *StdUnorderedStringMap) Del(key string) {
	n, ok := m.lookup[key]
	if !ok {
		return
	}
	m.table.unlinkNode(n)
	delete(m.lookup, key)
	m.keys[n] = ""
}

// All iterates in begin()..end() order. The yielded entry may be deleted
// during iteration; other mutations invalidate the walk.
func (m *StdUnorderedStringMap) All() func(yield func(string, uint32) bool) {
	return func(yield func(string, uint32) bool) {
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
func (m *StdUnorderedStringMap) Keys() []string {
	out := make([]string, 0, m.table.count)
	for p := m.table.beforeBegin; p != stdNil; p = m.table.next[p] {
		out = append(out, m.keys[p])
	}
	return out
}
