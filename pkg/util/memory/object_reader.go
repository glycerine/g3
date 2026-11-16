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
	if e := a.open(); e != nil {
		return SmallObjectReader{}, e
	}
	loc, e := a.object(ref)
	if e != nil {
		return SmallObjectReader{}, e
	}
	if objectReaderValidate {
		return SmallObjectReader{allocator: a, ref: ref}, nil
	}
	r := SmallObjectReader{requested: loc.requested, offset: uint64(loc.offset)}
	m := a.config.Runtime
	b, e := m.block(loc.raw)
	if e != nil {
		r.blockErr = e
		return r, nil
	}
	n, e := m.headerRead(b)
	if e != nil {
		r.blockErr = e
		return r, nil
	}
	r.header = uint64(m.config.SizeTBits / 8)
	r.extent = n - r.header
	r.data, r.initialized = b.data, b.initialized
	return r, nil
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
