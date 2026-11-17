package rel

// Source: src/muz/rel/dl_sparse_table.{h,cpp} sparse_table::column_info,
// sparse_table::column_layout and get_domain_length.
import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// sparseColumn is sparse_table::column_info: a column of length bits at bit
// offset of a row, read and written as the little-endian 64-bit word at
// byte bigOffset (rows keep 8 readable bytes past their end, see
// entryStorage.resizeData).
type sparseColumn struct {
	bigOffset, smallOffset uint32
	mask, writeMask        uint64
	offset, length         uint32
}

func newSparseColumn(offset, length uint32) sparseColumn {
	c := sparseColumn{bigOffset: offset / 8, smallOffset: offset % 8, offset: offset, length: length}
	if length == 64 {
		c.mask = ^uint64(0)
	} else {
		c.mask = uint64(1)<<length - 1
	}
	c.writeMask = ^(c.mask << c.smallOffset)
	return c
}

func (c sparseColumn) get(rec []byte) Element {
	res := binary.LittleEndian.Uint64(rec[c.bigOffset : c.bigOffset+8])
	res >>= c.smallOffset
	res &= c.mask
	return Element(res)
}

func (c sparseColumn) set(rec []byte, val Element) {
	w := rec[c.bigOffset : c.bigOffset+8]
	v := binary.LittleEndian.Uint64(w)
	v &= c.writeMask
	v |= uint64(val) << c.smallOffset
	binary.LittleEndian.PutUint64(w, v)
}

func (c sparseColumn) nextOfs() uint32 { return c.offset + c.length }

// SparseLayout is sparse_table::column_layout. entryBytes is m_entry_size,
// functionalBytes m_functional_part_size and functionalColumns
// m_functional_col_cnt.
type SparseLayout struct {
	columns                     []sparseColumn
	entryBytes, functionalBytes uint32
	functionalColumns           int
}

// sparseDomainBits is get_domain_length.
func sparseDomainBits(n uint64) uint32 {
	if n == 0 {
		return 1
	}
	if n <= uint64(^uint32(0)) {
		if n == 1 {
			return 1
		}
		return uint32(bits.Len32(uint32(n - 1)))
	}
	// Match get_domain_length's split into a low 32-bit word and a high
	// word. Its unary-high-word branch intentionally allocates 33 bits for
	// the exact 2^32 domain boundary.
	high := uint32(n >> 32)
	if uint32(n) != 0 && high != ^uint32(0) {
		high++
	}
	if high == 1 {
		return 33
	}
	if high > 0x80000000 {
		return 64
	}
	return 32 + uint32(bits.Len32(high-1))
}

// NewSparseLayout is column_layout::column_layout(sig). The nullary layout
// (zero bytes) is g3's: the source's sparse plugin cannot handle the empty
// signature.
func NewSparseLayout(signature Signature, functional int) (SparseLayout, error) {
	if functional < 0 || functional > signature.Len() {
		return SparseLayout{}, ErrTable
	}
	l := SparseLayout{functionalColumns: functional}
	if signature.Len() == 0 {
		return l, nil
	}
	var ofs uint32
	sigSz := signature.Len()
	firstFunctional := sigSz - functional
	for i := 0; i < sigSz; i++ {
		length := sparseDomainBits(signature.domains[i])
		if len(l.columns) > 0 && (length > 54 || i == firstFunctional) {
			// large domains must start byte-aligned, as well as functional columns
			l.makeByteAlignedEnd(len(l.columns) - 1)
			ofs = l.columns[len(l.columns)-1].nextOfs()
		}
		l.columns = append(l.columns, newSparseColumn(ofs, length))
		ofs += length
	}
	l.makeByteAlignedEnd(len(l.columns) - 1)
	l.entryBytes = l.columns[len(l.columns)-1].nextOfs() / 8
	if functional != 0 {
		l.functionalBytes = l.entryBytes - l.columns[firstFunctional].offset/8
	}
	return l, nil
}

// makeByteAlignedEnd is column_layout::make_byte_aligned_end: the padding
// up to the next byte is swallowed by the preceding columns, the last first.
func (l *SparseLayout) makeByteAlignedEnd(colIndex0 int) {
	ofs := l.columns[colIndex0].nextOfs()
	ofsBitPart := ofs % 8
	roundedOfs := ofs
	if ofsBitPart != 0 {
		roundedOfs = ofs + 8 - ofsBitPart
	}
	if roundedOfs != ofs {
		diff := int(roundedOfs - ofs)
		colIdx := colIndex0 + 1
		for diff != 0 {
			colIdx--
			ci := l.columns[colIdx]
			newLength := ci.length
			if ci.length < 64 {
				swallowed := min(64-int(ci.length), diff)
				diff -= swallowed
				newLength += uint32(swallowed)
			}
			newOfs := ci.offset + uint32(diff)
			l.columns[colIdx] = newSparseColumn(newOfs, newLength)
		}
	}
}

func (l SparseLayout) EntryBytes() uint32 { return l.entryBytes }

func (l SparseLayout) get(rec []byte, col int) Element { return l.columns[col].get(rec) }

func (l SparseLayout) set(rec []byte, col int, val Element) { l.columns[col].set(rec, val) }

// checkFact is the value check of write_into_reserve's SASSERT(f[i] <
// get_signature()[i]), an error in g3.
func (l SparseLayout) checkFact(signature Signature, f Fact) error {
	if len(f) != len(l.columns) {
		return fmt.Errorf("%w: fact arity mismatch", ErrTable)
	}
	for i, v := range f {
		if uint64(v) >= signature.domains[i] {
			return fmt.Errorf("%w: fact value outside column domain", ErrTable)
		}
	}
	return nil
}

// unpack reads every column of the row at rec.
func (l SparseLayout) unpack(rec []byte) Fact {
	out := make(Fact, len(l.columns))
	for i, c := range l.columns {
		out[i] = c.get(rec)
	}
	return out
}
