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
	// Arena pages stay at fixed addresses while the canonical table grows.
	var operands [astPreparedAppArgs]*astNodeSlot
	for i, h := range input {
		if h.origin != m.origin || h.slot == 0 || h.slot >= m.nextNode {
			return astNodeRef{}, false, nil
		}
		operands[i] = m.nodeSlotAt(h.slot)
		node := &operands[i].node
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
		existing, found, index, err := m.tableLookup(r, hash, mixed)
		if err != nil {
			if inline {
				return astNodeRef{}, false, nil
			}
			m.freeNodeStorage(r)
			return astNodeRef{}, true, err
		}
		if found {
			if inline {
				m.table.rememberApp(decl, input, uint32(m.table.indices[index])+1)
			}
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
	m.table.add(hash, mixed, r, slot, m.table.size != 0)
	result, err := m.publishSliceApp(r, domain, operands[:len(input)])
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
		switch m.nodeAt(h.slot).kind {
		case ASTApp:
			sort = m.appAt(h.slot).rangeSort
		case ASTVar:
			sort = m.varAt(h.slot).sort
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

// A canonical table hit enables speculative lookups for subsequent applications.
// After a run of misses, novel terms use only the canonical table again.
const (
	appHintAttempts = 32
	maxAppHints     = 1024 // at most 4 KiB of weak table positions per manager
)

func (t *astCanonicalTable) rememberApp(decl astNodeRef, args []ASTHandle, position uint32) {
	t.hintAttempts = appHintAttempts
	if t.size >= 64 && len(t.appHints) < maxAppHints && uint32(len(t.appHints)) < t.size {
		// A table rebuild can invalidate every position. It is always safe to
		// start with empty buckets and repopulate them on later canonical hits.
		size := 64
		for uint32(size) < t.size && size < maxAppHints {
			size *= 2
		}
		t.appHints = make([]uint32, size)
	}
	*t.inlineAppHint(decl, args) = position
}

// Choose a bucket using only inexpensive slot values. hintedApp checks the
// declaration, arity and all operands before accepting its candidate.
func (t *astCanonicalTable) inlineAppHint(decl astNodeRef, args []ASTHandle) *uint32 {
	if len(t.appHints) == 0 {
		return &t.smallAppHint
	}
	hash := decl.slot*0x9e3779b1 ^ uint32(len(args))
	if len(args) > 0 {
		hash += args[0].slot
	}
	if len(args) > 1 {
		hash = hash*0x85ebca77 + args[len(args)-1].slot
	}
	return &t.appHints[hash&uint32(len(t.appHints)-1)]
}

func (m *ASTManager) hintedApp(decl astNodeRef, args []ASTHandle, hint uint32) astNodeRef {
	if len(args) > astInlineArgs || hint == 0 || uint64(hint) > uint64(len(m.table.entries)) {
		return astNodeRef{}
	}
	ref := m.table.entries[hint-1].ref
	if ref.slot == 0 || ref.slot >= m.nextNode {
		return astNodeRef{}
	}
	slot := m.nodeSlotAt(ref.slot)
	if !slot.node.live || slot.node.generation != ref.generation || slot.node.kind != ASTApp {
		return astNodeRef{}
	}
	app := &slot.app
	if app.decl != decl || app.count != uint32(len(args)) {
		return astNodeRef{}
	}
	for i, arg := range args {
		if arg.origin != m.origin || app.inlineArgs[i] != (astNodeRef{arg.slot, arg.generation}) {
			return astNodeRef{}
		}
	}
	return ref
}

// The slice path has validated the declaration and every operand. Capture
// the same references and derived application flags while those slots are hot.
func (m *ASTManager) publishSliceApp(r astNodeRef, decl *astFuncDeclRecord, operands []*astNodeSlot) (astNodeRef, error) {
	if !decl.identityCached && decl.info != (ASTFuncDeclInfo{}) {
		return m.publishRegisteredNode(r)
	}
	if err := m.assignRegisteredID(r); err != nil {
		return astNodeRef{}, err
	}
	a := m.appAt(r.slot)
	a.rangeSort = decl.rangeSort
	a.declFamily, a.declKind = NullFamilyID, -1
	if decl.identityCached {
		a.declFamily, a.declKind = decl.identityFamily, decl.identityKind
	}
	a.declCached = true
	if len(operands) > 0 {
		a.subtree = appDeclSubtreeFlags(a.declFamily, a.declKind)
	}
	a.ground = true
	m.nodeAt(a.decl.slot).rc++
	for _, child := range operands {
		child.node.rc++
		m.updateAppFlags(a, child)
	}
	a.depth = min(a.depth+1, 65535)
	return r, nil
}
