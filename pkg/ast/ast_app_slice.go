package ast

// Bounded scratch keeps the common application hashes on the stack.
const astPreparedAppArgs = 16

// registerSliceApp registers the candidate already placed by mk_app_core.
// A canonical match has the same declaration and exact operands. It therefore
// already passed the declaration's sort checks. On a sort mismatch, the caller
// performs coercion and reports errors in source order.
func (m *ASTManager) registerSliceApp(r, decl astNodeRef, domain *astFuncDeclRecord, associative bool, input []ASTHandle) (astNodeRef, bool, error) {
	app := m.appAt(r.slot)
	var hashes [astPreparedAppArgs]uint32
	for i, h := range input {
		if h.origin != m.origin || h.slot == 0 || h.slot >= m.nextNode {
			return astNodeRef{}, false, nil
		}
		node := m.nodeAt(h.slot)
		if !node.live || node.generation != h.generation || node.kind > ASTQuantifier {
			return astNodeRef{}, false, nil
		}
		hashes[i] = node.hash
		if i < astInlineArgs {
			app.inlineArgs[i] = astNodeRef{h.slot, h.generation}
		}
	}
	inline := len(input) <= astInlineArgs
	seed := m.nodeAt(decl.slot).hash
	var hash uint32
	if inline {
		hash = astInlineHash(uint32(len(input)), seed, (*[astInlineArgs]uint32)(hashes[:]))
	} else {
		if !m.appSliceSortsMatch(domain, associative, input) {
			return astNodeRef{}, false, nil
		}
		// A wide candidate obtains its ordinary arena payload before the
		// lookup, even when the table already contains this application.
		size := uint32(len(input)) * astRefBytes
		payload, err := m.allocator.Allocate(size)
		if err != nil {
			m.freeNodeStorage(r)
			return astNodeRef{}, true, err
		}
		app.args = payload
		data, err := m.allocator.Borrow(payload)
		if err != nil || len(data) < int(size) {
			// The checked payload writer supplies the allocator error when
			// the freshly allocated bytes cannot be borrowed as a slice.
			for i, h := range input {
				if err := m.writeInternal(payload, uint32(i), astNodeRef{h.slot, h.generation}); err != nil {
					m.freeNodeStorage(r)
					return astNodeRef{}, true, err
				}
			}
		} else {
			for i, h := range input {
				putASTNodeRef(data[i*int(astRefBytes):], astNodeRef{h.slot, h.generation})
			}
		}
		hash = astHashArray(uint32(len(input)), seed, func(i uint32) uint32 { return hashes[i] })
	}
	mixed := astTableHash(hash)
	var slot uint64
	if m.table.size != 0 {
		existing, found, index, err := m.tableLookup(r, mixed)
		if err != nil {
			if inline {
				return astNodeRef{}, false, nil
			}
			m.freeNodeStorage(r)
			return astNodeRef{}, true, err
		}
		if found {
			m.nodeAt(r.slot).hash = hash
			if inline {
				m.returnNodeSlot(r.slot)
			} else {
				m.freeNodeStorage(r)
			}
			return existing, true, nil
		}
		slot = index
	}
	if inline && !m.appSliceSortsMatch(domain, associative, input) {
		return astNodeRef{}, false, nil
	}
	m.nodeAt(r.slot).hash = hash
	m.table.add(mixed, r, slot, m.table.size != 0)
	result, err := m.publishRegisteredNode(r, input)
	return result, true, err
}

func (m *ASTManager) appSliceSortsMatch(decl *astFuncDeclRecord, associative bool, input []ASTHandle) bool {
	for i, h := range input {
		pos := i
		if associative {
			pos = 0
		}
		if uint32(pos) >= decl.arity || pos >= len(decl.domainRefs) {
			return false
		}
		var sort astNodeRef
		if m.nodeAt(h.slot).kind == ASTApp {
			sort = m.appAt(h.slot).rangeSort
		}
		if sort.slot == 0 {
			var err error
			sort, err = m.sortOf(astNodeRef{h.slot, h.generation})
			if err != nil {
				return false
			}
		}
		if sort != decl.domainRefs[pos] {
			return false
		}
	}
	return true
}
