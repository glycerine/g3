package ast

// Source AST scalar/indexed readers. All public borrows remain handle values.

import (
	"math"
	"slices"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) declIdentity(r astNodeRef) (FamilyID, int32, error) {
	if e := m.validNode(r); e != nil {
		return -1, -1, e
	}
	var slot, generation uint32
	switch m.nodeAt(r.slot).kind {
	case ASTSort:
		v := m.sortAt(r.slot)
		if v.identityCached {
			return v.identityFamily, v.identityKind, nil
		}
		p := v.info
		slot, generation = p.slot, p.generation
	case ASTFuncDecl:
		d := m.declAt(r.slot)
		if d.identityCached {
			return d.identityFamily, d.identityKind, nil
		}
		p := d.info
		slot, generation = p.slot, p.generation
	default:
		return -1, -1, ErrASTDomain
	}
	if slot == 0 {
		return -1, -1, nil
	}
	info, e := m.info(slot, generation)
	if e != nil {
		return -1, -1, e
	}
	return info.family, info.kind, nil
}

// appIdentity is declIdentity(a.decl) for the record a of a live app. It
// reads the copy taken at registration; debug builds keep the checked read.
func (m *ASTManager) appIdentity(a *astAppRecord) (FamilyID, int32, error) {
	if a.declCached && !ASTSourceDebug {
		return a.declFamily, a.declKind, nil
	}
	return m.declIdentity(a.decl)
}
func (m *ASTManager) sortOf(r astNodeRef) (astNodeRef, error) {
	for {
		if e := m.validNode(r); e != nil {
			return astNodeRef{}, e
		}
		switch m.nodeAt(r.slot).kind {
		case ASTApp:
			a := m.appAt(r.slot)
			if a.rangeSort.slot != 0 && !ASTSourceDebug {
				return a.rangeSort, nil
			}
			return m.declAt(a.decl.slot).rangeSort, nil
		case ASTVar:
			return m.varAt(r.slot).sort, nil
		case ASTQuantifier:
			r = m.quantifierAt(r.slot).body
		default:
			return astNodeRef{}, ErrASTDomain
		}
	}
}

// SortHandles orders handles by slot and generation. Go randomizes map iteration,
// so loops releasing references from a map sort first: release order decides
// slot and id reuse, and with them later term orders.
func SortHandles(handles []ASTHandle) {
	slices.SortFunc(handles, func(a, b ASTHandle) int {
		if a.slot != b.slot {
			if a.slot < b.slot {
				return -1
			}
			return 1
		}
		if a.generation != b.generation {
			if a.generation < b.generation {
				return -1
			}
			return 1
		}
		return 0
	})
}

// ASTNodeHeader is what callers usually read about one node, in one validated
// access. For anything but an application NumArgs and Decl are zero and Family
// and DeclKind are -1, whatever the single accessors would report.
type ASTNodeHeader struct {
	Kind     ASTKind
	RefCount uint32
	NumArgs  uint32
	Family   FamilyID
	DeclKind int32
	Decl     ASTHandle
}

func (m *ASTManager) NodeHeader(h ASTHandle) (ASTNodeHeader, error) {
	header := ASTNodeHeader{Family: -1, DeclKind: -1}
	if e := m.lock(); e != nil {
		return header, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return header, e
	}
	n := m.nodeAt(r.slot)
	header.Kind, header.RefCount = n.kind, n.rc
	if n.kind == ASTApp {
		v := m.appAt(r.slot)
		header.NumArgs, header.Decl = v.count, m.external(v.decl)
		header.Family, header.DeclKind, e = m.appIdentity(v)
	}
	return header, e
}

func (m *ASTManager) GetKind(h ASTHandle) (ASTKind, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).kind, nil
}
func (m *ASTManager) GetASTID(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).id, nil
}
func (m *ASTManager) GetHash(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).hash, nil
}
func (m *ASTManager) GetRefCount(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).rc, nil
}
func (m *ASTManager) GetSort(h ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e = m.sortOf(r)
	return m.external(r), e
}
func (m *ASTManager) GetVarIndex(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	if m.nodeAt(r.slot).kind != ASTVar {
		return 0, ErrASTDomain
	}
	return m.varAt(r.slot).index, nil
}
func (m *ASTManager) GetArg(h ASTHandle, index uint32) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return ASTHandle{}, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return ASTHandle{}, ErrASTDomain
	}
	v := m.appAt(r.slot)
	if index >= v.count {
		return ASTHandle{}, ErrASTDomain
	}
	child, e := m.appArg(v, index)
	return m.external(child), e
}
func (m *ASTManager) GetFamilyID(h ASTHandle) (FamilyID, error) {
	if e := m.lock(); e != nil {
		return -1, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return -1, e
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		r = m.appAt(r.slot).decl
	}
	fid, _, e := m.declIdentity(r)
	return fid, e
}
func (m *ASTManager) GetDeclKind(h ASTHandle) (int32, error) {
	if e := m.lock(); e != nil {
		return -1, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return -1, e
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		r = m.appAt(r.slot).decl
	}
	_, kind, e := m.declIdentity(r)
	return kind, e
}
func (m *ASTManager) IsSkolemDecl(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	flags, e := m.funcFlags(r)
	return flags.Skolem, e
}

// GetFuncDeclFlags returns the source func_decl_info flags as a plain value.
func (m *ASTManager) GetFuncDeclFlags(h ASTHandle) (ASTFuncFlags, error) {
	if e := m.lock(); e != nil {
		return ASTFuncFlags{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(h, ASTFuncDecl)
	if e != nil {
		return ASTFuncFlags{}, e
	}
	return m.funcFlags(r)
}
func (m *ASTManager) GetSortSize(h ASTHandle) (SortSize, error) {
	if e := m.lock(); e != nil {
		return SortSize{}, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return SortSize{}, e
	}
	if m.nodeAt(r.slot).kind != ASTSort {
		return SortSize{}, ErrASTDomain
	}
	p := m.sortAt(r.slot).info
	if p == (ASTSortInfo{}) {
		return InfiniteSortSize(), nil
	}
	info, e := m.sortInfo(p)
	if e != nil {
		return SortSize{}, e
	}
	return info.size, nil
}
func (m *ASTManager) GetDepth(h ASTHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return 0, e
	}
	return m.depth(r), nil
}
func (m *ASTManager) HasLabels(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.hasLabels(r), nil
}
func (m *ASTManager) IncRef(h ASTHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if h.IsNull() {
		return nil
	}
	// internal followed by incNode, validating the node once.
	if h.origin != m.origin || h.slot == 0 || h.slot >= m.nextNode {
		return ErrASTHandle
	}
	n := m.nodeAt(h.slot)
	if !n.live || n.generation != h.generation {
		return ErrASTHandle
	}
	if ASTSourceDebug && n.rc == math.MaxUint32 {
		return ErrASTDomain
	}
	n.rc++
	return nil
}
func (m *ASTManager) DecRef(h ASTHandle) error {
	_, e := m.DecRefConsumed(h)
	return e
}

// DecRefConsumed reports the existing deletion transport's consumption flag.
// A caller must retire its ownership when deletion consumes the reference,
// including a later worklist reservation or destructor error.
func (m *ASTManager) DecRefConsumed(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if h.IsNull() {
		return true, nil
	}
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.decNode(r)
}
func (m *ASTManager) BasicFamilyID() FamilyID { return 0 }
func (m *ASTManager) IsFormatManager() bool   { return m != nil && m.Format == nil }
func (m *ASTManager) GetFormatManager() (*ASTManager, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	if m.Format == nil {
		return m, nil
	}
	return m.Format, nil
}

// BorrowSourceTraceTags returns the selected source-global/per-manager stream
// owner. Consumers borrow its identity; the manager retains no extra lifetime.
func (m *ASTManager) BorrowSourceTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

// RewriterParameterServices borrows the constructor-supplied parameter owners
// for source-default rewriters. Plain AST managers may have no such services.
func (m *ASTManager) RewriterParameterServices() (*uparams.ParamsContext, *uparams.GlobalParamsService, error) {
	if err := m.lock(); err != nil {
		return nil, nil, err
	}
	defer m.mu.Unlock()
	return m.params, m.globals, nil
}
