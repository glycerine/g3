package ast

// Source AST scalar/indexed readers. All public borrows remain handle values.

import (
	"math"
	"slices"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) declIdentity(r astNodeRef) (FamilyID, int32, error) {
	if e := m.checkNode(r); e != nil {
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
		if e := m.checkNode(r); e != nil {
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
	r := astNodeRef{h.slot, h.generation}
	if !m.fastHandle(h) {
		var e error
		if r, e = m.readNodeChecked(h); e != nil {
			return ASTNodeHeader{Family: -1, DeclKind: -1}, e
		}
	}
	n := m.nodeAt(r.slot)
	if n.kind != ASTApp {
		return ASTNodeHeader{Kind: n.kind, RefCount: n.rc, Family: -1, DeclKind: -1}, nil
	}
	v := m.appAt(r.slot)
	if v.declCached && !ASTSourceDebug {
		return m.cachedAppHeader(n, v), nil
	}
	family, declKind, e := m.declIdentity(v.decl)
	return ASTNodeHeader{Kind: ASTApp, RefCount: n.rc, NumArgs: v.count, Family: family, DeclKind: declKind, Decl: m.external(v.decl)}, e
}

func (m *ASTManager) cachedAppHeader(n *astNodeRecord, v *astAppRecord) ASTNodeHeader {
	return ASTNodeHeader{Kind: ASTApp, RefCount: n.rc, NumArgs: v.count, Family: v.declFamily, DeclKind: v.declKind, Decl: m.external(v.decl)}
}

// NodeShape is NodeHeader's Kind, RefCount and NumArgs alone, in one read:
// what traversals need per visited node, without the declaration identity.
func (m *ASTManager) NodeShape(h ASTHandle) (kind ASTKind, refCount, numArgs uint32, err error) {
	r := astNodeRef{h.slot, h.generation}
	if !m.fastHandle(h) {
		var e error
		if r, e = m.readNodeChecked(h); e != nil {
			return 0, 0, 0, e
		}
	}
	n := m.nodeAt(r.slot)
	if n.kind == ASTApp {
		return n.kind, n.rc, m.appAt(r.slot).count, nil
	}
	return n.kind, n.rc, 0, nil
}

// AppIdentity is GetFamilyID and GetDeclKind of an application in one read;
// for any other node it reports isApp false and (-1, -1).
func (m *ASTManager) AppIdentity(h ASTHandle) (isApp bool, family FamilyID, declKind int32, err error) {
	r := astNodeRef{h.slot, h.generation}
	if !m.fastHandle(h) {
		var e error
		if r, e = m.readNodeChecked(h); e != nil {
			return false, -1, -1, e
		}
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, -1, -1, nil
	}
	if a := m.appAt(r.slot); a.declCached && !ASTSourceDebug {
		return true, a.declFamily, a.declKind, nil
	}
	family, declKind, err = m.declIdentity(m.appAt(r.slot).decl)
	return true, family, declKind, err
}

func (m *ASTManager) GetKind(h ASTHandle) (ASTKind, error) {
	if m.fastHandle(h) {
		return m.nodeAt(h.slot).kind, nil
	}
	r, e := m.readNodeChecked(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).kind, nil
}
func (m *ASTManager) GetASTID(h ASTHandle) (uint32, error) {
	r, e := m.readNode(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).id, nil
}
func (m *ASTManager) GetHash(h ASTHandle) (uint32, error) {
	r, e := m.readNode(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).hash, nil
}
func (m *ASTManager) GetRefCount(h ASTHandle) (uint32, error) {
	if m.fastHandle(h) {
		return m.nodeAt(h.slot).rc, nil
	}
	r, e := m.readNodeChecked(h)
	if e != nil {
		return 0, e
	}
	return m.nodeAt(r.slot).rc, nil
}
func (m *ASTManager) GetSort(h ASTHandle) (ASTHandle, error) {
	r, e := m.readNode(h)
	if e != nil {
		return ASTHandle{}, e
	}
	r, e = m.sortOf(r)
	return m.external(r), e
}
func (m *ASTManager) GetVarIndex(h ASTHandle) (uint32, error) {
	r, e := m.readNode(h)
	if e != nil {
		return 0, e
	}
	if m.nodeAt(r.slot).kind != ASTVar {
		return 0, ErrASTDomain
	}
	return m.varAt(r.slot).index, nil
}
func (m *ASTManager) GetArg(h ASTHandle, index uint32) (ASTHandle, error) {
	r := astNodeRef{h.slot, h.generation}
	if !m.fastHandle(h) {
		var e error
		if r, e = m.readNodeChecked(h); e != nil {
			return ASTHandle{}, e
		}
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return ASTHandle{}, ErrASTDomain
	}
	v := m.appAt(r.slot)
	if index >= v.count {
		return ASTHandle{}, ErrASTDomain
	}
	if index < astInlineArgs && !ASTSourceDebug { // appArg's inline arguments
		return m.external(v.inlineArgs[index]), nil
	}
	child, e := m.appArg(v, index)
	return m.external(child), e
}
func (m *ASTManager) GetFamilyID(h ASTHandle) (FamilyID, error) {
	r := astNodeRef{h.slot, h.generation}
	if !m.fastHandle(h) {
		var e error
		if r, e = m.readNodeChecked(h); e != nil {
			return -1, e
		}
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		a := m.appAt(r.slot)
		if a.declCached && !ASTSourceDebug { // appIdentity's cache of declIdentity(decl)
			return a.declFamily, nil
		}
		r = a.decl
	}
	fid, _, e := m.declIdentity(r)
	return fid, e
}
func (m *ASTManager) GetDeclKind(h ASTHandle) (int32, error) {
	r := astNodeRef{h.slot, h.generation}
	if !m.fastHandle(h) {
		var e error
		if r, e = m.readNodeChecked(h); e != nil {
			return -1, e
		}
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		a := m.appAt(r.slot)
		if a.declCached && !ASTSourceDebug { // appIdentity's cache of declIdentity(decl)
			return a.declKind, nil
		}
		r = a.decl
	}
	_, kind, e := m.declIdentity(r)
	return kind, e
}
func (m *ASTManager) IsSkolemDecl(h ASTHandle) (bool, error) {
	r, e := m.readKind(h, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	flags, e := m.funcFlags(r)
	return flags.Skolem, e
}

// GetFuncDeclFlags returns the source func_decl_info flags as a plain value.
func (m *ASTManager) GetFuncDeclFlags(h ASTHandle) (ASTFuncFlags, error) {
	r, e := m.readKind(h, ASTFuncDecl)
	if e != nil {
		return ASTFuncFlags{}, e
	}
	return m.funcFlags(r)
}
func (m *ASTManager) GetSortSize(h ASTHandle) (SortSize, error) {
	r, e := m.readNode(h)
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
	r, e := m.readNode(h)
	if e != nil {
		return 0, e
	}
	return m.depth(r), nil
}
func (m *ASTManager) HasLabels(h ASTHandle) (bool, error) {
	r, e := m.readNode(h)
	if e != nil {
		return false, e
	}
	return m.hasLabels(r), nil
}

// lockFree reports that lock() would succeed without work: a usable,
// constructed, open manager with no trace or format callback running and no
// pending node deletions.
func (m *ASTManager) lockFree() bool {
	return m.Self == m && !m.traceExecuting && !m.formatExecuting && !m.Closed &&
		!m.sourceDTStarted && m.constructorComplete && len(m.pendingDeletions) == 0
}

// IncRefHeld is IncRef for a handle read from a live AST or held by the
// caller (rewriter result stacks). Like the read accessors, normal builds
// skip the handle validation; g3checked/g3debug builds run IncRef.
func (m *ASTManager) IncRefHeld(h ASTHandle) error {
	if astValidateHandles || h.slot == 0 || !m.lockFree() {
		return m.IncRef(h)
	}
	n := m.nodeAt(h.slot)
	if ASTSourceDebug && n.rc == math.MaxUint32 {
		return ErrASTDomain
	}
	n.rc++
	return nil
}

// ReleaseSharedAppSuffix consumes one reference to each application at the
// end of owned that will survive its decrement. It stops before any deletion,
// invalid handle, or other node kind. No destructor or trace callback runs.
func (m *ASTManager) ReleaseSharedAppSuffix(owned []ASTHandle) int {
	if astValidateHandles || m == nil || !m.lockFree() {
		return 0
	}
	for i := len(owned) - 1; i >= 0; i-- {
		h := owned[i]
		if h.origin != m.origin || h.slot == 0 || h.slot >= m.nextNode {
			return len(owned) - 1 - i
		}
		n := m.nodeAt(h.slot)
		if !n.live || n.generation != h.generation || n.kind != ASTApp || n.rc <= 1 {
			return len(owned) - 1 - i
		}
		n.rc--
	}
	return len(owned)
}

// DecRefHeldConsumed is DecRefConsumed for a handle the caller holds a
// reference to. Normal builds skip the handle validation when the node
// survives; the last reference takes DecRefConsumed's deletion path.
func (m *ASTManager) DecRefHeldConsumed(h ASTHandle) (bool, error) {
	if !astValidateHandles && h.slot != 0 && m.lockFree() {
		if n := m.nodeAt(h.slot); n.rc > 1 {
			n.rc--
			return true, nil
		}
	}
	return m.DecRefConsumed(h)
}

// ShrinkHeldRefs releases the owned suffix in first-to-last order, as an
// AST reference vector does. On error the returned slice contains the
// preserved prefix followed by the references that still need a decrement.
// All non-null handles must be held references from this manager.
func (m *ASTManager) ShrinkHeldRefs(values []ASTHandle, limit int) ([]ASTHandle, error) {
	if limit < 0 || limit > len(values) {
		return values, ErrASTDomain
	}
	for i := limit; i < len(values); {
		// Shared references cannot run a destructor. No manager callback can
		// interrupt this single-goroutine stretch or change manager readiness.
		if !astValidateHandles && m != nil && m.lockFree() {
			for i < len(values) {
				h := values[i]
				if h.slot == 0 {
					break
				}
				n := m.nodeAt(h.slot)
				if n.rc <= 1 {
					break
				}
				n.rc--
				values[i] = ASTHandle{}
				i++
			}
			if i == len(values) {
				break
			}
		}
		consumed, err := m.DecRefConsumed(values[i])
		if consumed {
			values[i] = ASTHandle{}
			i++
		}
		if err != nil || !consumed {
			remaining := append(values[:limit], values[i:]...)
			clear(values[len(remaining):])
			if err == nil {
				err = ErrASTPending
			}
			return remaining, err
		}
	}
	return values[:limit], nil
}

func (m *ASTManager) IncRef(h ASTHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	return m.incRef(h)
}

// IncRefPair acquires both references together. Equal handles acquire two
// references and null handles are ignored, as in two calls to IncRef.
func (m *ASTManager) IncRefPair(first, second ASTHandle) error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	if err := m.incRef(first); err != nil {
		return err
	}
	if err := m.incRef(second); err != nil {
		// Neither increment runs callbacks. Restore the original count even
		// when it was zero: a failed acquisition must not delete that AST.
		if !first.IsNull() {
			m.nodeAt(first.slot).rc--
		}
		return err
	}
	return nil
}

func (m *ASTManager) incRef(h ASTHandle) error {
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

// HasBoolSort reports whether the expression has this manager's Boolean sort.
func (m *ASTManager) HasBoolSort(h ASTHandle) (bool, error) {
	r := astNodeRef{h.slot, h.generation}
	if astValidateHandles || m == nil || !m.lockFree() {
		var err error
		r, err = m.readNodeChecked(h)
		if err != nil {
			return false, err
		}
	} else {
		if h.origin != m.origin || h.slot == 0 || h.slot >= m.nextNode {
			return false, ErrASTHandle
		}
		slot := &m.nodes[(h.slot-1)/astPageSize].slots[(h.slot-1)%astPageSize]
		if !slot.node.live || slot.node.generation != h.generation {
			return false, ErrASTHandle
		}
		if slot.node.kind == ASTApp && slot.app.rangeSort.slot != 0 {
			return slot.app.rangeSort == m.boolSort, nil
		}
	}
	sort, err := m.sortOf(r)
	return sort == m.boolSort && err == nil, err
}

// ExpressionHeader returns NodeHeader and the Boolean result-sort predicate.
// A handle crossing the SMT entry point is checked before indexing the arena.
func (m *ASTManager) ExpressionHeader(h ASTHandle) (ASTNodeHeader, bool, error) {
	if !astValidateHandles && m != nil && m.lockFree() {
		if h.origin != m.origin || h.slot == 0 || h.slot >= m.nextNode {
			return ASTNodeHeader{}, false, ErrASTHandle
		}
		slot := m.nodeSlotAt(h.slot)
		if !slot.node.live || slot.node.generation != h.generation {
			return ASTNodeHeader{}, false, ErrASTHandle
		}
		app := &slot.app
		if slot.node.kind == ASTApp && app.declCached && !ASTSourceDebug && app.rangeSort.slot != 0 {
			return m.cachedAppHeader(&slot.node, app), app.rangeSort == m.boolSort, nil
		}
	} else if _, err := m.readNodeChecked(h); err != nil {
		return ASTNodeHeader{}, false, err
	}
	header, err := m.NodeHeader(h)
	if err != nil {
		return header, false, err
	}
	sort, err := m.sortOf(astNodeRef{h.slot, h.generation})
	return header, err == nil && sort == m.boolSort, err
}
