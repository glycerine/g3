package memory

// Explicit little-endian word views over the sole initialized-byte backing.

import (
	"encoding/binary"

	"github.com/glycerine/g3/pkg/arena"
)

// ReadUint32 reads one initialized word, without borrowing a backing slice.
// The complete word range is checked at this access, with no offset overflow.

func (m *Runtime) ReadUint32(ref Block, offset uint64) (uint32, error) {
	b, e := m.block(ref)
	if e != nil {
		return 0, e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return 0, e
	}
	header := uint64(m.config.SizeTBits / 8)
	extent := n - header
	if offset > extent || extent-offset < 4 {
		return 0, arena.ErrBounds
	}
	at := header + offset
	if !initializedRangeSet(b.initialized, at, 4) {
		return 0, ErrRuntimeUninitialized
	}
	return binary.LittleEndian.Uint32(b.data[at : at+4]), nil
}

// WriteUint32 validates the whole word before publishing its four bytes.
// It neither reserves storage nor alters source allocation counters.
func (m *Runtime) WriteUint32(ref Block, offset uint64, word uint32) error {
	b, e := m.block(ref)
	if e != nil {
		return e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return e
	}
	header := uint64(m.config.SizeTBits / 8)
	extent := n - header
	if offset > extent || extent-offset < 4 {
		return arena.ErrBounds
	}
	at := header + offset
	binary.LittleEndian.PutUint32(b.data[at:at+4], word)
	markInitializedRange(b.initialized, at, 4)
	return nil
}

// ReadUint32 retains the allocator byte API's requested-size bound. Source
// typed-object views can instead use ObjectExtent and the actual raw location.
func (a *SmallAllocator) ReadUint32(ref ObjectRef, offset uint64) (uint32, error) {
	if e := a.open(); e != nil {
		return 0, e
	}
	loc, e := a.object(ref)
	if e != nil {
		return 0, e
	}
	if offset > loc.requested || loc.requested-offset < 4 {
		return 0, arena.ErrBounds
	}
	return a.config.Runtime.ReadUint32(loc.raw, uint64(loc.offset)+offset)
}

func (a *SmallAllocator) WriteUint32(ref ObjectRef, offset uint64, word uint32) error {
	if e := a.open(); e != nil {
		return e
	}
	loc, e := a.object(ref)
	if e != nil {
		return e
	}
	if offset > loc.requested || loc.requested-offset < 4 {
		return arena.ErrBounds
	}
	return a.config.Runtime.WriteUint32(loc.raw, uint64(loc.offset)+offset, word)
}

// ObjectExtent reports actual allocated storage: the rounded class block for a
// pooled object, or the stored runtime extent for a direct object. It does not
// widen Read/Write requested bounds or permit access past the object's lifetime.
func (a *SmallAllocator) ObjectExtent(ref ObjectRef) (uint64, error) {
	if e := a.open(); e != nil {
		return 0, e
	}
	loc, e := a.object(ref)
	if e != nil {
		return 0, e
	}
	if loc.kind == smallObjectPooled {
		return uint64(loc.extent), nil
	}
	return a.config.Runtime.Size(loc.raw)
}

// CopyUint32s copies count words from src at srcOffset to dst at dstOffset.
// It applies the ReadUint32/WriteUint32 checks once for the whole range:
// both blocks live, both ranges in bounds, and every source byte
// initialized (checked before anything is written). Destination bytes are
// marked initialized, as WriteUint32 does. Distinct objects never overlap.
func (m *Runtime) CopyUint32s(dst Block, dstOffset uint64, src Block, srcOffset uint64, count uint64) error {
	if count == 0 {
		return nil
	}
	if count > ^uint64(0)/4 {
		return arena.ErrBounds
	}
	sb, e := m.block(src)
	if e != nil {
		return e
	}
	db, e := m.block(dst)
	if e != nil {
		return e
	}
	sn, e := m.headerRead(sb)
	if e != nil {
		return e
	}
	dn, e := m.headerRead(db)
	if e != nil {
		return e
	}
	header := uint64(m.config.SizeTBits / 8)
	bytes := 4 * count
	if sn < header || dn < header {
		return arena.ErrBounds
	}
	sext, dext := sn-header, dn-header
	if srcOffset > sext || sext-srcOffset < bytes || dstOffset > dext || dext-dstOffset < bytes {
		return arena.ErrBounds
	}
	sAt, dAt := header+srcOffset, header+dstOffset
	if !initializedRangeSet(sb.initialized, sAt, bytes) {
		return ErrRuntimeUninitialized
	}
	copy(db.data[dAt:dAt+bytes], sb.data[sAt:sAt+bytes])
	markInitializedRange(db.initialized, dAt, bytes)
	return nil
}

// initializedRangeSet reports whether bytes [from, from+n) are all
// initialized, testing up to 64 bits per bitmap word instead of one bit per
// byte. Same answer as the per-byte loop it replaces.
func initializedRangeSet(bits []uint64, from, n uint64) bool {
	for n > 0 {
		w, off := from/64, from%64
		k := 64 - off
		if k > n {
			k = n
		}
		mask := ^uint64(0)
		if k < 64 {
			mask = (uint64(1)<<k - 1) << off
		}
		if bits[w]&mask != mask {
			return false
		}
		from, n = from+k, n-k
	}
	return true
}

// markInitializedRange marks bytes [from, from+n) initialized, a bitmap
// word at a time.
func markInitializedRange(bits []uint64, from, n uint64) {
	for n > 0 {
		w, off := from/64, from%64
		k := 64 - off
		if k > n {
			k = n
		}
		mask := ^uint64(0)
		if k < 64 {
			mask = (uint64(1)<<k - 1) << off
		}
		bits[w] |= mask
		from, n = from+k, n-k
	}
}
