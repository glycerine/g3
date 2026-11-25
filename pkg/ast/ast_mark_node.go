package ast

// Source: src/ast/ast.h:495-518.

func (m *ASTManager) nodeMark(r astNodeRef, plane uint8, owner uint64, op uint8, flag bool) (bool, error) {
	if e := m.validNode(r); e != nil {
		return false, e
	}
	n := m.nodeAt(r.slot)
	bit := n.hasFlag(astFirstMark)
	if plane == 2 {
		bit = n.hasFlag(astSecondMark)
	}
	if ASTSourceDebug {
		if old := m.nodeMarkOwner(r.slot, plane); old != 0 && old != owner {
			return false, ErrASTDomain
		}
	}
	if op == 0 {
		return bit, nil
	}
	if op == 2 {
		flag = false
		owner = 0
	}
	if plane == 1 {
		n.setFlag(astFirstMark, flag)
	} else {
		n.setFlag(astSecondMark, flag)
	}
	m.setNodeMarkOwner(r.slot, plane, owner)
	return flag, nil
}
func (m *ASTManager) publicNodeMark(h ASTHandle, plane uint8, owner uint64, op uint8, flag bool) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if h.origin != m.origin {
		return false, ErrASTHandle
	}
	return m.nodeMark(astNodeRef{h.slot, h.generation}, plane, owner, op, flag)
}
func (m *ASTManager) MarkNode1(h ASTHandle, flag bool, owner uint64) error {
	_, e := m.publicNodeMark(h, 1, owner, 1, flag)
	return e
}
func (m *ASTManager) MarkNode2(h ASTHandle, flag bool, owner uint64) error {
	_, e := m.publicNodeMark(h, 2, owner, 1, flag)
	return e
}
func (m *ASTManager) ResetNodeMark1(h ASTHandle, owner uint64) error {
	_, e := m.publicNodeMark(h, 1, owner, 2, false)
	return e
}
func (m *ASTManager) ResetNodeMark2(h ASTHandle, owner uint64) error {
	_, e := m.publicNodeMark(h, 2, owner, 2, false)
	return e
}
func (m *ASTManager) IsNodeMarked1(h ASTHandle, owner uint64) (bool, error) {
	return m.publicNodeMark(h, 1, owner, 0, false)
}
func (m *ASTManager) IsNodeMarked2(h ASTHandle, owner uint64) (bool, error) {
	return m.publicNodeMark(h, 2, owner, 0, false)
}

func (m *ASTManager) nodeMarkOwner(slot uint32, plane uint8) uint64 {
	p := m.nodes[(slot-1)/astPageSize].markOwners[plane-1]
	if p == nil {
		return 0
	}
	return p[(slot-1)%astPageSize]
}

func (m *ASTManager) setNodeMarkOwner(slot uint32, plane uint8, owner uint64) {
	p := &m.nodes[(slot-1)/astPageSize].markOwners[plane-1]
	if *p == nil {
		if owner == 0 {
			return
		}
		*p = new(astMarkOwnerPage)
	}
	(*p)[(slot-1)%astPageSize] = owner
}
