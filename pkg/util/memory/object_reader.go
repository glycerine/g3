package memory

// A run of word reads from one allocator object, validated once.

import (
	"encoding/binary"

	"github.com/glycerine/g3/pkg/arena"
)

// SmallObjectReader reads initialized words of one live allocator object.
// Reads change nothing, so the allocator, the object and its runtime block
// are validated once, by Reader; each ReadUint32 then keeps the requested
// bound, runtime extent and initialization tests of SmallAllocator.ReadUint32
// in the same order and returns the same word or error, provided nothing
// changes the allocator, its runtime or the object between Reader and the
// reads. Callers hold a reader only across a run of reads.
type SmallObjectReader struct {
	data              []byte
	initialized       []uint64
	requested, offset uint64
	extent, header    uint64
	blockErr          error
	allocator         *SmallAllocator // checked builds: full ReadUint32
	ref               ObjectRef
}

// Reader returns the error SmallAllocator.ReadUint32 would report first for
// ref (allocator or object), or a reader for its words. A runtime block or
// header failure is reported by the reads, after their requested bound, as
// ReadUint32 orders it.
func (a *SmallAllocator) Reader(ref ObjectRef) (SmallObjectReader, error) {
	var r SmallObjectReader
	e := a.ReaderInto(ref, &r)
	return r, e
}

// ReaderInto is Reader writing the reader into *dst, which it leaves
// unchanged when it returns an error (Reader then returns a zero reader).
// It saves the by-value copies of the reader that Reader's callers make.
func (a *SmallAllocator) ReaderInto(ref ObjectRef, dst *SmallObjectReader) error {
	if e := a.open(); e != nil {
		return e
	}
	loc, e := a.object(ref)
	if e != nil {
		return e
	}
	if objectReaderValidate {
		*dst = SmallObjectReader{allocator: a, ref: ref}
		return nil
	}
	m := a.config.Runtime
	b, e := m.block(loc.raw)
	if e != nil {
		*dst = SmallObjectReader{requested: loc.requested, offset: uint64(loc.offset), blockErr: e}
		return nil
	}
	n, e := m.headerRead(b)
	if e != nil {
		*dst = SmallObjectReader{requested: loc.requested, offset: uint64(loc.offset), blockErr: e}
		return nil
	}
	header := uint64(m.config.SizeTBits / 8)
	// Every field, as the composite literal of Reader set them.
	dst.data, dst.initialized = b.data, b.initialized
	dst.requested, dst.offset = loc.requested, uint64(loc.offset)
	dst.extent, dst.header = n-header, header
	dst.blockErr, dst.allocator, dst.ref = nil, nil, ObjectRef{}
	return nil
}

// ReadUint32 is SmallAllocator.ReadUint32(ref, offset) for the reader's ref.
func (r *SmallObjectReader) ReadUint32(offset uint64) (uint32, error) {
	if objectReaderValidate {
		return r.allocator.ReadUint32(r.ref, offset)
	}
	if offset > r.requested || r.requested-offset < 4 {
		return 0, arena.ErrBounds
	}
	if r.blockErr != nil {
		return 0, r.blockErr
	}
	o := r.offset + offset
	if o > r.extent || r.extent-o < 4 {
		return 0, arena.ErrBounds
	}
	at := r.header + o
	if !initializedRangeSet(r.initialized, at, 4) {
		return 0, ErrRuntimeUninitialized
	}
	return binary.LittleEndian.Uint32(r.data[at : at+4]), nil
}

// Bytes returns the n bytes at offset when ReadUint32 would succeed for every
// word in [offset, offset+n), so a caller can compare those words in place.
// Otherwise ok is false (always in checked and debug builds, whose reads go
// through SmallAllocator.ReadUint32), and the caller reads word by word to get
// ReadUint32's exact results and error order. n must be a positive multiple
// of 4.
func (r *SmallObjectReader) Bytes(offset, n uint64) (b []byte, ok bool) {
	if objectReaderValidate || n == 0 || n%4 != 0 {
		return nil, false
	}
	if offset > r.requested || r.requested-offset < n || r.blockErr != nil {
		return nil, false
	}
	o := r.offset + offset
	if o > r.extent || r.extent-o < n {
		return nil, false
	}
	at := r.header + o
	if !initializedRangeSet(r.initialized, at, n) {
		return nil, false
	}
	return r.data[at : at+n], true
}
