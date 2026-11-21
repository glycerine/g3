package ast

// RetainedLiteralBuffer owns references to signed expressions in one manager.
// Each position keeps the arena slot and its full generation. All positions
// share the manager's origin. Release consumes a range in first-to-last order.
type RetainedLiteralBuffer struct {
	manager *ASTManager
	refs    []astNodeRef
	signs   []uint64
}

func (m *ASTManager) NewRetainedLiteralBuffer(size int) (*RetainedLiteralBuffer, error) {
	if m == nil || m.Self != m {
		return nil, ErrASTHandle
	}
	if size < 0 {
		return nil, ErrASTDomain
	}
	if !m.lockFree() {
		if err := m.lock(); err != nil {
			return nil, err
		}
		m.mu.Unlock()
	}
	var words int
	if size != 0 {
		words = 1 + (size-1)/64
	}
	return &RetainedLiteralBuffer{m, make([]astNodeRef, size), make([]uint64, words)}, nil
}

func (b *RetainedLiteralBuffer) Len() int { return len(b.refs) }

func (b *RetainedLiteralBuffer) negative(i int) bool {
	return b.signs[i/64]&(uint64(1)<<uint(i%64)) != 0
}

func (b *RetainedLiteralBuffer) At(i int) (ASTHandle, bool, error) {
	if b == nil || uint(i) >= uint(len(b.refs)) {
		return ASTHandle{}, false, ErrASTDomain
	}
	return b.manager.external(b.refs[i]), b.negative(i), nil
}

// Retain acquires h in an empty position. The manager validates h, including
// its origin and generation, before the buffer takes ownership.
func (b *RetainedLiteralBuffer) Retain(i int, h ASTHandle, negative bool) error {
	if b == nil || uint(i) >= uint(len(b.refs)) || b.refs[i].slot != 0 {
		return ErrASTDomain
	}
	if err := b.manager.IncRef(h); err != nil {
		return err
	}
	b.refs[i] = astNodeRef{h.slot, h.generation}
	mask := uint64(1) << uint(i%64)
	if negative {
		b.signs[i/64] |= mask
	} else {
		b.signs[i/64] &^= mask
	}
	return nil
}

// Swap exchanges two saved literals. Both indices must be in the buffer.
func (b *RetainedLiteralBuffer) Swap(i, j int) {
	b.refs[i], b.refs[j] = b.refs[j], b.refs[i]
	if b.negative(i) != b.negative(j) {
		b.signs[i/64] ^= uint64(1) << uint(i%64)
		b.signs[j/64] ^= uint64(1) << uint(j%64)
	}
}

// Release leaves consumed positions null, with their original signs. On error
// the same range can be retried, including when a destructor has consumed its
// reference but has pending work.
func (b *RetainedLiteralBuffer) Release(first, count int) error {
	if b == nil || uint(first) > uint(len(b.refs)) || uint(count) > uint(len(b.refs)-first) {
		return ErrASTDomain
	}
	refs := b.refs[first : first+count]
	m := b.manager
	for i := 0; i < len(refs); {
		if !astValidateHandles && m.lockFree() {
			// Shared decrements cannot invoke a destructor. Check readiness
			// again after the first reference needing the deletion path.
			for i < len(refs) {
				if r := refs[i]; r.slot != 0 {
					node := m.nodeAt(r.slot)
					if node.rc <= 1 {
						break
					}
					node.rc--
					refs[i] = astNodeRef{}
				}
				i++
			}
			if i == len(refs) {
				break
			}
		}
		if refs[i].slot == 0 {
			i++
			continue
		}
		consumed, err := m.DecRefConsumed(m.external(refs[i]))
		if consumed {
			refs[i] = astNodeRef{}
			i++
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ErrASTPending
		}
	}
	return nil
}
