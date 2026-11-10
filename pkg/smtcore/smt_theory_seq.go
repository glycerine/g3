package smtcore

// Source: src/smt/theory_seq.h, and theory_seq.cpp up to the constructor
// (solution_map, exclusion_table). See SEQ_PORT.md.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ---- assumption and scoped_dependency_manager<assumption> ----

// seqAssumption is theory_seq::assumption: an enode pair or a literal.
// seqAssumption is theory_seq::assumption. An equality assumption has lit
// SMTNullLiteral, as assumption(n1, n2) sets null_literal; the zero
// SMTLiteral is the true literal.
type seqAssumption struct {
	n1, n2 *SMTEnode
	lit    SMTLiteral
}

// seqDep is a dependency_manager<assumption>::dependency: a leaf holding an
// assumption or a join of two dependencies. A nil *seqDep is nullptr.
type seqDep struct {
	leaf        bool
	value       seqAssumption
	left, right *seqDep
	marked      bool
}

// seqDepManager is scoped_dependency_manager<assumption>. Its region
// allocation is not observable; joins and leaves are Go values.
type seqDepManager struct {
	todo []*seqDep
}

func (dm *seqDepManager) mkLeaf(a seqAssumption) *seqDep {
	return &seqDep{leaf: true, value: a}
}

func (dm *seqDepManager) mkJoin(d1, d2 *seqDep) *seqDep {
	if d1 == nil {
		return d2
	}
	if d2 == nil {
		return d1
	}
	if d1 == d2 {
		return d1
	}
	return &seqDep{left: d1, right: d2}
}

// linearize is dependency_manager::linearize: a breadth-first walk that
// visits each node once and collects leaf values in visiting order.
func (dm *seqDepManager) linearize(d *seqDep) []seqAssumption {
	if d == nil {
		return nil
	}
	var vs []seqAssumption
	dm.todo = dm.todo[:0]
	d.marked = true
	dm.todo = append(dm.todo, d)
	for qhead := 0; qhead < len(dm.todo); qhead++ {
		d = dm.todo[qhead]
		if d.leaf {
			vs = append(vs, d.value)
			continue
		}
		for _, child := range [2]*seqDep{d.left, d.right} {
			if !child.marked {
				dm.todo = append(dm.todo, child)
				child.marked = true
			}
		}
	}
	for _, n := range dm.todo {
		n.marked = false
	}
	dm.todo = dm.todo[:0]
	return vs
}

// ---- expr_ref ownership ----

// seqRefs keeps AST references for one owner, in the order the source's
// expr_ref and expr_ref_vector members acquire them. release drops them in
// reverse order, as destructors of locals run.
type seqRefs struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
}

func (r *seqRefs) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	if h.IsNull() {
		return h, nil
	}
	ref, err := r.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r.refs = append(r.refs, ref)
	return h, nil
}

func (r *seqRefs) release() (result error) {
	for i := len(r.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.refs[i]))
	}
	r.refs = r.refs[:0]
	return result
}

// seqExprRef is an expr_ref local: assignment references the new value
// before it releases the old one.
type seqExprRef struct {
	manager *ast.ASTManager
	h       ast.ASTHandle
}

func newSeqExprRef(m *ast.ASTManager, h ast.ASTHandle) (seqExprRef, error) {
	r := seqExprRef{manager: m}
	return r, r.set(h)
}

func (r *seqExprRef) set(h ast.ASTHandle) error {
	if !h.IsNull() {
		if err := r.manager.IncRef(h); err != nil {
			return err
		}
	}
	old := r.h
	r.h = h
	if old.IsNull() {
		return nil
	}
	return r.manager.DecRef(old)
}

func (r *seqExprRef) release() error { return r.set(ast.ASTHandle{}) }

// seqExprVector is an expr_ref_vector: every element holds one reference.
type seqExprVector struct {
	manager *ast.ASTManager
	elems   []ast.ASTHandle
	refs    []ast.ASTRef
}

func newSeqExprVector(m *ast.ASTManager) seqExprVector { return seqExprVector{manager: m} }

func (v *seqExprVector) size() int                { return len(v.elems) }
func (v *seqExprVector) get(i int) ast.ASTHandle  { return v.elems[i] }
func (v *seqExprVector) handles() []ast.ASTHandle { return v.elems }
func (v *seqExprVector) back() ast.ASTHandle      { return v.elems[len(v.elems)-1] }
func (v *seqExprVector) empty() bool              { return len(v.elems) == 0 }
func (v *seqExprVector) pushBack(h ast.ASTHandle) error {
	ref, err := v.manager.NewRef(h)
	if err != nil {
		return err
	}
	v.elems = append(v.elems, h)
	v.refs = append(v.refs, ref)
	return nil
}

func (v *seqExprVector) append(hs []ast.ASTHandle) error {
	for _, h := range hs {
		if err := v.pushBack(h); err != nil {
			return err
		}
	}
	return nil
}

// shrink is expr_ref_vector::shrink/resize to a smaller size: references
// are released front to back, as ref_vector::resize does.
func (v *seqExprVector) shrink(n int) (result error) {
	if n >= len(v.elems) {
		return nil
	}
	for i := n; i < len(v.refs); i++ {
		result = errors.Join(result, v.manager.DelRef(&v.refs[i]))
	}
	v.elems = v.elems[:n]
	v.refs = v.refs[:n]
	return result
}

func (v *seqExprVector) reset() error { return v.shrink(0) }

func (v *seqExprVector) set(i int, h ast.ASTHandle) error {
	ref, err := v.manager.NewRef(h)
	if err != nil {
		return err
	}
	err = v.manager.DelRef(&v.refs[i])
	v.elems[i], v.refs[i] = h, ref
	return err
}

// ---- eval_cache and solution_map ----

type seqExprDep struct {
	expr ast.ASTHandle
	dep  *seqDep
}

// seqEvalCache is theory_seq::eval_cache.
type seqEvalCache struct {
	m     map[ast.ASTHandle]seqExprDep
	trail seqExprVector
}

func (c *seqEvalCache) find(v ast.ASTHandle) (seqExprDep, bool) {
	r, ok := c.m[v]
	return r, ok
}

func (c *seqEvalCache) insert(v ast.ASTHandle, r seqExprDep) error {
	if err := c.trail.pushBack(v); err != nil {
		return err
	}
	if err := c.trail.pushBack(r.expr); err != nil {
		return err
	}
	c.m[v] = r
	return nil
}

func (c *seqEvalCache) reset() error {
	clear(c.m)
	return c.trail.reset()
}

type seqMapUpdate uint8

const (
	seqMapIns seqMapUpdate = iota
	seqMapDel
)

// seqSolutionMap is theory_seq::solution_map: variables to representatives
// with a cache for normalization.
type seqSolutionMap struct {
	dm       *seqDepManager
	m        map[ast.ASTHandle]seqExprDep
	cache    seqEvalCache
	lhs, rhs seqExprVector
	deps     []*seqDep
	updates  []seqMapUpdate
	limit    []int
}

func newSeqSolutionMap(m *ast.ASTManager, dm *seqDepManager) seqSolutionMap {
	return seqSolutionMap{
		dm:    dm,
		m:     make(map[ast.ASTHandle]seqExprDep),
		cache: seqEvalCache{m: make(map[ast.ASTHandle]seqExprDep), trail: newSeqExprVector(m)},
		lhs:   newSeqExprVector(m), rhs: newSeqExprVector(m),
	}
}

func (s *seqSolutionMap) empty() bool { return len(s.m) == 0 }

func (s *seqSolutionMap) update(e, r ast.ASTHandle, d *seqDep) error {
	if e == r {
		return nil
	}
	if err := s.cache.reset(); err != nil {
		return err
	}
	if value, ok := s.m[e]; ok {
		if err := s.addTrail(seqMapDel, e, value.expr, value.dep); err != nil {
			return err
		}
	}
	s.m[e] = seqExprDep{r, d}
	return s.addTrail(seqMapIns, e, r, d)
}

func (s *seqSolutionMap) addTrail(op seqMapUpdate, l, r ast.ASTHandle, d *seqDep) error {
	s.updates = append(s.updates, op)
	if err := s.lhs.pushBack(l); err != nil {
		return err
	}
	if err := s.rhs.pushBack(r); err != nil {
		return err
	}
	s.deps = append(s.deps, d)
	return nil
}

func (s *seqSolutionMap) addCache(v ast.ASTHandle, r seqExprDep) error { return s.cache.insert(v, r) }
func (s *seqSolutionMap) findCache(v ast.ASTHandle) (seqExprDep, bool) { return s.cache.find(v) }
func (s *seqSolutionMap) resetCache() error                            { return s.cache.reset() }

func (s *seqSolutionMap) isRoot(e ast.ASTHandle) bool {
	_, ok := s.m[e]
	return !ok
}

func (s *seqSolutionMap) findRec(e ast.ASTHandle, finds *[]seqExprDep) {
	var d *seqDep
	value := seqExprDep{e, d}
	for {
		e = value.expr
		d = s.dm.mkJoin(d, value.dep)
		*finds = append(*finds, value)
		var ok bool
		if value, ok = s.m[e]; !ok {
			return
		}
	}
}

func (s *seqSolutionMap) find1(e ast.ASTHandle, r *ast.ASTHandle, d **seqDep) bool {
	if value, ok := s.m[e]; ok {
		*d = s.dm.mkJoin(*d, value.dep)
		*r = value.expr
		return true
	}
	return false
}

func (s *seqSolutionMap) findDep(e ast.ASTHandle, d **seqDep) ast.ASTHandle {
	*d = nil
	result := e
	for {
		value, ok := s.m[result]
		if !ok {
			return result
		}
		*d = s.dm.mkJoin(*d, value.dep)
		result = value.expr
	}
}

func (s *seqSolutionMap) find(e ast.ASTHandle) ast.ASTHandle {
	for {
		value, ok := s.m[e]
		if !ok {
			return e
		}
		e = value.expr
	}
}

func (s *seqSolutionMap) pushScope() { s.limit = append(s.limit, len(s.updates)) }

func (s *seqSolutionMap) popScope(numScopes int) error {
	if numScopes == 0 {
		return nil
	}
	if err := s.cache.reset(); err != nil {
		return err
	}
	start := s.limit[len(s.limit)-numScopes]
	for i := len(s.updates) - 1; i >= start; i-- {
		if s.updates[i] == seqMapIns {
			delete(s.m, s.lhs.get(i))
		} else {
			s.m[s.lhs.get(i)] = seqExprDep{s.rhs.get(i), s.deps[i]}
		}
	}
	s.updates = s.updates[:start]
	err := errors.Join(s.lhs.shrink(start), s.rhs.shrink(start))
	s.deps = s.deps[:start]
	s.limit = s.limit[:len(s.limit)-numScopes]
	return err
}

// ---- exclusion_table ----

type seqExprPair struct{ a, b ast.ASTHandle }

// seqExclusionTable is theory_seq::exclusion_table, the asserted
// disequalities.
type seqExclusionTable struct {
	manager  *ast.ASTManager
	table    map[seqExprPair]struct{}
	lhs, rhs seqExprVector
	limit    []int
}

func (x *seqExclusionTable) ordered(e, r ast.ASTHandle) (seqExprPair, error) {
	ie, err := x.manager.GetASTID(e)
	if err != nil {
		return seqExprPair{}, err
	}
	ir, err := x.manager.GetASTID(r)
	if err != nil {
		return seqExprPair{}, err
	}
	if ie > ir {
		e, r = r, e
	}
	return seqExprPair{e, r}, nil
}

func (x *seqExclusionTable) empty() bool { return len(x.table) == 0 }

func (x *seqExclusionTable) contains(e, r ast.ASTHandle) (bool, error) {
	p, err := x.ordered(e, r)
	if err != nil {
		return false, err
	}
	_, ok := x.table[p]
	return ok, nil
}

func (x *seqExclusionTable) update(e, r ast.ASTHandle) error {
	p, err := x.ordered(e, r)
	if err != nil {
		return err
	}
	if _, ok := x.table[p]; p.a != p.b && !ok {
		if err = x.lhs.pushBack(p.a); err != nil {
			return err
		}
		if err = x.rhs.pushBack(p.b); err != nil {
			return err
		}
		x.table[p] = struct{}{}
	}
	return nil
}

func (x *seqExclusionTable) pushScope() { x.limit = append(x.limit, x.lhs.size()) }

func (x *seqExclusionTable) popScope(numScopes int) error {
	if numScopes == 0 {
		return nil
	}
	start := x.limit[len(x.limit)-numScopes]
	for i := start; i < x.lhs.size(); i++ {
		delete(x.table, seqExprPair{x.lhs.get(i), x.rhs.get(i)})
	}
	err := errors.Join(x.lhs.shrink(start), x.rhs.shrink(start))
	x.limit = x.limit[:len(x.limit)-numScopes]
	return err
}

// ---- eq, ne, nc ----

// seqEq is theory_seq::eq. Its ls and rs are expr_ref_vectors.
type seqEq struct {
	id     uint32
	ls, rs seqExprVector
	dep    *seqDep
}

// seqNe is theory_seq::ne.
type seqNe struct {
	l, r     ast.ASTHandle
	lhs, rhs []seqExprVector
	lits     []SMTLiteral
	dep      *seqDep
	refs     seqRefs // m_l and m_r
}

// seqNc is theory_seq::nc.
type seqNc struct {
	contains ast.ASTHandle
	dep      *seqDep
	refs     seqRefs // m_contains
}

// copy constructors: each copy holds its own references, as the source's
// expr_ref members do.

func (t *SMTTheorySeq) copyEq(e *seqEq) (seqEq, error) {
	r := seqEq{id: e.id, ls: newSeqExprVector(t.m), rs: newSeqExprVector(t.m), dep: e.dep}
	if err := r.ls.append(e.ls.handles()); err != nil {
		return r, err
	}
	return r, r.rs.append(e.rs.handles())
}

func (t *SMTTheorySeq) destroyEq(e *seqEq) error {
	return errors.Join(e.rs.reset(), e.ls.reset())
}

func (t *SMTTheorySeq) copyNe(n *seqNe) (seqNe, error) {
	r := seqNe{dep: n.dep, refs: seqRefs{manager: t.m}}
	var err error
	if r.l, err = r.refs.hold(n.l); err != nil {
		return r, err
	}
	if r.r, err = r.refs.hold(n.r); err != nil {
		return r, err
	}
	for i := range n.lhs {
		v := newSeqExprVector(t.m)
		if err = v.append(n.lhs[i].handles()); err != nil {
			return r, err
		}
		r.lhs = append(r.lhs, v)
	}
	for i := range n.rhs {
		v := newSeqExprVector(t.m)
		if err = v.append(n.rhs[i].handles()); err != nil {
			return r, err
		}
		r.rhs = append(r.rhs, v)
	}
	r.lits = append([]SMTLiteral(nil), n.lits...)
	return r, nil
}

func (t *SMTTheorySeq) destroyNe(n *seqNe) (result error) {
	for i := len(n.rhs) - 1; i >= 0; i-- {
		result = errors.Join(result, n.rhs[i].reset())
	}
	for i := len(n.lhs) - 1; i >= 0; i-- {
		result = errors.Join(result, n.lhs[i].reset())
	}
	return errors.Join(result, n.refs.release())
}

func (t *SMTTheorySeq) copyNc(n *seqNc) (seqNc, error) {
	r := seqNc{dep: n.dep, refs: seqRefs{manager: t.m}}
	var err error
	r.contains, err = r.refs.hold(n.contains)
	return r, err
}

func (t *SMTTheorySeq) destroyNc(n *seqNc) error { return n.refs.release() }

// ---- scoped_vector<eq>, scoped_vector<ne>, scoped_vector<nc> ----
//
// Three concrete ports of util/scoped_vector.h. The element vector owns
// copies; a dropped element is destroyed like the source's vector<T>
// shrink and pop_back destroy theirs.

type seqScopedIndex struct {
	size       int
	elemsStart int
	sizes      []int
	elemsLim   []int
	index      []int
	src, dst   []int
	srcLim     []int
}

func (v *seqScopedIndex) pushScopeIndex(elems int) {
	v.elemsStart = elems
	v.sizes = append(v.sizes, v.size)
	v.srcLim = append(v.srcLim, len(v.src))
	v.elemsLim = append(v.elemsLim, v.elemsStart)
}

// popScopeIndex returns the element count to shrink to.
func (v *seqScopedIndex) popScopeIndex(numScopes int) int {
	newSize := len(v.sizes) - numScopes
	srcLim := v.srcLim[newSize]
	for i := len(v.src); i > srcLim; {
		i--
		v.index[v.src[i]] = v.dst[i]
	}
	v.src = v.src[:srcLim]
	v.dst = v.dst[:srcLim]
	v.srcLim = v.srcLim[:newSize]
	keep := v.elemsLim[newSize]
	v.elemsLim = v.elemsLim[:newSize]
	v.elemsStart = keep
	v.size = v.sizes[newSize]
	v.sizes = v.sizes[:newSize]
	return keep
}

func (v *seqScopedIndex) setIndex(src, dst int) {
	for src >= len(v.index) {
		v.index = append(v.index, 0)
	}
	if src < v.elemsStart {
		v.src = append(v.src, src)
		v.dst = append(v.dst, v.index[src])
	}
	v.index[src] = dst
}

type seqEqVector struct {
	seqScopedIndex
	th    *SMTTheorySeq
	elems []seqEq
}

func (v *seqEqVector) len() int        { return v.size }
func (v *seqEqVector) at(i int) *seqEq { return &v.elems[v.index[i]] }
func (v *seqEqVector) pushScope()      { v.pushScopeIndex(len(v.elems)) }
func (v *seqEqVector) popScope(n int) (result error) {
	if n == 0 {
		return nil
	}
	keep := v.popScopeIndex(n)
	for i := len(v.elems) - 1; i >= keep; i-- {
		result = errors.Join(result, v.th.destroyEq(&v.elems[i]))
	}
	v.elems = v.elems[:keep]
	return result
}

// pushBack takes ownership of e.
// destroy is ~scoped_vector: m_elems destroys every stored element, front
// to back.
func (v *seqEqVector) destroy() (result error) {
	for i := range v.elems {
		result = errors.Join(result, v.th.destroyEq(&v.elems[i]))
	}
	v.elems, v.size = nil, 0
	return result
}

func (v *seqEqVector) pushBack(e seqEq) {
	v.setIndex(v.size, len(v.elems))
	v.elems = append(v.elems, e)
	v.size++
}

// set takes ownership of e.
func (v *seqEqVector) set(idx int, e seqEq) error {
	n := v.index[idx]
	if n >= v.elemsStart {
		old := v.elems[n]
		v.elems[n] = e
		return v.th.destroyEq(&old)
	}
	v.setIndex(idx, len(v.elems))
	v.elems = append(v.elems, e)
	return nil
}

func (v *seqEqVector) popBack() error {
	var err error
	if v.index[v.size-1] == len(v.elems)-1 && len(v.elems) > v.elemsStart {
		err = v.th.destroyEq(&v.elems[len(v.elems)-1])
		v.elems = v.elems[:len(v.elems)-1]
	}
	v.size--
	return err
}

func (v *seqEqVector) eraseAndSwap(i int) error {
	if i+1 < v.size {
		e, err := v.th.copyEq(&v.elems[v.index[v.size-1]])
		if err != nil {
			return err
		}
		if err = v.set(i, e); err != nil {
			return err
		}
	}
	return v.popBack()
}

type seqNeVector struct {
	seqScopedIndex
	th    *SMTTheorySeq
	elems []seqNe
}

func (v *seqNeVector) len() int        { return v.size }
func (v *seqNeVector) at(i int) *seqNe { return &v.elems[v.index[i]] }
func (v *seqNeVector) pushScope()      { v.pushScopeIndex(len(v.elems)) }
func (v *seqNeVector) popScope(n int) (result error) {
	if n == 0 {
		return nil
	}
	keep := v.popScopeIndex(n)
	for i := len(v.elems) - 1; i >= keep; i-- {
		result = errors.Join(result, v.th.destroyNe(&v.elems[i]))
	}
	v.elems = v.elems[:keep]
	return result
}

// destroy is ~scoped_vector: m_elems destroys every stored element, front
// to back.
func (v *seqNeVector) destroy() (result error) {
	for i := range v.elems {
		result = errors.Join(result, v.th.destroyNe(&v.elems[i]))
	}
	v.elems, v.size = nil, 0
	return result
}

func (v *seqNeVector) pushBack(e seqNe) {
	v.setIndex(v.size, len(v.elems))
	v.elems = append(v.elems, e)
	v.size++
}

func (v *seqNeVector) set(idx int, e seqNe) error {
	n := v.index[idx]
	if n >= v.elemsStart {
		old := v.elems[n]
		v.elems[n] = e
		return v.th.destroyNe(&old)
	}
	v.setIndex(idx, len(v.elems))
	v.elems = append(v.elems, e)
	return nil
}

func (v *seqNeVector) popBack() error {
	var err error
	if v.index[v.size-1] == len(v.elems)-1 && len(v.elems) > v.elemsStart {
		err = v.th.destroyNe(&v.elems[len(v.elems)-1])
		v.elems = v.elems[:len(v.elems)-1]
	}
	v.size--
	return err
}

func (v *seqNeVector) eraseAndSwap(i int) error {
	if i+1 < v.size {
		e, err := v.th.copyNe(&v.elems[v.index[v.size-1]])
		if err != nil {
			return err
		}
		if err = v.set(i, e); err != nil {
			return err
		}
	}
	return v.popBack()
}

type seqNcVector struct {
	seqScopedIndex
	th    *SMTTheorySeq
	elems []seqNc
}

func (v *seqNcVector) len() int        { return v.size }
func (v *seqNcVector) at(i int) *seqNc { return &v.elems[v.index[i]] }
func (v *seqNcVector) pushScope()      { v.pushScopeIndex(len(v.elems)) }
func (v *seqNcVector) popScope(n int) (result error) {
	if n == 0 {
		return nil
	}
	keep := v.popScopeIndex(n)
	for i := len(v.elems) - 1; i >= keep; i-- {
		result = errors.Join(result, v.th.destroyNc(&v.elems[i]))
	}
	v.elems = v.elems[:keep]
	return result
}

// destroy is ~scoped_vector: m_elems destroys every stored element, front
// to back.
func (v *seqNcVector) destroy() (result error) {
	for i := range v.elems {
		result = errors.Join(result, v.th.destroyNc(&v.elems[i]))
	}
	v.elems, v.size = nil, 0
	return result
}

func (v *seqNcVector) pushBack(e seqNc) {
	v.setIndex(v.size, len(v.elems))
	v.elems = append(v.elems, e)
	v.size++
}

func (v *seqNcVector) set(idx int, e seqNc) error {
	n := v.index[idx]
	if n >= v.elemsStart {
		old := v.elems[n]
		v.elems[n] = e
		return v.th.destroyNc(&old)
	}
	v.setIndex(idx, len(v.elems))
	v.elems = append(v.elems, e)
	return nil
}

func (v *seqNcVector) popBack() error {
	var err error
	if v.index[v.size-1] == len(v.elems)-1 && len(v.elems) > v.elemsStart {
		err = v.th.destroyNc(&v.elems[len(v.elems)-1])
		v.elems = v.elems[:len(v.elems)-1]
	}
	v.size--
	return err
}

func (v *seqNcVector) eraseAndSwap(i int) error {
	if i+1 < v.size {
		e, err := v.th.copyNc(&v.elems[v.index[v.size-1]])
		if err != nil {
			return err
		}
		if err = v.set(i, e); err != nil {
			return err
		}
	}
	return v.popBack()
}

// ---- replay actions and trail ----

// seqApplyKind selects a theory_seq::apply subclass.
type seqApplyKind uint8

const (
	seqReplayLengthCoherence seqApplyKind = iota
	seqReplayFixedLength
	seqReplayAxiom
)

// seqApply is a replay_length_coherence, replay_fixed_length or
// replay_axiom: one expr_ref and the action it replays.
type seqApply struct {
	kind seqApplyKind
	e    ast.ASTHandle
	ref  ast.ASTRef
}

// seqTrail is one trail<theory_seq> entry: its undo action.
type seqTrail func(t *SMTTheorySeq) error

// ---- stats ----

type seqStats struct {
	numSplits            uint32
	numReductions        uint32
	propagateAutomata    uint32
	checkLengthCoherence uint32
	branchVariable       uint32
	solveNqs             uint32
	solveEqs             uint32
	addAxiom             uint32
	extensionality       uint32
	fixedLength          uint32
	propagateContains    uint32
	intString            uint32
}

// ---- cell ----

// seqCell is theory_seq::cell, used by explain_eq.
type seqCell struct {
	parent *seqCell
	expr   ast.ASTHandle
	dep    *seqDep
	last   int
}

// ---- theory_seq ----

// SMTTheorySeq is theory_seq.
type SMTTheorySeq struct {
	SMTTheoryBase

	m              *ast.ASTManager
	dm             seqDepManager
	rep            seqSolutionMap // unification representative
	resetCacheFlag bool           // m_reset_cache: invalidate cache
	eqs            seqEqVector    // set of current equations
	nqs            seqNeVector    // set of current disequalities
	ncs            seqNcVector    // set of non-contains constraints
	eqID           uint32

	exclude    seqExclusionTable // set of asserted disequalities
	axioms     seqExprVector     // list of axioms to add
	axiomSet   map[ast.ASTHandle]struct{}
	axiomsHead int  // index of first axiom to add
	incomplete bool // is the solver (clearly) incomplete for the fragment
	intString  seqExprVector
	itosAxioms map[string]struct{} // rational_set, keyed by RatString
	stoiAxioms map[string]struct{}
	length     seqObjHashtable // is length applied
	replay     []seqApply      // set of actions to replay

	rewrite    *seqRewriterOwner
	seqRewrite *rewriter.SequenceRewriter
	trailStack []seqTrail
	trailLim   []int
	stats      seqStats

	prefix, suffix, accept, reject                                usymbol.Symbol
	tail, nth, seqFirst, seqLast, indexofLeft, indexofRight, step usymbol.Symbol
	pre, post, eqSym                                              usymbol.Symbol

	todo        []ast.ASTHandle
	ls, rs      seqExprVector
	lhsV, rhsV  seqExprVector // m_lhs, m_rhs
	expandTodo  []ast.ASTHandle
	allCells    []*seqCell
	branchStart map[uint32]int

	// maintain automata with regular expressions
	automata     []*rewriter.StringRegexAutomaton
	exprSolver   *smtStringExpressionSolver // seq_expr_solver of m_mk_aut
	setupSkipped bool                       // setup chose a profile without theory_seq (unused placeholder)
	re2aut       map[ast.ASTHandle]*rewriter.StringRegexAutomaton
	res          seqExprVector

	// queue of asserted atoms
	atoms          []ast.ASTHandle
	atomsLim       []int
	atomsQhead     int
	newSolution    bool // new solution added
	newPropagation bool // new propagation to core

	fixed seqObjHashtable // string variables that are fixed length

	uf      seqUnionFind // m_find
	factory *protomodel.SequenceFactory
	tmps    seqRefs // expr_ref temporaries released when an entry point returns
	closed  bool
}

// seqRewriterOwner is the source's m_rewrite member: a th_rewriter with
// default parameters, owned by the theory.
type seqRewriterOwner struct {
	context  *uparams.ParamsContext
	catalog  *uparams.GlobalParamsCatalog
	globals  *uparams.GlobalParamsService
	rewriter *rewriter.TheoryRewriter
}

func newSeqRewriterOwner(m *ast.ASTManager) (r *seqRewriterOwner, result error) {
	r = &seqRewriterOwner{}
	defer func() {
		if result != nil {
			result = errors.Join(result, r.close())
			r = nil
		}
	}()
	var err error
	if r.context, err = uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()}); err != nil {
		return r, err
	}
	if r.catalog, err = uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()}); err != nil {
		return r, err
	}
	if r.globals, err = uparams.NewGlobalParamsService(m.Symbols, r.context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: r.catalog}); err != nil {
		return r, err
	}
	if err = r.globals.InitializeSource(); err != nil {
		return r, err
	}
	r.rewriter, err = rewriter.NewTheoryRewriter(m, r.context, r.globals, nil)
	return r, err
}

func (r *seqRewriterOwner) rewriterFor(*ast.ASTManager) (*rewriter.TheoryRewriter, error) {
	if r == nil || r.rewriter == nil {
		return nil, ast.ErrASTHandle
	}
	return r.rewriter, nil
}

func (r *seqRewriterOwner) close() (result error) {
	if r.rewriter != nil {
		result = errors.Join(result, r.rewriter.Close())
	}
	if r.globals != nil {
		result = errors.Join(result, r.globals.Close())
	}
	if r.catalog != nil {
		result = errors.Join(result, r.catalog.Close())
	}
	if r.context != nil {
		result = errors.Join(result, r.context.Close())
	}
	return result
}

// keep big referenced for rational_set keys
var _ = big.NewRat
