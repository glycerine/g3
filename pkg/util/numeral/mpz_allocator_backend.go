package numeral

// Source: src/util/mpz.h; src/util/mpz.cpp (_MP_INTERNAL, digit32).
// Actual mpz.h cell allocation and object-bounded initialized-byte access.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Go safety metadata, to be embedded/charged in the numeric descriptor.
// The actual source payload remains one contiguous 8+4*capacity object.

type SourceCellBacking struct {
	Object                    umemory.ObjectRef
	Raw                       umemory.Block
	Offset, extent, Requested uint64
}

func sourceMPZCellBytes(capacity uint32) uint64 { return uint64(uint32(8) + uint32(4)*capacity) }

// requested retains the result of the source unsigned32 cell-byte expression,
// widened only for the Go storage API. It cannot legitimately exceed uint32.
// Guard corrupt retained metadata before source size_t conversion can erase it.
func (c SourceCellBacking) deallocate(a *umemory.SmallAllocator) (umemory.ReleaseProgress, error) {
	if c.Requested > uint64(^uint32(0)) {
		return umemory.ReleaseProgress{}, arena.ErrBounds
	}
	return a.DeallocateTracked(c.Requested, c.Object)
}

// The returned scalar root is nonzero on a completed allocation even if the
// capacity write fails. Its caller must retain it before descriptor preparation.
func allocateSourceCell(a *umemory.SmallAllocator, capacity uint32) (SourceCellBacking, error) {
	return allocateSourceCellWithOutcome(a, capacity, nil)
}
func allocateSourceCellWithOutcome(a *umemory.SmallAllocator, capacity uint32, outcome *umemory.SmallAllocatorAllocationOutcome) (SourceCellBacking, error) {
	var c SourceCellBacking
	c.Requested = sourceMPZCellBytes(capacity)
	object, e := a.AllocateWithOutcome(c.Requested, outcome)
	c.Object = object
	if e != nil {
		return c, e
	}
	if object == (umemory.ObjectRef{}) {
		return c, arena.ErrBounds
	} // actual null header access after size0 allocation
	c.extent, e = a.ObjectExtent(object)
	if e != nil {
		return c, e
	}
	c.Raw, c.Offset, e = a.RawLocation(object)
	if e != nil {
		return c, e
	}
	// Digits live in a native payload (sourceCellDigits); no budget event.
	if e = a.AttachWords(object, sourceCellDigitWords(c.extent)); e != nil {
		return c, e
	}
	return c, c.writeWord(a, 4, capacity) // allocate() initializes capacity ONLY
}

// sourceCellDigitOffset is the byte offset of m_digits in the source cell.
// The 8-byte header (m_size, m_capacity) stays in the memory runtime; the
// digit array, which z3 indexes directly, is the object's native word
// payload (SmallAllocator.AttachWords), so digit access is slice indexing.
// Digit word i at source offset 8+4*i is payload index i.
const sourceCellDigitOffset = 8

func sourceCellDigitWords(extent uint64) uint64 {
	if extent < sourceCellDigitOffset {
		return 0
	}
	return (extent - sourceCellDigitOffset) / 4
}

// digitIndex maps a validated source offset inside the digit array to its
// payload index. Unaligned digit offsets have no native word.
func digitIndex(offset uint64) (uint64, error) {
	if (offset-sourceCellDigitOffset)%4 != 0 {
		return 0, arena.ErrBounds
	}
	return (offset - sourceCellDigitOffset) / 4, nil
}
func (c SourceCellBacking) location(a *umemory.SmallAllocator, offset uint64) (umemory.Block, uint64, error) {
	// Revalidate object generation before borrowing the underlying pooled chunk.
	// A raw chunk alone would otherwise authorize reads after cell recycle.
	raw, start, e := a.RawLocation(c.Object)
	if e != nil {
		return umemory.Block{}, 0, e
	}
	extent, e := a.ObjectExtent(c.Object)
	if e != nil {
		return umemory.Block{}, 0, e
	}
	if offset > extent || extent-offset < 4 {
		return umemory.Block{}, 0, arena.ErrBounds
	}
	if raw != c.Raw || start != c.Offset {
		return umemory.Block{}, 0, umemory.ErrAllocatorOwner
	}
	if start > ^uint64(0)-offset {
		return umemory.Block{}, 0, arena.ErrBounds
	}
	return raw, start + offset, nil
}

// copyWordsFrom copies count digit words at offset from src into c,
// validating each cell's location and range once. Both cells' payloads are
// native; the source cell may belong to another allocator.
func (c SourceCellBacking) copyWordsFrom(a *umemory.SmallAllocator, src SourceCellBacking, srcA *umemory.SmallAllocator, offset, count uint64) error {
	if count == 0 {
		return nil
	}
	if offset < sourceCellDigitOffset {
		return arena.ErrBounds
	}
	bytes := 4 * count
	last := offset + bytes - 4
	if _, _, e := src.location(srcA, offset); e != nil {
		return e
	}
	if _, _, e := src.location(srcA, last); e != nil {
		return e
	}
	if _, _, e := c.location(a, offset); e != nil {
		return e
	}
	if _, _, e := c.location(a, last); e != nil {
		return e
	}
	at, e := digitIndex(offset)
	if e != nil {
		return e
	}
	// Validated lookups: an object without a payload for its generation
	// (ErrRuntimeUninitialized) must stop the copy, never read or write the
	// slot's previous payload.
	sw, e := srcA.Words(src.Object)
	if e != nil {
		return e
	}
	dw, e := a.Words(c.Object)
	if e != nil {
		return e
	}
	if at+count > uint64(len(sw)) || at+count > uint64(len(dw)) {
		return arena.ErrBounds
	}
	copy(dw[at:at+count], sw[at:at+count])
	return nil
}

// digitsUnchecked returns the cell's native digit payload without
// re-validating the pooled object (normal builds; numeric_validate_off.go).
func (c *SourceCellBacking) digitsUnchecked(a *umemory.SmallAllocator) []uint32 {
	return a.WordsUnchecked(c.Object)
}

// ReadWord reads the source cell word at a byte offset: a header word from
// the memory runtime, or a digit from the native payload. Both validate the
// object's generation and the rounded extent first.
func (c SourceCellBacking) ReadWord(a *umemory.SmallAllocator, offset uint64) (uint32, error) {
	raw, at, e := c.location(a, offset)
	if e != nil {
		return 0, e
	}
	if offset >= sourceCellDigitOffset {
		i, e := digitIndex(offset)
		if e != nil {
			return 0, e
		}
		w, e := a.Words(c.Object)
		if e != nil {
			return 0, e
		}
		if i >= uint64(len(w)) {
			return 0, arena.ErrBounds
		}
		return w[i], nil
	}
	return a.RuntimeOwner().ReadUint32(raw, at)
}
func (c SourceCellBacking) writeWord(a *umemory.SmallAllocator, offset uint64, word uint32) error {
	raw, at, e := c.location(a, offset)
	if e != nil {
		return e
	}
	if offset >= sourceCellDigitOffset {
		i, e := digitIndex(offset)
		if e != nil {
			return e
		}
		w, e := a.Words(c.Object)
		if e != nil {
			return e
		}
		if i >= uint64(len(w)) {
			return arena.ErrBounds
		}
		w[i] = word
		return nil
	}
	return a.RuntimeOwner().WriteUint32(raw, at, word)
}

// writeHeaderUnchecked writes a header word through the cached raw location
// without re-validating the pooled object (normal builds). The runtime still
// bounds-checks the raw block.
func (c *SourceCellBacking) writeHeaderUnchecked(a *umemory.SmallAllocator, offset uint64, word uint32) error {
	if offset > c.extent || c.extent-offset < 4 {
		return arena.ErrBounds
	}
	return a.RuntimeOwner().WriteUint32(c.Raw, c.Offset+offset, word)
}
