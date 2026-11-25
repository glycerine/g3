package sat

// Source sat::watched stores two 32-bit words. In a clause watch the first
// word addresses a clause. A Go list shares the allocator owner across its
// ordinary clause addresses and keeps full offsets for the exceptional ones.
// Every backing owns its words and the matching optional offset column.
type satWatchBacking struct {
	data     []satWatchWords
	wide     *satWideWatchOffsets
	owner    uint64
	ownerSet bool
	alive    bool // a captured iterator may still refer to a retired backing
}
type satWideWatchOffsets struct{ values []satClauseOffset }

type satWatchWords struct{ val1, val2 uint32 }

const satWatchSlotBits = 24
const satWatchMaxGeneration = 127 // the all-ones high byte denotes a full offset
const satWatchWide = uint32(1)<<(32-satWatchSlotBits) - 1

func (b *satWatchBacking) invalidate() { *b = satWatchBacking{} }
func (b *satWatchBacking) offset(w satWatchWords, i uint32) satClauseOffset {
	code := w.val1 >> satWatchSlotBits
	if code == satWatchWide {
		return b.wide.values[i]
	}
	return satClauseOffset{owner: b.owner, slot: w.val1 & ((1 << satWatchSlotBits) - 1), generation: code >> 1, storageKind: code & 1}
}
func (b *satWatchBacking) read(i uint32) satWatched {
	w := b.data[i]
	r := satWatched{val1: w.val1, val2: w.val2}
	if r.Clause() {
		r.locator = b.offset(w, i)
		r.val1 = r.locator.slot
	}
	return r
}
func (b *satWatchBacking) store(i uint32, w satWatched) {
	first := w.val1
	if w.Clause() {
		o := w.locator
		o.slot = first
		if !b.ownerSet {
			b.owner = o.owner
			b.ownerSet = true
		}
		if b.owner == o.owner && first < 1<<satWatchSlotBits && o.generation < satWatchMaxGeneration && o.storageKind <= 1 {
			first |= (o.generation<<1 | o.storageKind) << satWatchSlotBits
		} else {
			if b.wide == nil {
				b.wide = &satWideWatchOffsets{values: make([]satClauseOffset, cap(b.data))}
			}
			b.wide.values[i] = o
			first = satWatchWide << satWatchSlotBits
		}
	}
	b.data[i] = satWatchWords{first, w.val2}
}
func (b *satWatchBacking) swap(i, j int) {
	b.data[i], b.data[j] = b.data[j], b.data[i]
	if b.wide != nil {
		b.wide.values[i], b.wide.values[j] = b.wide.values[j], b.wide.values[i]
	}
}

// The propagation loop may move a compact clause word directly between lists
// that reference the same allocator. Other cases use the checked insertion.
func (x *satSolverWatchListRecord) appendEncoded(w satWatchWords, owner uint64) bool {
	b := x.backing
	if b == nil || b.owner != owner || !b.ownerSet || w.val1>>satWatchSlotBits == satWatchWide {
		return false
	}
	n := len(b.data)
	if n >= cap(b.data) || uint64(n) >= uint64(^uint32(0)) {
		return false
	}
	b.data = b.data[:n+1]
	b.data[n] = w
	return true
}

func (b *satWatchBacking) copyRange(to, from, end uint32) {
	copy(b.data[to:to+(end-from)], b.data[from:end])
	if b.wide != nil {
		copy(b.wide.values[to:to+(end-from)], b.wide.values[from:end])
	}
}
