package rel

// Source: src/muz/rel/dl_sparse_table.{h,cpp} entry_storage, over
// src/util/vector.h svector<char, size_t> and src/util/hashtable.h
// hashtable<store_offset, offset_hash_proc, offset_eq_proc>.

import (
	"bytes"

	"github.com/glycerine/g3/pkg/util/bitutil"
	"github.com/glycerine/g3/pkg/util/hash"
)

// storeOffset is entry_storage::store_offset (size_t).
type storeOffset = uint64

// entryStorageNoReserve is entry_storage::NO_RESERVE (UINT_MAX).
const entryStorageNoReserve storeOffset = storeOffset(^uint32(0))

// charSvector is svector<char, size_t>: len(buf) is the size and cap(buf)
// the capacity; a nil buf is the vector without storage.
type charSvector struct{ buf []byte }

// expand is vector::expand_vector: the first block holds 2 elements, each
// later one (3*capacity+1)/2.
func (v *charSvector) expand() {
	if v.buf == nil {
		v.buf = make([]byte, 0, 2)
		return
	}
	old := cap(v.buf)
	grown := make([]byte, len(v.buf), (3*old+1)>>1)
	copy(grown, v.buf)
	v.buf = grown
}

// resize is vector::resize(s): a smaller size shrinks, a larger one expands
// until the capacity holds s and value-initializes the new elements.
func (v *charSvector) resize(s int) {
	sz := len(v.buf)
	if s <= sz {
		if v.buf != nil {
			v.buf = v.buf[:s]
		}
		return
	}
	for s > cap(v.buf) {
		v.expand()
	}
	v.buf = v.buf[:s]
	clear(v.buf[sz:s])
}

// assign is vector::operator=: copy_core allocates the source's capacity.
func (v *charSvector) assign(source *charSvector) {
	if source.buf == nil {
		v.buf = nil
		return
	}
	copied := make([]byte, len(source.buf), cap(source.buf))
	copy(copied, source.buf)
	v.buf = copied
}

// offsetHashEntryBytes is sizeof(default_hash_entry<size_t>): the unsigned
// hash, the state and the size_t offset.
const offsetHashEntryBytes = 16

const (
	offsetHashInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	offsetHashSmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

type offsetHashState uint8

const (
	offsetHashFree offsetHashState = iota
	offsetHashDeleted
	offsetHashUsed
)

type offsetHashEntry struct {
	hash  uint32
	state offsetHashState
	data  storeOffset
}

// offsetIndexer is entry_storage::storage_indexer, core_hashtable with
// default_hash_entry<store_offset>. Its hash and equality procs read the
// unique part of the rows at the offsets in the storage's m_data.
type offsetIndexer struct {
	table      []offsetHashEntry
	size       int
	numDeleted int
	storage    *entryStorage
}

func (h *offsetIndexer) unique(ofs storeOffset) []byte {
	return h.storage.data.buf[ofs : ofs+storeOffset(h.storage.uniquePartSize)]
}

// getHash is offset_hash_proc: string_hash of the row's unique part.
func (h *offsetIndexer) getHash(ofs storeOffset) uint32 { return hash.Bytes(h.unique(ofs), 0) }

// equals is offset_eq_proc: memcmp of the unique parts.
func (h *offsetIndexer) equals(o1, o2 storeOffset) bool {
	return bytes.Equal(h.unique(o1), h.unique(o2))
}

func (h *offsetIndexer) capacity() int { return len(h.table) }

// moveTable is core_hashtable::move_table.
func offsetHashMoveTable(source, target []offsetHashEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != offsetHashUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == offsetHashFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == offsetHashFree {
				target[i], placed = entry, true
			}
		}
		if !placed {
			panic("muz/rel: offset hashtable has no free slot") // UNREACHABLE
		}
	}
}

// expandTable is core_hashtable::expand_table.
func (h *offsetIndexer) expandTable() {
	target := make([]offsetHashEntry, len(h.table)<<1)
	offsetHashMoveTable(h.table, target)
	h.table, h.numDeleted = target, 0
}

// removeDeletedEntries is core_hashtable::remove_deleted_entries.
func (h *offsetIndexer) removeDeletedEntries() {
	target := make([]offsetHashEntry, len(h.table))
	offsetHashMoveTable(h.table, target)
	h.table, h.numDeleted = target, 0
}

// insertCore is the shared body of core_hashtable::insert (overwrite is
// true: an equal entry takes the new data) and insert_if_not_there_core.
// It returns the slot holding e or its equal and whether e was added.
func (h *offsetIndexer) insertCore(e storeOffset, overwrite bool) (int, bool) {
	if (h.size+h.numDeleted)<<2 > len(h.table)*3 {
		h.expandTable()
	}
	hv := h.getHash(e)
	mask := uint32(len(h.table) - 1)
	begin := int(hv & mask)
	del := -1
	visit := func(i int) (int, bool, bool) {
		curr := &h.table[i]
		switch curr.state {
		case offsetHashUsed:
			if curr.hash == hv && h.equals(curr.data, e) {
				if overwrite {
					curr.data = e
				}
				return i, false, true
			}
		case offsetHashFree:
			n := i
			if del >= 0 {
				n = del
				h.numDeleted--
			}
			h.table[n] = offsetHashEntry{hash: hv, state: offsetHashUsed, data: e}
			h.size++
			return n, true, true
		default:
			del = i
		}
		return 0, false, false
	}
	for i := begin; i < len(h.table); i++ {
		if slot, added, done := visit(i); done {
			return slot, added
		}
	}
	for i := 0; i < begin; i++ {
		if slot, added, done := visit(i); done {
			return slot, added
		}
	}
	panic("muz/rel: offset hashtable has no free slot") // UNREACHABLE
}

// insert is core_hashtable::insert.
func (h *offsetIndexer) insert(e storeOffset) { h.insertCore(e, true) }

// insertIfNotThere is core_hashtable::insert_if_not_there: the stored
// offset equal to e.
func (h *offsetIndexer) insertIfNotThere(e storeOffset) storeOffset {
	slot, _ := h.insertCore(e, false)
	return h.table[slot].data
}

// findCore is core_hashtable::find_core: the slot of e's equal or -1.
func (h *offsetIndexer) findCore(e storeOffset) int {
	hv := h.getHash(e)
	mask := uint32(len(h.table) - 1)
	begin := int(hv & mask)
	for _, r := range [2][2]int{{begin, len(h.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &h.table[i]
			if curr.state == offsetHashUsed {
				if curr.hash == hv && h.equals(curr.data, e) {
					return i
				}
			} else if curr.state == offsetHashFree {
				return -1
			}
		}
	}
	return -1
}

func (h *offsetIndexer) contains(e storeOffset) bool { return h.findCore(e) >= 0 }

// remove is core_hashtable::remove.
func (h *offsetIndexer) remove(e storeOffset) {
	slot := h.findCore(e)
	if slot < 0 {
		return
	}
	next := slot + 1
	if next == len(h.table) {
		next = 0
	}
	if h.table[next].state == offsetHashFree {
		h.table[slot].state = offsetHashFree
		h.size--
		return
	}
	h.table[slot].state = offsetHashDeleted
	h.numDeleted++
	h.size--
	if h.numDeleted > h.size && h.numDeleted > offsetHashSmallCapacity {
		h.removeDeletedEntries()
	}
}

// reset is core_hashtable::reset: a mostly free table above 16 slots halves.
func (h *offsetIndexer) reset() {
	if h.size == 0 && h.numDeleted == 0 {
		return
	}
	overhead := 0
	for i := range h.table {
		if h.table[i].state != offsetHashFree {
			h.table[i].state = offsetHashFree
		} else {
			overhead++
		}
	}
	if len(h.table) > 16 && overhead<<2 > len(h.table)*3 {
		h.table = make([]offsetHashEntry, len(h.table)>>1)
	}
	h.size = 0
	h.numDeleted = 0
}

// entryStorage is entry_storage. Every row of entrySize bytes in data below
// afterLastOffset is unique and indexed, except the reserve: the last block
// when its offset is not in the indexer. data is 8 bytes longer than
// dataSize so that a 64-bit word can be read at the end of any row.
type entryStorage struct {
	entrySize      uint32
	uniquePartSize uint32
	dataSize       storeOffset
	data           charSvector
	indexer        offsetIndexer
	reserve        storeOffset
}

// newEntryStorage is entry_storage(entry_size, functional_size, init_size).
// A zero entry size is g3's nullary table, which keeps one zero byte per
// row (the source asserts entry_size > 0).
func newEntryStorage(entrySize, functionalSize, initSize uint32) *entryStorage {
	if entrySize == 0 {
		entrySize = 1
	}
	s := &entryStorage{entrySize: entrySize, uniquePartSize: entrySize - functionalSize}
	s.indexer = offsetIndexer{table: make([]offsetHashEntry, bitutil.NextPowerOfTwo32(max(8, initSize))), storage: s}
	s.reserve = entryStorageNoReserve
	s.resizeData(storeOffset(initSize))
	s.resizeData(0)
	return s
}

// assign is entry_storage::operator=.
func (s *entryStorage) assign(o *entryStorage) {
	s.indexer.reset()
	s.entrySize = o.entrySize
	s.uniquePartSize = o.uniquePartSize
	s.dataSize = o.dataSize
	s.data.assign(&o.data)
	s.reserve = o.reserve
	afterLast := s.afterLastOffset()
	for i := storeOffset(0); i < afterLast; i += storeOffset(s.entrySize) {
		s.indexer.insert(i)
	}
}

func (s *entryStorage) reset() {
	s.resizeData(0)
	s.indexer.reset()
	s.reserve = entryStorageNoReserve
}

// getSizeEstimateBytes is entry_storage::get_size_estimate_bytes.
func (s *entryStorage) getSizeEstimateBytes() uint32 {
	sz := uint64(cap(s.data.buf))
	sz += uint64(s.indexer.capacity()) * offsetHashEntryBytes
	return uint32(sz)
}

// get is m_data.begin()+ofs: the bytes from ofs to the end of the vector.
func (s *entryStorage) get(ofs storeOffset) []byte { return s.data.buf[ofs:] }

func (s *entryStorage) entryCount() int { return s.indexer.size }

func (s *entryStorage) afterLastOffset() storeOffset {
	if s.reserve == entryStorageNoReserve {
		return s.dataSize
	}
	return s.reserve
}

func (s *entryStorage) hasReserve() bool { return s.reserve != entryStorageNoReserve }

func (s *entryStorage) ensureReserve() {
	if s.hasReserve() {
		return
	}
	s.reserve = s.dataSize
	s.resizeData(s.dataSize + storeOffset(s.entrySize))
}

func (s *entryStorage) reservePtr() []byte { return s.data.buf[s.reserve:] }

func (s *entryStorage) reserveContentAlreadyPresent() bool { return s.indexer.contains(s.reserve) }

// findReserveContent is entry_storage::find_reserve_content.
func (s *entryStorage) findReserveContent() (storeOffset, bool) {
	slot := s.indexer.findCore(s.reserve)
	if slot < 0 {
		return 0, false
	}
	return s.indexer.table[slot].data, true
}

// writeIntoReserve is entry_storage::write_into_reserve(const char*).
func (s *entryStorage) writeIntoReserve(data []byte) {
	s.ensureReserve()
	copy(s.reservePtr()[:s.entrySize], data[:s.entrySize])
}

// insertReserveContent is entry_storage::insert_reserve_content.
func (s *entryStorage) insertReserveContent() bool {
	entryOfs := s.indexer.insertIfNotThere(s.reserve)
	if s.reserve == entryOfs {
		// entry inserted, so reserve is no longer a reserve
		s.reserve = entryStorageNoReserve
		return true
	}
	return false
}

// insertOrGetReserveContent is entry_storage::insert_or_get_reserve_content.
func (s *entryStorage) insertOrGetReserveContent() storeOffset {
	entryOfs := s.indexer.insertIfNotThere(s.reserve)
	if s.reserve == entryOfs {
		s.reserve = entryStorageNoReserve
	}
	return entryOfs
}

// removeReserveContent is entry_storage::remove_reserve_content.
func (s *entryStorage) removeReserveContent() bool {
	entryOfs, found := s.findReserveContent()
	if !found {
		return false
	}
	s.removeOffset(entryOfs)
	return true
}

// removeOffset is entry_storage::remove_offset: the last row moves into the
// removed one's place and the vacated block becomes the reserve.
func (s *entryStorage) removeOffset(ofs storeOffset) {
	s.indexer.remove(ofs)
	lastOfs := s.afterLastOffset() - storeOffset(s.entrySize)
	if ofs != lastOfs {
		s.indexer.remove(lastOfs)
		copy(s.data.buf[ofs:ofs+storeOffset(s.entrySize)], s.data.buf[lastOfs:lastOfs+storeOffset(s.entrySize)])
		s.indexer.insert(ofs)
	}
	if s.hasReserve() {
		// we already have a reserve, so we need to shrink a little to keep having just one
		s.resizeData(s.dataSize - storeOffset(s.entrySize))
	}
	s.reserve = lastOfs
}

// resizeData is entry_storage::resize_data.
func (s *entryStorage) resizeData(sz storeOffset) {
	s.dataSize = sz
	if sz+8 < sz {
		panic("overflow resizing data section for sparse table")
	}
	s.data.resize(int(sz + 8))
}

func (s *entryStorage) insertOffset(ofs storeOffset) bool {
	return s.indexer.insertIfNotThere(ofs) == ofs
}
