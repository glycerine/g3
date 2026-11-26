package arena

import (
	"encoding/binary"
	"reflect"
)

const (
	clausePageShift       = 6
	clausePageSlots       = 1 << clausePageShift
	clausePageDirectSize  = 248 // SmallBytes's direct threshold at the default alignment.
	clausePageChunkBytes  = 8192 - 2*8
	clausePageHeaderBytes = 4 // The full generation, zero in an unused or released row.
)

type clausePage struct {
	data            []byte
	stride, count   uint32
	taken, live     uint32
	firstFree, next uint32 // one-based row within the page and next nonfull page
	epoch, maximum  uint32
	_               uint64 // Keep the directory stride at 64 bytes on amd64.
}

type clausePageClass struct {
	first, warm, growth uint32 // one-based nonfull and warm page IDs, initial growth stage
}

type clausePageClasses [clausePageDirectSize / 4]clausePageClass

// ClauseBytes stores word-sized SAT clause records. In the pooled layout a
// slot addresses its physical page and row. Each row starts with its full
// generation, so borrowing needs no separate allocation descriptor.
// Explicit SmallBytes configurations use that allocator's layout and policy.
type ClauseBytes struct {
	compat    *SmallBytes
	pages     []clausePage
	classes   *clausePageClasses
	budget    *Budget
	directory uint64
	resident  uint64
	requested uint64
	live      uint32
	free      uint32
	closed    bool
}

var (
	clauseBytesOwnerCharge = uint64(reflect.TypeOf(ClauseBytes{}).Size())
	clausePageClassCharge  = uint64(reflect.TypeOf(clausePageClasses{}).Size())
	clausePageDirStride    = uint64(reflect.TypeOf(clausePage{}).Size())
)

func NewClauseBytes(c SmallBytesConfig) (*ClauseBytes, error) {
	budget := c.Descriptors.Budget
	if c.DirectOnly || c.ChunkSize != 0 || c.Alignment != 0 || c.Descriptors.PageSize != 0 || c.Descriptors.SlotLimit != 0 {
		compat, err := NewSmallBytes(c)
		if err != nil {
			return nil, err
		}
		if err := budget.Reserve(clauseBytesOwnerCharge); err != nil {
			compat.Close()
			return nil, err
		}
		return &ClauseBytes{compat: compat, budget: budget}, nil
	}
	if err := budget.Reserve(clauseBytesOwnerCharge + clausePageClassCharge); err != nil {
		return nil, err
	}
	return &ClauseBytes{classes: new(clausePageClasses), budget: budget}, nil
}

// RequiresDirect tells the SAT allocator to use its direct store for wide
// records when using the physical page layout.
func (a *ClauseBytes) RequiresDirect(size uint64) bool {
	return a.classes != nil && size >= clausePageDirectSize
}

func (a *ClauseBytes) addPage(stride, preferred uint32) error {
	if uint64(len(a.pages)) >= uint64(^uint32(0))/clausePageSlots {
		return ErrSlotLimit
	}
	count := min(preferred, clausePageChunkBytes/stride)
	bytes := uint64(count) * uint64(stride)
	directory := uint64(0)
	capacity := cap(a.pages)
	if len(a.pages) == capacity {
		capacity = max(1, capacity*2)
		directory = uint64(capacity) * clausePageDirStride
		if directory > uint64(^uint(0)>>1) {
			return ErrBounds
		}
	}
	if err := a.budget.Reserve(bytes + directory); err != nil {
		return err
	}
	if directory != 0 {
		pages := make([]clausePage, len(a.pages), capacity)
		copy(pages, a.pages)
		a.pages = pages
		a.budget.Release(a.directory)
		a.directory = directory
	}
	a.pages = append(a.pages, clausePage{
		data: make([]byte, int(bytes)), stride: stride, count: count, epoch: 1, maximum: 1,
	})
	a.resident += bytes
	return nil
}

func (a *ClauseBytes) Allocate(size uint32) (SmallBytesRef, error) {
	if a.compat != nil {
		return a.compat.Allocate(size)
	}
	if a.closed {
		return SmallBytesRef{}, ErrClosed
	}
	if size == 0 {
		return SmallBytesRef{}, nil
	}
	// A free row stores two words in the space otherwise used by the clause.
	if size < 8 || size >= clausePageDirectSize || size&3 != 0 {
		return SmallBytesRef{}, ErrConfig
	}
	stride := size + clausePageHeaderBytes
	class := &a.classes[size>>2]
	if class.first == 0 {
		preferred := uint32(clausePageSlots)
		switch class.growth {
		case 0:
			preferred = 8
		case 1:
			preferred = 32
		}
		if err := a.addPage(stride, preferred); err != nil {
			return SmallBytesRef{}, err
		}
		class.first = uint32(len(a.pages))
		if class.growth < 2 {
			class.growth++
		}
	}
	pageID := class.first
	page := &a.pages[pageID-1]
	if page.data == nil {
		bytes := uint64(page.count) * uint64(stride)
		if err := a.budget.Reserve(bytes); err != nil {
			return SmallBytesRef{}, err
		}
		page.data = make([]byte, int(bytes))
		a.resident += bytes
	}
	if class.warm == pageID {
		class.warm = 0
	}
	var index uint32
	generation := page.epoch
	if page.firstFree != 0 {
		index = page.firstFree - 1
		start := index*stride + clausePageHeaderBytes
		free := page.data[start : start+8]
		page.firstFree = binary.LittleEndian.Uint32(free)
		generation = binary.LittleEndian.Uint32(free[4:])
		a.free--
	} else {
		index = page.taken
		page.taken++
	}
	binary.LittleEndian.PutUint32(page.data[index*stride:], generation)
	page.maximum = max(page.maximum, generation)
	page.live++
	if page.firstFree == 0 && page.taken == page.count {
		class.first = page.next
		page.next = 0
	}
	a.live++
	a.requested += uint64(size)
	return SmallBytesRef{Slot((pageID-1)*clausePageSlots + index + 1), Generation(generation)}, nil
}

// Borrow returns the exact payload for a live row. Page directory growth keeps
// that payload in place. Its lifetime ends when the row is deallocated.
func (a *ClauseBytes) Borrow(r SmallBytesRef) ([]byte, error) {
	if a.compat != nil {
		return a.compat.Borrow(r)
	}
	if a.closed {
		return nil, ErrClosed
	}
	if r == (SmallBytesRef{}) {
		return nil, nil
	}
	n := uint32(r.Slot) - 1
	page := n >> clausePageShift
	if uint64(page) >= uint64(len(a.pages)) || r.Generation == 0 {
		return nil, ErrInvalidSlot
	}
	p := &a.pages[page]
	index := n & (clausePageSlots - 1)
	if index >= p.taken {
		return nil, ErrInvalidSlot
	}
	start := index * p.stride
	end := start + p.stride
	row := p.data[start:end:end]
	if binary.LittleEndian.Uint32(row) != uint32(r.Generation) {
		return nil, ErrInvalidSlot
	}
	return row[clausePageHeaderBytes:], nil
}

func (a *ClauseBytes) IsLive(r SmallBytesRef) bool {
	if a.compat != nil {
		return a.compat.IsLive(r)
	}
	data, err := a.Borrow(r)
	return err == nil && data != nil
}

func (a *ClauseBytes) ReadUint32LE(r SmallBytesRef, offset uint32) (uint32, error) {
	if a.compat != nil {
		return a.compat.ReadUint32LE(r, offset)
	}
	data, err := a.Borrow(r)
	if err != nil {
		return 0, err
	}
	if uint64(offset)+4 > uint64(len(data)) {
		return 0, ErrBounds
	}
	return binary.LittleEndian.Uint32(data[offset:]), nil
}

func (a *ClauseBytes) WriteUint32LE(r SmallBytesRef, offset, value uint32) error {
	if a.compat != nil {
		return a.compat.WriteUint32LE(r, offset, value)
	}
	data, err := a.Borrow(r)
	if err != nil {
		return err
	}
	if uint64(offset)+4 > uint64(len(data)) {
		return ErrBounds
	}
	binary.LittleEndian.PutUint32(data[offset:], value)
	return nil
}

func (a *ClauseBytes) Deallocate(size uint32, r SmallBytesRef) error {
	if a.compat != nil {
		return a.compat.Deallocate(size, r)
	}
	if a.closed {
		return ErrClosed
	}
	if size == 0 {
		return nil
	}
	data, err := a.Borrow(r)
	if err != nil {
		return err
	}
	if data == nil {
		return ErrInvalidSlot
	}
	if uint64(len(data)) != uint64(size) {
		return ErrSizeMismatch
	}
	n := uint32(r.Slot) - 1
	page := n >> clausePageShift
	index := n & (clausePageSlots - 1)
	p := &a.pages[page]
	binary.LittleEndian.PutUint32(p.data[index*p.stride:], 0)
	class := &a.classes[size>>2]
	pageID := page + 1
	if r.Generation != Generation(^uint32(0)) {
		if p.firstFree == 0 && p.taken == p.count {
			p.next = class.first
			class.first = pageID
		}
		binary.LittleEndian.PutUint32(data, p.firstFree)
		binary.LittleEndian.PutUint32(data[4:], uint32(r.Generation)+1)
		p.firstFree = index + 1
		a.free++
	}
	p.live--
	// Keep one empty page per size for reuse. An exhausted page retains its
	// row identities, since there is no epoch beyond its highest generation.
	if p.live == 0 && p.maximum != ^uint32(0) {
		if class.warm != 0 {
			bytes := uint64(len(p.data))
			a.resident -= bytes
			a.free -= p.taken
			p.data = nil
			p.firstFree, p.taken = 0, 0
			p.epoch = p.maximum + 1
			a.budget.Release(bytes)
		} else {
			class.warm = pageID
		}
	}
	a.live--
	a.requested -= uint64(size)
	return nil
}

func (a *ClauseBytes) Stats() SmallBytesStats {
	if a.closed {
		return SmallBytesStats{}
	}
	if a.compat != nil {
		s := a.compat.Stats()
		s.OwnerBytes += clauseBytesOwnerCharge
		return s
	}
	var pages uint32
	var generations uint64
	for i := range a.pages {
		p := &a.pages[i]
		if p.data != nil {
			pages++
			generations += uint64(p.count) * clausePageHeaderBytes
		}
	}
	payload := a.resident - generations
	return SmallBytesStats{
		OwnerBytes: clauseBytesOwnerCharge + clausePageClassCharge, Live: a.live, Chunks: pages,
		FreeObjects: a.free, PayloadBytes: payload, MetadataBytes: generations, DirectoryBytes: a.directory,
		RequestedBytes: a.requested, SourceAllocationBytes: a.requested, WastedBytes: payload - a.requested,
	}
}

func (a *ClauseBytes) Close() {
	if a.closed {
		return
	}
	if a.compat != nil {
		a.compat.Close()
		a.compat = nil
		a.budget.Release(clauseBytesOwnerCharge)
	} else {
		a.budget.Release(clauseBytesOwnerCharge + clausePageClassCharge + a.directory + a.resident)
		a.pages = nil
		a.classes = nil
	}
	a.closed = true
}
