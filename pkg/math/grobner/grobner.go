package grobner

// Package grobner implements the retained, unindexed Grobner basis engine from
// src/math/grobner/grobner.{h,cpp}.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

const (
	arithNumeral = int32(0)
	arithAdd     = int32(6)
	arithMul     = int32(9)
)

// Stats is the source grobner_stats record.
type Stats struct {
	Simplify, Superpose, ComputeBasis, NumProcessed int64
}

func (s *Stats) Reset() { *s = Stats{} }

// Monomial owns references to its sorted variables until it is consumed by an
// assertion or released with DelMonomial.
//
// coeff is a rational of the AST manager's rational manager
// (NumeralService().Manager); the monomial owns it and frees it with its
// variables.
type Monomial struct {
	coeff numeral.Rational
	vars  []ast.ASTHandle
	owner *Solver
	live  bool
}

// Coefficient is get_coeff(): the monomial's own rational, borrowed until
// the monomial is deleted.
func (m *Monomial) Coefficient() *numeral.Rational { return &m.coeff }
func (m *Monomial) Degree() uint32                 { return uint32(len(m.vars)) }
func (m *Monomial) Size() uint32                   { return uint32(len(m.vars)) }
func (m *Monomial) Variable(i uint32) ast.ASTHandle {
	if m == nil || i >= uint32(len(m.vars)) {
		return ast.ASTHandle{}
	}
	return m.vars[i]
}
func (m *Monomial) Variables() []ast.ASTHandle {
	if m == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), m.vars...)
}

// Equation is borrowed from Solver and remains valid until the solver deletes
// it during a later basis computation, pop, reset, or close.
type Equation struct {
	scope      uint32
	index      uint32
	linear     bool
	monomials  []*Monomial
	dependency *Dependency
	owner      *Solver
	live       bool
}

func (e *Equation) NumMonomials() uint32 { return uint32(len(e.monomials)) }
func (e *Equation) Monomial(i uint32) *Monomial {
	if e == nil || i >= uint32(len(e.monomials)) {
		return nil
	}
	return e.monomials[i]
}
func (e *Equation) Monomials() []*Monomial {
	if e == nil {
		return nil
	}
	return append([]*Monomial(nil), e.monomials...)
}
func (e *Equation) Dependency() *Dependency   { return e.dependency }
func (e *Equation) IsLinearCombination() bool { return e.linear }

// equationSet is grobner::equation_set, an obj_hashtable<equation> whose
// hash is the equation's m_bidx (src/math/grobner/grobner.h:73,120-122). It
// is a concrete port of core_hashtable's open addressing
// (src/util/hashtable.h): capacity 8, linear probing, deleted slots reused
// on insert, growth when (size+deleted)*4 > capacity*3, and iteration in
// slot order.
type equationSet struct {
	table      []*Equation
	size       int
	numDeleted int
}

// deletedEquation marks a deleted slot, as obj_hash_entry uses pointer 1.
var deletedEquation = &Equation{}

const (
	equationSetInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	equationSetSmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

func equationSlotUsed(e *Equation) bool { return e != nil && e != deletedEquation }

func (s *equationSet) ensureTable() {
	if s.table == nil {
		s.table = make([]*Equation, equationSetInitialCapacity)
	}
}

// moveEquationTable is core_hashtable::move_table.
func moveEquationTable(source, target []*Equation) {
	mask := uint32(len(target) - 1)
	for _, e := range source {
		if !equationSlotUsed(e) {
			continue
		}
		begin := int(e.index & mask)
		placed := false
		for i := begin; i < len(target); i++ {
			if target[i] == nil {
				target[i] = e
				placed = true
				break
			}
		}
		for i := 0; !placed && i < begin; i++ {
			if target[i] == nil {
				target[i] = e
				placed = true
			}
		}
	}
}

func (s *equationSet) expandTable() {
	table := make([]*Equation, len(s.table)<<1)
	moveEquationTable(s.table, table)
	s.table = table
	s.numDeleted = 0
}

func (s *equationSet) removeDeletedEntries() {
	table := make([]*Equation, len(s.table))
	moveEquationTable(s.table, table)
	s.table = table
	s.numDeleted = 0
}

// insert is core_hashtable::insert.
func (s *equationSet) insert(e *Equation) {
	if e == nil {
		return
	}
	s.ensureTable()
	if (s.size+s.numDeleted)<<2 > len(s.table)*3 {
		s.expandTable()
	}
	begin := int(e.index & uint32(len(s.table)-1))
	delEntry := -1
	visit := func(i int) bool {
		switch curr := s.table[i]; {
		case curr == e:
			return true
		case curr == nil:
			if delEntry >= 0 {
				i = delEntry
				s.numDeleted--
			}
			s.table[i] = e
			s.size++
			return true
		case curr == deletedEquation:
			delEntry = i
		}
		return false
	}
	for i := begin; i < len(s.table); i++ {
		if visit(i) {
			return
		}
	}
	for i := 0; i < begin; i++ {
		if visit(i) {
			return
		}
	}
	panic("grobner: equation set has no free slot")
}

// erase is core_hashtable::remove.
func (s *equationSet) erase(e *Equation) {
	if e == nil || s.table == nil {
		return
	}
	begin := int(e.index & uint32(len(s.table)-1))
	found := -1
	for i := begin; found < 0 && i < len(s.table); i++ {
		if s.table[i] == e {
			found = i
		} else if s.table[i] == nil {
			return
		}
	}
	for i := 0; found < 0 && i < begin; i++ {
		if s.table[i] == e {
			found = i
		} else if s.table[i] == nil {
			return
		}
	}
	if found < 0 {
		return
	}
	next := found + 1
	if next == len(s.table) {
		next = 0
	}
	if s.table[next] == nil {
		s.table[found] = nil
		s.size--
		return
	}
	s.table[found] = deletedEquation
	s.numDeleted++
	s.size--
	if s.numDeleted > s.size && s.numDeleted > equationSetSmallCapacity {
		s.removeDeletedEntries()
	}
}

// reset is core_hashtable::reset.
func (s *equationSet) reset() {
	if s.size == 0 && s.numDeleted == 0 {
		return
	}
	overhead := 0
	for i, e := range s.table {
		if e != nil {
			s.table[i] = nil
		} else {
			overhead++
		}
	}
	if len(s.table) > 16 && overhead<<2 > len(s.table)*3 {
		s.table = make([]*Equation, len(s.table)>>1)
	}
	s.size = 0
	s.numDeleted = 0
}

func (s *equationSet) empty() bool { return s.size == 0 }

// snapshot lists the used slots in table order, as the source iterator
// visits them.
func (s *equationSet) snapshot() []*Equation {
	r := make([]*Equation, 0, s.size)
	for _, e := range s.table {
		if equationSlotUsed(e) {
			r = append(r, e)
		}
	}
	return r
}

type scope struct{ unfreeze, delete int }

// Solver is the source grobner object. It is single-goroutine state tied to
// one AST manager.
type Solver struct {
	manager *ast.ASTManager
	q       *numeral.RationalManager
	family  ast.FamilyID
	weights map[ast.ASTHandle]int
	// weightLayout is m_var2weight's obj_map layout (its iteration order).
	weightLayout *ast.ObjMap
	ids          map[ast.ASTHandle]uint32

	processed, pending  equationSet
	unfreeze, equations []*Equation
	scopes              []scope
	changedLeading      bool
	unsat               *Equation
	numNew              uint32
	closed              bool

	Stats Stats
}

func New(manager *ast.ASTManager) (*Solver, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	return &Solver{manager: manager, q: manager.NumeralService().Manager, family: family, weights: make(map[ast.ASTHandle]int), weightLayout: ast.NewObjMap(manager), ids: make(map[ast.ASTHandle]uint32)}, nil
}

func (s *Solver) check() error {
	if s == nil || s.closed || s.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Solver) ScopeLevel() uint32      { return uint32(len(s.scopes)) }
func (s *Solver) Inconsistent() bool      { return s != nil && s.unsat != nil }
func (s *Solver) NumNewEquations() uint32 { return s.numNew }

func (s *Solver) SetWeight(n ast.ASTHandle, weight int) error {
	if err := s.check(); err != nil {
		return err
	}
	if weight == 0 {
		return nil
	}
	if _, ok := s.weights[n]; !ok {
		if err := s.manager.IncRef(n); err != nil {
			return err
		}
		if err := s.weightLayout.Insert(n, [2]ast.ASTHandle{}); err != nil {
			return errors.Join(err, s.manager.DecRef(n))
		}
	}
	s.weights[n] = weight
	return nil
}
func (s *Solver) Weight(n ast.ASTHandle) int { return s.weights[n] }

func (s *Solver) astID(n ast.ASTHandle) (uint32, error) {
	if id, ok := s.ids[n]; ok {
		return id, nil
	}
	id, err := s.manager.GetASTID(n)
	if err == nil {
		s.ids[n] = id
	}
	return id, err
}

func (s *Solver) varLess(a, b ast.ASTHandle) (bool, error) {
	if a == b {
		return false, nil
	}
	wa, wb := s.weights[a], s.weights[b]
	if wa != wb {
		return wa > wb, nil
	}
	ia, err := s.astID(a)
	if err != nil {
		return false, err
	}
	ib, err := s.astID(b)
	if err != nil {
		return false, err
	}
	return ia < ib, nil
}

func (s *Solver) monomialLess(a, b *Monomial) (bool, error) {
	if len(a.vars) != len(b.vars) {
		return len(a.vars) > len(b.vars), nil
	}
	for i := range a.vars {
		if a.vars[i] == b.vars[i] {
			continue
		}
		return s.varLess(a.vars[i], b.vars[i])
	}
	return false, nil
}

func (s *Solver) sortVars(v []ast.ASTHandle) error {
	var cause error
	sort.SliceStable(v, func(i, j int) bool {
		less, err := s.varLess(v[i], v[j])
		if err != nil {
			cause = errors.Join(cause, err)
		}
		return less
	})
	return cause
}

func (s *Solver) sortMonomials(ms []*Monomial) error {
	var cause error
	sort.SliceStable(ms, func(i, j int) bool {
		less, err := s.monomialLess(ms[i], ms[j])
		if err != nil {
			cause = errors.Join(cause, err)
		}
		return less
	})
	return cause
}

// NewMonomial copies coeff, a rational of the AST manager's rational
// manager.
func (s *Solver) NewMonomial(coeff *numeral.Rational, vars []ast.ASTHandle) (*Monomial, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if coeff == nil {
		return nil, ast.ErrASTHandle
	}
	m := &Monomial{coeff: numeral.NewRational(), owner: s, live: true}
	if err := s.q.Copy(&m.coeff, coeff); err != nil {
		return nil, err
	}
	for _, v := range vars {
		if isNumeral, err := s.manager.IsArithmeticNumeral(v); err != nil {
			s.delMonomial(m)
			return nil, err
		} else if isNumeral {
			s.delMonomial(m)
			return nil, ast.ErrASTDomain
		}
		if err := s.manager.IncRef(v); err != nil {
			s.delMonomial(m)
			return nil, err
		}
		m.vars = append(m.vars, v)
	}
	if err := s.sortVars(m.vars); err != nil {
		s.delMonomial(m)
		return nil, err
	}
	return m, nil
}

func (s *Solver) delMonomial(m *Monomial) error {
	if m == nil || !m.live {
		return nil
	}
	if m.owner != s {
		return ast.ErrASTHandle
	}
	var result error
	for _, v := range m.vars {
		result = errors.Join(result, s.manager.DecRef(v))
	}
	result = errors.Join(result, s.q.Del(&m.coeff))
	m.vars = nil
	m.live = false
	return result
}
func (s *Solver) DelMonomial(m *Monomial) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.delMonomial(m)
}

func (s *Solver) cloneMonomial(m *Monomial) (*Monomial, error) {
	return s.NewMonomial(&m.coeff, m.vars)
}

// monomialFromExpr is grobner::mk_monomial(coeff, e). rational r and the
// scaled coefficient are freed on return.
func (s *Solver) monomialFromExpr(coeff *numeral.Rational, h ast.ASTHandle) (_ *Monomial, err error) {
	r, value := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, s.q.Del(&value), s.q.Del(&r)) }()
	_, isNumeral, err := s.manager.ArithmeticNumeralRational(h, &r)
	if err != nil {
		return nil, err
	}
	if isNumeral {
		if err = s.q.Mul(&r, coeff, &value); err != nil {
			return nil, err
		}
		return s.NewMonomial(&value, nil)
	}
	isMul, err := s.isArith(h, arithMul)
	if err != nil {
		return nil, err
	}
	if !isMul {
		return s.NewMonomial(coeff, []ast.ASTHandle{h})
	}

	body := h
	if err = s.q.Copy(&value, coeff); err != nil {
		return nil, err
	}
	first, err := s.manager.GetArg(body, 0)
	if err != nil {
		return nil, err
	}
	if _, yes, e := s.manager.ArithmeticNumeralRational(first, &r); e != nil {
		return nil, e
	} else if yes {
		if err = s.q.Mul(&value, &r, &value); err != nil {
			return nil, err
		}
		body, err = s.manager.GetArg(body, 1)
		if err != nil {
			return nil, err
		}
	}
	var vars []ast.ASTHandle
	for {
		mul, e := s.isArith(body, arithMul)
		if e != nil {
			return nil, e
		}
		if !mul {
			break
		}
		v, e := s.manager.GetArg(body, 0)
		if e != nil {
			return nil, e
		}
		vars = append(vars, v)
		body, e = s.manager.GetArg(body, 1)
		if e != nil {
			return nil, e
		}
	}
	vars = append(vars, body)
	return s.NewMonomial(&value, vars)
}

func (s *Solver) isArith(h ast.ASTHandle, op int32) (bool, error) {
	kind, err := s.manager.GetKind(h)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := s.manager.GetFamilyID(h)
	if err != nil {
		return false, err
	}
	if family != s.family {
		return false, nil
	}
	k, err := s.manager.GetDeclKind(h)
	return k == op, err
}

func sameBody(a, b *Monomial) bool {
	if len(a.vars) != len(b.vars) {
		return false
	}
	for i := range a.vars {
		if a.vars[i] != b.vars[i] {
			return false
		}
	}
	return true
}

func (s *Solver) simplifyMonomials(ms []*Monomial) ([]*Monomial, error) {
	if err := s.sortMonomials(ms); err != nil {
		return ms, err
	}
	if len(ms) == 0 {
		return ms, nil
	}
	// grobner.cpp merge_monomials: m_manager.limit().inc(sz), charged with
	// the size before merging; the C++ ignores the return value.
	s.manager.ResourceLimit().IncOffset(uint32(len(ms)))
	out := ms[:0]
	for _, m := range ms {
		if len(out) != 0 && sameBody(out[len(out)-1], m) {
			last := &out[len(out)-1].coeff
			if err := s.q.Add(last, &m.coeff, last); err != nil {
				return out, err
			}
			if err := s.delMonomial(m); err != nil {
				return out, err
			}
			continue
		}
		out = append(out, m)
	}
	keep := out[:0]
	for _, m := range out {
		if m.coeff.IsZero() {
			if err := s.delMonomial(m); err != nil {
				return keep, err
			}
		} else {
			keep = append(keep, m)
		}
	}
	if len(keep) != 0 && !keep[0].coeff.IsOne() {
		// normalize_coeff: rational c = m_monomials[0]->m_coeff, then
		// m_coeff /= c for every monomial.
		lead := numeral.NewRational()
		if err := s.q.Copy(&lead, &keep[0].coeff); err != nil {
			return keep, err
		}
		for _, m := range keep {
			if err := s.q.Div(&m.coeff, &lead, &m.coeff); err != nil {
				return keep, errors.Join(err, s.q.Del(&lead))
			}
		}
		if err := s.q.Del(&lead); err != nil {
			return keep, err
		}
	}
	return keep, nil
}

func (s *Solver) initEquation(ms []*Monomial, dep *Dependency) (*Equation, error) {
	e := &Equation{scope: s.ScopeLevel(), index: uint32(len(s.equations)), linear: true, monomials: ms, dependency: dep, owner: s, live: true}
	s.equations = append(s.equations, e)
	return e, nil
}

func (s *Solver) AssertMonomialsZero(ms []*Monomial, dep *Dependency) error {
	if err := s.check(); err != nil {
		return err
	}
	for _, m := range ms {
		if m == nil || !m.live || m.owner != s {
			return ast.ErrASTHandle
		}
	}
	work := append([]*Monomial(nil), ms...)
	result, err := s.simplifyMonomials(work)
	if err != nil {
		return err
	}
	if len(result) == 0 {
		return nil
	}
	e, err := s.initEquation(result, dep)
	if err != nil {
		return err
	}
	s.pending.insert(e)
	return nil
}

func (s *Solver) AssertExpressionsZero(expressions []ast.ASTHandle, dep *Dependency) error {
	coeffs := make([]numeral.Rational, len(expressions))
	for i := range coeffs {
		coeffs[i] = numeral.Rational{Num: numeral.NewInt(1), Den: numeral.NewInt(1)}
	}
	return s.AssertWeightedExpressionsZero(coeffs, expressions, dep)
}

// AssertWeightedExpressionsZero reads coeffs, rationals of the AST
// manager's rational manager.
func (s *Solver) AssertWeightedExpressionsZero(coeffs []numeral.Rational, expressions []ast.ASTHandle, dep *Dependency) error {
	if err := s.check(); err != nil {
		return err
	}
	if len(coeffs) != len(expressions) {
		return ast.ErrASTDomain
	}
	ms := make([]*Monomial, 0, len(expressions))
	for i, h := range expressions {
		m, err := s.monomialFromExpr(&coeffs[i], h)
		if err != nil {
			for _, x := range ms {
				_ = s.delMonomial(x)
			}
			return err
		}
		ms = append(ms, m)
	}
	return s.AssertMonomialsZero(ms, dep)
}

func (s *Solver) extractAdd(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	var out []ast.ASTHandle
	for {
		add, err := s.isArith(h, arithAdd)
		if err != nil {
			return nil, err
		}
		if !add {
			return append(out, h), nil
		}
		left, err := s.manager.GetArg(h, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, left)
		h, err = s.manager.GetArg(h, 1)
		if err != nil {
			return nil, err
		}
	}
}

func (s *Solver) AssertEquality(eq ast.ASTHandle, dep *Dependency) (err error) {
	if err := s.check(); err != nil {
		return err
	}
	if kind, err := s.manager.GetDeclKind(eq); err != nil || kind != int32(ast.ASTOpEq) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	lhs, err := s.manager.GetArg(eq, 0)
	if err != nil {
		return err
	}
	rhs, err := s.manager.GetArg(eq, 1)
	if err != nil {
		return err
	}
	// rational val
	val := numeral.NewRational()
	defer func() { err = errors.Join(err, s.q.Del(&val)) }()
	integer, yes, err := s.manager.ArithmeticNumeralRational(rhs, &val)
	if err != nil {
		return err
	}
	if !yes {
		return ast.ErrASTDomain
	}
	terms, err := s.extractAdd(lhs)
	if err != nil {
		return err
	}
	if !val.IsZero() {
		if err = s.q.Neg(&val); err != nil {
			return err
		}
		c, err := s.manager.ArithmeticNumeralFromRational(&val, integer)
		if err != nil {
			return err
		}
		terms = append(terms, c)
	}
	return s.AssertExpressionsZero(terms, dep)
}

func (s *Solver) deleteEquation(e *Equation) error {
	if e == nil || !e.live {
		return nil
	}
	s.processed.erase(e)
	s.pending.erase(e)
	if int(e.index) < len(s.equations) && s.equations[e.index] == e {
		s.equations[e.index] = nil
	}
	var result error
	for _, m := range e.monomials {
		result = errors.Join(result, s.delMonomial(m))
	}
	e.dependency = nil
	e.monomials = nil
	e.live = false
	return result
}

func (s *Solver) deleteFrom(limit int) error {
	var result error
	for _, e := range s.equations[limit:] {
		result = errors.Join(result, s.deleteEquation(e))
	}
	s.equations = s.equations[:limit]
	return result
}

func (s *Solver) Push() error {
	if err := s.check(); err != nil {
		return err
	}
	s.scopes = append(s.scopes, scope{len(s.unfreeze), len(s.equations)})
	return nil
}

func (s *Solver) Pop(n uint32) error {
	if err := s.check(); err != nil {
		return err
	}
	if n > uint32(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	if n == 0 {
		return nil
	}
	level := len(s.scopes) - int(n)
	mark := s.scopes[level]
	for _, e := range s.unfreeze[mark.unfreeze:] {
		if e != nil && e.live {
			s.pending.insert(e)
		}
	}
	s.unfreeze = s.unfreeze[:mark.unfreeze]
	err := s.deleteFrom(mark.delete)
	s.scopes = s.scopes[:level]
	s.recomputeUnsat()
	return err
}

func (s *Solver) recomputeUnsat() {
	s.unsat = nil
	for _, e := range s.equations {
		if e != nil && e.live && inconsistent(e) {
			s.unsat = e
			return
		}
	}
}

func (s *Solver) Reset() error {
	if err := s.check(); err != nil {
		return err
	}
	err := s.deleteFrom(0)
	s.processed.reset()
	s.pending.reset()
	s.unfreeze = nil
	s.scopes = nil
	s.unsat = nil
	return err
}

func (s *Solver) Close() error {
	if s == nil || s.closed {
		return nil
	}
	var result error
	if s.manager != nil {
		// flush: dec_ref_map_keys(m_var2weight) in table order, then
		// del_equations(0).
		for _, entry := range s.weightLayout.Entries() {
			result = errors.Join(result, s.manager.DecRef(entry.Key))
		}
		result = errors.Join(result, s.deleteFrom(0))
	}
	s.closed = true
	s.manager = nil
	s.weights = nil
	s.ids = nil
	return result
}

func (s *Solver) updateEquationOrder(e *Equation) (bool, error) {
	if len(e.monomials) == 0 {
		return false, nil
	}
	first := e.monomials[0]
	for _, m := range e.monomials {
		if err := s.sortVars(m.vars); err != nil {
			return false, err
		}
	}
	if err := s.sortMonomials(e.monomials); err != nil {
		return false, err
	}
	return first != e.monomials[0], nil
}

func (s *Solver) UpdateOrder() error {
	if err := s.check(); err != nil {
		return err
	}
	for _, e := range s.pending.snapshot() {
		if _, err := s.updateEquationOrder(e); err != nil {
			return err
		}
	}
	var remove []*Equation
	for _, e := range s.processed.snapshot() {
		changed, err := s.updateEquationOrder(e)
		if err != nil {
			return err
		}
		if changed {
			remove = append(remove, e)
			s.pending.insert(e)
		}
	}
	for _, e := range remove {
		s.processed.erase(e)
	}
	return nil
}

func (s *Solver) Equations() []*Equation {
	if s == nil {
		return nil
	}
	r := s.processed.snapshot()
	return append(r, s.pending.snapshot()...)
}

func (s *Solver) DisplayMonomial(w io.Writer, m *Monomial) error {
	if err := s.check(); err != nil {
		return err
	}
	if m == nil || !m.live || m.owner != s {
		return ast.ErrASTHandle
	}
	if len(m.vars) == 0 {
		return s.q.DisplayTo(w, &m.coeff)
	}
	if !m.coeff.IsOne() {
		if err := s.q.DisplayTo(w, &m.coeff); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "*"); err != nil {
			return err
		}
	}
	for i := 0; i < len(m.vars); {
		j := i + 1
		for j < len(m.vars) && m.vars[j] == m.vars[i] {
			j++
		}
		if i != 0 {
			if _, err := io.WriteString(w, "*"); err != nil {
				return err
			}
		}
		id, err := s.astID(m.vars[i])
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "#%d", id); err != nil {
			return err
		}
		if j-i > 1 {
			if _, err = fmt.Fprintf(w, "^%d", j-i); err != nil {
				return err
			}
		}
		i = j
	}
	return nil
}

func (s *Solver) DisplayEquation(w io.Writer, e *Equation) error {
	if e == nil || !e.live || e.owner != s {
		return ast.ErrASTHandle
	}
	for i, m := range e.monomials {
		if i != 0 {
			if _, err := io.WriteString(w, " + "); err != nil {
				return err
			}
		}
		if err := s.DisplayMonomial(w, m); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, " = 0\n")
	return err
}

func (s *Solver) Display(w io.Writer) error {
	if !s.processed.empty() {
		if _, err := io.WriteString(w, "processed:\n"); err != nil {
			return err
		}
		for _, e := range s.processed.snapshot() {
			if err := s.DisplayEquation(w, e); err != nil {
				return err
			}
		}
	}
	if !s.pending.empty() {
		if _, err := io.WriteString(w, "to process:\n"); err != nil {
			return err
		}
		for _, e := range s.pending.snapshot() {
			if err := s.DisplayEquation(w, e); err != nil {
				return err
			}
		}
	}
	return nil
}

func inconsistent(e *Equation) bool { return len(e.monomials) == 1 && len(e.monomials[0].vars) == 0 }
