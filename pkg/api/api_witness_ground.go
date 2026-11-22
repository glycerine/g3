package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

const maxWitnessBinders = 8

type witnessTuple struct {
	args [maxWitnessBinders]uint32
}
type witnessCandidate struct {
	tuple witnessTuple
	score uint32
}

// Bound names and matching annotations do not affect the value of a closed quantifier.
type witnessAtomKey struct {
	body      ast.ASTHandle
	sorts     [maxWitnessBinders]ast.ASTHandle
	n         int
	universal bool
}
type witnessAtom struct {
	key                  witnessAtomKey
	proxy                ast.ASTHandle
	used                 uint8
	goal                 bool
	singleBinaryRelation bool
	patterns             []witnessPattern
	tuples               map[witnessTuple]bool
}
type witnessPattern struct {
	decl      ast.ASTHandle
	positions [maxWitnessBinders]uint8
}

// The clause set holds all input and generated roots in a private arena.
// Their subterms stay live until the last ground assertion is installed.
type witnessClauseSet struct {
	m         *ast.ASTManager
	sub       *rewriter.VarSubstitution
	out       ast.ASTRef
	held      []ast.ASTHandle
	pages     []*[256]witnessTerm
	goalSorts map[ast.ASTHandle]bool
	terms     map[ast.ASTHandle][]ast.ASTHandle
	apps      map[ast.ASTHandle][][]ast.ASTHandle
	keys      map[witnessAtomKey]*witnessAtom
	atoms     []*witnessAtom

	roots      []ast.ASTHandle
	err        error
	instances  int
	generating bool
}

const (
	witnessHeld uint8 = 1 << iota
	witnessGoalTerm
	witnessFresh
	witnessDerived
	witnessObserved
)

type witnessTerm struct {
	term, lowered           ast.ASTHandle
	occurrences             uint32
	flags, polarity, forced uint8
}

// Pages give the matcher a stable pointer to each term's metadata while new
// instances grow the arena. All participating terms are held directly or as
// subterms of held roots, so a slot cannot be recycled during an attempt.
func (s *witnessClauseSet) term(h ast.ASTHandle) *witnessTerm {
	id := h.Slot()
	if h.IsNull() || id >= 300_000 {
		s.err = errWitnessLimit
		return nil
	}
	p := int(id / 256)
	if p >= len(s.pages) {
		s.pages = append(s.pages, make([]*[256]witnessTerm, p+1-len(s.pages))...)
	}
	if s.pages[p] == nil {
		s.pages[p] = new([256]witnessTerm)
	}
	t := &s.pages[p][id%256]
	if t.term.IsNull() {
		t.term = h
	} else if t.term != h {
		s.err = errWitnessLimit
		return nil
	}
	return t
}

func (s *witnessClauseSet) value(id uint32) *witnessTerm { return &s.pages[id/256][id%256] }

var errWitnessLimit = errors.New("ground witness proof limit")

func (s *witnessClauseSet) charge(n int) bool {
	if s.err != nil {
		return false
	}
	if !s.m.ResourceLimit().IncOffset(uint32(n)) {
		s.err = errWitnessLimit
		return false
	}
	return true
}
func (s *witnessClauseSet) keep(h ast.ASTHandle, e error) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	if e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	t := s.term(h)
	if t == nil {
		return ast.ASTHandle{}
	}
	if t.flags&witnessHeld == 0 {
		if e = s.m.IncRef(h); e != nil {
			s.err = e
			return ast.ASTHandle{}
		}
		t.flags |= witnessHeld
		s.held = append(s.held, h)
	}
	return h
}
func (s *witnessClauseSet) freshConst(sort ast.ASTHandle) ast.ASTHandle {
	h, err := s.m.MkFreshConst("g3!ground", sort)
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	// Translate and hold the complete input first. Any colliding application
	// or declaration (including one referenced in a declaration parameter) is
	// already owned. A new nullary application owns exactly its declaration.
	header, err := s.m.NodeHeader(h)
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	refs, err := s.m.GetRefCount(header.Decl)
	h = s.keep(h, err)
	if s.err != nil {
		return ast.ASTHandle{}
	}
	if header.RefCount != 0 || refs != 1 {
		s.err = errWitnessLimit
		return ast.ASTHandle{}
	}
	t := s.value(h.Slot())
	t.flags |= witnessFresh
	if s.generating {
		t.flags |= witnessDerived
	}
	return h
}

func (s *witnessClauseSet) substitute(q *witnessAtom, ids []uint32) ast.ASTHandle {
	if !s.charge(1 + len(ids)) {
		return ast.ASTHandle{}
	}
	var args [maxWitnessBinders]ast.ASTHandle
	for i, id := range ids {
		args[i] = s.value(id).term
	}
	if e := s.sub.Substitute(q.key.body, args[:len(ids)], s.out); e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	return s.keep(s.m.RefGet(s.out))
}

// For a false universal choose a fresh tuple where its body is false. For a
// true existential choose one where the body is true. Guard each witness by
// the proxy, so an unselected disjunct cannot impose a witness on the model.
func (s *witnessClauseSet) atom(h ast.ASTHandle, pol uint8, goal bool) ast.ASTHandle {
	m := s.m
	body, e := m.GetQuantifierBody(h)
	if e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	n, e := m.GetNumDeclarations(h)
	if e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	if n == 0 || n > maxWitnessBinders {
		s.err = errWitnessLimit
		return ast.ASTHandle{}
	}
	all, e := m.IsForall(h)
	if e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	key := witnessAtomKey{body: body, universal: all, n: int(n)}
	for i := uint32(0); i < n; i++ {
		key.sorts[i], e = m.GetDeclarationSort(h, i)
		if e != nil {
			s.err = e
			return ast.ASTHandle{}
		}
	}
	q := s.keys[key]
	if q == nil {
		if len(s.keys) > 2200 {
			s.err = errWitnessLimit
			return ast.ASTHandle{}
		}
		b, e := m.MkBoolSort()
		if e != nil {
			s.err = e
			return ast.ASTHandle{}
		}
		q = &witnessAtom{key: key, proxy: s.freshConst(b)}
		s.keys[key] = q
		s.atoms = append(s.atoms, q)
	}
	q.goal = q.goal || goal
	newPol := pol &^ q.used
	q.used |= pol
	witness := witnessPositive
	if all {
		witness = witnessNegative
	}
	if newPol&witness != 0 && s.err == nil {
		var tuple witnessTuple
		for i := 0; i < int(n); i++ {
			h := s.freshConst(key.sorts[i])
			tuple.args[i] = h.Slot()
			if s.err != nil {
				return q.proxy
			}
			if goal {
				s.value(h.Slot()).flags |= witnessGoalTerm
			}
		}
		b := s.substitute(q, tuple.args[:n])
		if s.err != nil {
			return q.proxy
		}
		b = s.lower(b, witness, goal)
		if s.err != nil {
			return q.proxy
		}
		if all {
			s.roots = append(s.roots, s.keep(m.MkImplies(b, q.proxy)))
		} else {
			s.roots = append(s.roots, s.keep(m.MkImplies(q.proxy, b)))
		}
	}
	return q.proxy
}
func (s *witnessClauseSet) lower(h ast.ASTHandle, pol uint8, goal bool) ast.ASTHandle {
	if !s.charge(1) {
		return ast.ASTHandle{}
	}
	m := s.m
	t := s.term(h)
	if t == nil {
		return ast.ASTHandle{}
	}
	if pol&^t.polarity == 0 {
		return t.lowered
	}
	hd, e := m.NodeHeader(h)
	if e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	if hd.Kind == ast.ASTQuantifier {
		v := s.atom(h, pol, goal)
		t.lowered = v
		t.polarity |= pol
		return v
	}
	if hd.Kind != ast.ASTApp {
		s.err = errWitnessLimit
		return ast.ASTHandle{}
	}
	has, e := m.HasQuantifiers(h)
	if e != nil {
		s.err = e
		return ast.ASTHandle{}
	}
	if !has {
		s.observe(h, goal)
		t.lowered = h
		t.polarity = witnessEither
		return h
	}
	if hd.NumArgs > 8192 {
		s.err = errWitnessLimit
		return ast.ASTHandle{}
	}
	args := make([]ast.ASTHandle, hd.NumArgs)
	for i := range args {
		a, e := m.GetArg(h, uint32(i))
		if e != nil {
			s.err = e
			return ast.ASTHandle{}
		}
		k := witnessChildPolarity(m, hd, pol, uint32(i))
		args[i] = s.lower(a, k, goal)
		if s.err != nil {
			return ast.ASTHandle{}
		}
	}
	v := s.keep(m.MkApp(hd.Decl, args))
	t.lowered = v
	t.polarity |= pol
	return v
}

// observe indexes real ground applications and their argument tuples. Open
// subterms stay in the quantifier template until its variables are bound.
func (s *witnessClauseSet) observe(h ast.ASTHandle, goal bool) {
	if !s.charge(1) {
		return
	}
	t := s.term(h)
	if t == nil {
		return
	}
	if goal {
		t.flags |= witnessGoalTerm
	}
	t.occurrences++
	if t.flags&witnessObserved != 0 {
		return
	}
	t.flags |= witnessObserved
	hd, e := s.m.NodeHeader(h)
	if e != nil {
		s.err = e
		return
	}
	if hd.Kind != ast.ASTApp {
		return
	}
	var args []ast.ASTHandle
	if hd.Family == ast.NullFamilyID {
		st, e := s.m.GetSort(h)
		if e != nil {
			s.err = e
			return
		}
		if s.goalSorts[st] {
			s.terms[st] = append(s.terms[st], h)
		}
		if hd.NumArgs > 0 && hd.NumArgs <= 16 {
			args = make([]ast.ASTHandle, hd.NumArgs)
		}
	}
	for i := uint32(0); i < hd.NumArgs; i++ {
		a, e := s.m.GetArg(h, i)
		if e != nil {
			s.err = e
			return
		}
		if args != nil {
			args[i] = a
		}
		s.observe(a, goal)
		if s.err != nil {
			return
		}
	}
	if args != nil {
		s.apps[hd.Decl] = append(s.apps[hd.Decl], args)
	}
}

// In the other direction every ground tuple satisfies Q => body for a
// universal, or body => Q for an existential. A root unit can discharge the
// guard. Only a refutation of these consequences is returned to the caller.
func (s *witnessClauseSet) instantiate(q *witnessAtom, tuple witnessTuple) {
	if q.tuples == nil {
		q.tuples = make(map[witnessTuple]bool)
	}
	if q.tuples[tuple] {
		return
	}
	q.tuples[tuple] = true
	s.instances++
	b := s.substitute(q, tuple.args[:q.key.n])
	if s.err != nil {
		return
	}
	if q.key.universal {
		b = s.lower(b, witnessPositive, false)
		if s.err == nil {
			if s.value(q.proxy.Slot()).forced&witnessPositive != 0 {
				s.roots = append(s.roots, b)
			} else {
				s.roots = append(s.roots, s.keep(s.m.MkImplies(q.proxy, b)))
			}
		}
	} else {
		b = s.lower(b, witnessNegative, false)
		if s.err == nil {
			if s.value(q.proxy.Slot()).forced&witnessNegative != 0 {
				s.roots = append(s.roots, s.keep(s.m.MkNot(b)))
			} else {
				s.roots = append(s.roots, s.keep(s.m.MkImplies(b, q.proxy)))
			}
		}
	}
}
func (s *witnessClauseSet) collectForced(h ast.ASTHandle, pol uint8) {
	if !s.charge(1) {
		return
	}
	t := s.term(h)
	if t == nil || t.forced&pol != 0 {
		return
	}
	t.forced |= pol
	hd, e := s.m.NodeHeader(h)
	if e != nil {
		s.err = e
		return
	}
	if hd.Kind != ast.ASTApp || hd.Family != s.m.BasicFamilyID() {
		return
	}
	for i := uint32(0); i < hd.NumArgs; i++ {
		k := pol
		switch hd.DeclKind {
		case ast.ASTOpAnd:
			if pol != 1 {
				return
			}
		case ast.ASTOpOr:
			if pol != 2 {
				return
			}
		case ast.ASTOpImplies:
			if pol != 2 {
				return
			}
			if i == 0 {
				k = 1
			}
		case ast.ASTOpNot:
			k = 3 ^ pol
		default:
			return
		}
		a, e := s.m.GetArg(h, i)
		if e != nil {
			s.err = e
			return
		}
		s.collectForced(a, k)
	}
}
