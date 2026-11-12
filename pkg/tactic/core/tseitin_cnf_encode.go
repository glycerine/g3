package core

// Source: src/tactic/core/tseitin_cnf_tactic.cpp:48-748.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

type tseitinFrame struct {
	term  ast.ASTHandle
	first bool
}
type tseitinMatch uint8

const (
	tseitinNo tseitinMatch = iota
	tseitinContinue
	tseitinDone
)

type tseitinClause struct {
	ref ast.ASTRef
	dep ast.ASTDependency
}

type tseitinInvocation struct {
	owner         *TseitinCNFTactic
	rw            *rewriter.BoolRewriter
	occs          *tactic.GoalSharedOccurrences
	cache         map[ast.ASTHandle]ast.ASTHandle
	frames        []tseitinFrame
	freshRefs     []ast.ASTRef // m_fresh_vars
	domainRefs    []ast.ASTRef // m_cache_domain
	clauses       []tseitinClause
	fresh         []ast.ASTHandle
	currDep       ast.ASTDependency
	models, cores bool
	numAux        uint32
}

func newTseitinInvocation(t *TseitinCNFTactic) (_ *tseitinInvocation, err error) {
	rw, err := rewriter.NewBoolRewriter(t.manager, t.context, t.globals, &t.params)
	if err != nil {
		return nil, err
	}
	if err = rw.SetFlat(false); err != nil {
		return nil, errors.Join(err, rw.Close())
	}
	occs, err := tactic.NewGoalSharedOccurrences(t.manager, false, false, false)
	if err != nil {
		return nil, errors.Join(err, rw.Close())
	}
	return &tseitinInvocation{owner: t, rw: rw, occs: occs, cache: make(map[ast.ASTHandle]ast.ASTHandle)}, nil
}
func (c *tseitinInvocation) close() (err error) {
	// The imp members end in reverse declaration order: m_deps, m_clauses,
	// m_fresh_vars, then m_cache_domain, each vector in index order.
	for i := range c.clauses {
		if !c.clauses[i].dep.IsNull() {
			err = errors.Join(err, c.owner.manager.DependencyDec(c.clauses[i].dep))
		}
	}
	for i := range c.clauses {
		err = errors.Join(err, c.owner.manager.DelRef(&c.clauses[i].ref))
	}
	for i := range c.freshRefs {
		err = errors.Join(err, c.owner.manager.DelRef(&c.freshRefs[i]))
	}
	for i := range c.domainRefs {
		err = errors.Join(err, c.owner.manager.DelRef(&c.domainRefs[i]))
	}
	if c.occs != nil {
		err = errors.Join(err, c.occs.Close())
	}
	if c.rw != nil {
		err = errors.Join(err, c.rw.Close())
	}
	return err
}

// tseitinRef is one expr_ref/app_ref local of the source matchers:
// assignment retains the new value before releasing the old one, and the
// release at the end of the source scope may delete a temporary.
type tseitinRef struct {
	m   *ast.ASTManager
	ref ast.ASTRef
	h   ast.ASTHandle
	set bool
}

func (c *tseitinInvocation) newRef() *tseitinRef { return &tseitinRef{m: c.owner.manager} }
func (r *tseitinRef) get() ast.ASTHandle         { return r.h }
func (r *tseitinRef) assign(h ast.ASTHandle) error {
	next, err := r.m.NewRef(h)
	if err != nil {
		return err
	}
	if r.set {
		err = r.m.DelRef(&r.ref)
	}
	r.ref, r.h, r.set = next, h, true
	return err
}
func (r *tseitinRef) release() error {
	if !r.set {
		return nil
	}
	r.set = false
	r.h = ast.ASTHandle{}
	return r.m.DelRef(&r.ref)
}

// releaseRefs ends source locals in the given (reverse declaration) order.
func releaseRefs(refs ...*tseitinRef) (err error) {
	for _, r := range refs {
		err = errors.Join(err, r.release())
	}
	return err
}

// tseitinBuffer is an expr_ref_buffer: pushes retain, the destructor
// releases in index order.
type tseitinBuffer struct {
	m    *ast.ASTManager
	refs []ast.ASTRef
	hs   []ast.ASTHandle
}

func (c *tseitinInvocation) newBuffer() *tseitinBuffer { return &tseitinBuffer{m: c.owner.manager} }
func (b *tseitinBuffer) push(h ast.ASTHandle) error {
	r, err := b.m.NewRef(h)
	if err != nil {
		return err
	}
	b.refs = append(b.refs, r)
	b.hs = append(b.hs, h)
	return nil
}
func (b *tseitinBuffer) pop() error {
	i := len(b.refs) - 1
	err := b.m.DelRef(&b.refs[i])
	b.refs, b.hs = b.refs[:i], b.hs[:i]
	return err
}
func (b *tseitinBuffer) handles() []ast.ASTHandle { return append([]ast.ASTHandle(nil), b.hs...) }
func (b *tseitinBuffer) release() (err error) {
	for i := range b.refs {
		err = errors.Join(err, b.m.DelRef(&b.refs[i]))
	}
	b.refs, b.hs = nil, nil
	return err
}

// hold retains h in one of the source's member vectors, m_fresh_vars or
// m_cache_domain.
func (c *tseitinInvocation) hold(owned *[]ast.ASTRef, h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		*owned = append(*owned, r)
	}
	return h, err
}
func (c *tseitinInvocation) op(h ast.ASTHandle) (int32, []ast.ASTHandle, bool, error) {
	m := c.owner.manager
	k, err := m.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return 0, nil, false, err
	}
	f, err := m.GetFamilyID(h)
	if err != nil || f != m.BasicFamilyID() {
		return 0, nil, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return 0, nil, false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return 0, nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		if args[i], err = m.GetArg(h, uint32(i)); err != nil {
			return 0, nil, false, err
		}
	}
	return op, args, true, nil
}
func (c *tseitinInvocation) isBool(h ast.ASTHandle) (bool, error) {
	s, err := c.owner.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := c.owner.manager.MkBoolSort()
	return s == b, err
}
func (c *tseitinInvocation) iff(h ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	op, a, app, err := c.op(h)
	if err != nil || !app || len(a) != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if op == ast.ASTOpIff {
		return a[0], a[1], true, nil
	}
	if op == ast.ASTOpEq {
		b, e := c.isBool(a[0])
		return a[0], a[1], b, e
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}
func (c *tseitinInvocation) notArg(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	y, a, e := c.owner.manager.IsNot(h)
	return a, y, e
}
func (c *tseitinInvocation) isOp(h ast.ASTHandle, wanted int32) ([]ast.ASTHandle, bool, error) {
	op, a, app, e := c.op(h)
	return a, app && op == wanted, e
}
func (c *tseitinInvocation) shared(h ast.ASTHandle) (bool, error) { return c.occs.IsShared(h) }

func (c *tseitinInvocation) visit(h ast.ASTHandle, visited *bool, root bool) error {
	for {
		if _, ok := c.cache[h]; ok {
			return nil
		}
		op, args, app, err := c.op(h)
		if err != nil || !app || len(args) == 0 {
			return err
		}
		switch op {
		case ast.ASTOpNot:
			if !root {
				h = args[0]
				continue
			}
		case ast.ASTOpOr, ast.ASTOpIff:
		case ast.ASTOpIte, ast.ASTOpEq:
			b, e := c.isBool(args[1])
			if e != nil {
				return e
			}
			if !b {
				return nil
			}
		case ast.ASTOpAnd, ast.ASTOpXor, ast.ASTOpImplies, ast.ASTOpDistinct:
			return tactic.NewTacticException("operator not supported, apply simplifier before invoking this strategy")
		default:
			return nil
		}
		*visited = false
		c.frames = append(c.frames, tseitinFrame{h, true})
		return nil
	}
}
func (c *tseitinInvocation) mkNot(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	if m.IsTrue(h) {
		x, e := m.MkFalse()
		return x, e
	}
	if m.IsFalse(h) {
		x, e := m.MkTrue()
		return x, e
	}
	if a, yes, e := c.notArg(h); e != nil || yes {
		return a, e
	}
	return m.MkNot(h)
}

// mkLitNot is mk_lit's sign case: a plain m.mk_not, unlike inv (mkNot).
func (c *tseitinInvocation) mkLitNot(h ast.ASTHandle) (ast.ASTHandle, error) {
	return c.owner.manager.MkNot(h)
}

// lit is get_lit(n, sign, r).
func (c *tseitinInvocation) lit(h ast.ASTHandle, sign bool) (ast.ASTHandle, error) {
	for {
		op, args, app, e := c.op(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !app || len(args) == 0 {
			if sign {
				return c.mkLitNot(h)
			}
			return h, nil
		}
		if op == ast.ASTOpNot {
			h = args[0]
			sign = !sign
			continue
		}
		if op == ast.ASTOpOr || op == ast.ASTOpIff || op == ast.ASTOpIte || op == ast.ASTOpEq {
			if op == ast.ASTOpIte || op == ast.ASTOpEq {
				b, x := c.isBool(args[1])
				if x != nil {
					return ast.ASTHandle{}, x
				}
				if !b {
					if sign {
						return c.mkLitNot(h)
					}
					return h, nil
				}
			}
			v, ok := c.cache[h]
			if !ok {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			if sign {
				return c.mkLitNot(v)
			}
			return v, nil
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}
func (c *tseitinInvocation) clause(lits ...ast.ASTHandle) error {
	r, err := c.owner.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = c.rw.MkOr(lits, r); err != nil {
		return errors.Join(err, c.owner.manager.DelRef(&r))
	}
	d := ast.ASTDependency{}
	if c.cores {
		d = c.currDep
		if err = c.owner.manager.DependencyInc(d); err != nil {
			return errors.Join(err, c.owner.manager.DelRef(&r))
		}
	}
	c.clauses = append(c.clauses, tseitinClause{r, d})
	return nil
}
func (c *tseitinInvocation) freshVar() (ast.ASTHandle, error) {
	s, e := c.owner.manager.MkBoolSort()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	v, e := c.owner.manager.MkFreshConstNullable(nil, s)
	if e != nil {
		return v, e
	}
	v, e = c.hold(&c.freshRefs, v)
	if e == nil {
		c.fresh = append(c.fresh, v)
		c.numAux++
	}
	return v, e
}
func (c *tseitinInvocation) cacheResult(term, value ast.ASTHandle) error {
	if _, e := c.hold(&c.domainRefs, term); e != nil {
		return e
	}
	c.cache[term] = value
	return nil
}
func (c *tseitinInvocation) checkpoint() error {
	if e := c.owner.services.Cooperation.Cooperate("tseitin cnf"); e != nil {
		return e
	}
	n, e := c.owner.services.Memory.AllocationSize()
	if e != nil {
		return e
	}
	if n > c.owner.maxMemory {
		return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	y, e := c.owner.manager.Canceled()
	if e != nil {
		return e
	}
	if y {
		return tactic.NewTacticException(c.owner.manager.ResourceLimit().CancelMessage())
	}
	return nil
}

func (c *tseitinInvocation) or3and(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	if !c.owner.commonPatterns {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	x, ok, e := c.isOp(t, ast.ASTOpOr)
	if e != nil || !ok || len(x) != 3 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	pairs := make([][2]ast.ASTHandle, 3)
	for i := range x {
		n, yes, e := c.notArg(x[i])
		if e != nil || !yes {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		shared, e := c.shared(n)
		if e != nil || shared {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		a, yes, e := c.isOp(n, ast.ASTOpOr)
		if e != nil || !yes || len(a) != 2 {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		pairs[i] = [2]ast.ASTHandle{a[0], a[1]}
		sort.Slice(pairs[i][:], func(j, k int) bool {
			aj, _ := c.owner.manager.GetASTID(pairs[i][j])
			ak, _ := c.owner.manager.GetASTID(pairs[i][k])
			return aj < ak
		})
	}
	a1, a2, b1, b2, d1, d2 := pairs[0][0], pairs[0][1], pairs[1][0], pairs[1][1], pairs[2][0], pairs[2][1]
	if (a1 == b1 && a2 == d1 && b2 == d2) || (a1 == b1 && a2 == d2 && b2 == d1) || (a1 == d1 && a2 == b1 && b2 == d2) {
		return a1, a2, b2, true, nil
	}
	if (a1 == b2 && a2 == d2 && b1 == d1) || (a1 == d1 && a2 == b2 && b1 == d2) || (a1 == d2 && a2 == b2 && b1 == d1) {
		return a1, a2, b1, true, nil
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}
func (c *tseitinInvocation) iff3(t ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	l, r, yes, e := c.iff(t)
	if e != nil || !yes {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	sh, e := c.shared(l)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if !sh {
		a, b, y, x := c.iff(l)
		if x != nil {
			return a, b, r, false, x
		}
		if y {
			return a, b, r, true, nil
		}
	}
	sh, e = c.shared(r)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if !sh {
		b, d, y, x := c.iff(r)
		if x != nil {
			return l, b, d, false, x
		}
		if y {
			return l, b, d, true, nil
		}
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}

func (c *tseitinInvocation) visitAll(first bool, terms ...ast.ASTHandle) (tseitinMatch, error) {
	if !first {
		return tseitinDone, nil
	}
	visited := true
	for _, h := range terms {
		if e := c.visit(h, &visited, false); e != nil {
			return tseitinNo, e
		}
	}
	if !visited {
		return tseitinContinue, nil
	}
	return tseitinDone, nil
}
func (c *tseitinInvocation) matchNot(t ast.ASTHandle, first, root bool) (_ tseitinMatch, err error) {
	a, yes, e := c.notArg(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	nla := c.newRef()
	defer func() { err = errors.Join(err, nla.release()) }()
	h, e := c.lit(a, true)
	if e != nil {
		return tseitinNo, e
	}
	if e = nla.assign(h); e != nil {
		return tseitinNo, e
	}
	if root {
		if e = c.clause(nla.get()); e != nil {
			return tseitinNo, e
		}
	}
	return tseitinDone, nil
}

// assignLit is get_lit(n, sign, r).
func (c *tseitinInvocation) assignLit(r *tseitinRef, n ast.ASTHandle, sign bool) error {
	h, e := c.lit(n, sign)
	if e != nil {
		return e
	}
	return r.assign(h)
}

// assignInv is inv(n, r).
func (c *tseitinInvocation) assignInv(r *tseitinRef, n ast.ASTHandle) error {
	h, e := c.mkNot(n)
	if e != nil {
		return e
	}
	return r.assign(h)
}

// assignFresh is k = mk_fresh(); nk = m.mk_not(k); the caller caches k.
func (c *tseitinInvocation) assignFresh(k, nk *tseitinRef) error {
	v, e := c.freshVar()
	if e != nil {
		return e
	}
	if e = k.assign(v); e != nil {
		return e
	}
	n, e := c.owner.manager.MkNot(v)
	if e != nil {
		return e
	}
	return nk.assign(n)
}

func (c *tseitinInvocation) matchOr3And(t ast.ASTHandle, first, root bool) (_ tseitinMatch, err error) {
	a, b, d, yes, e := c.or3and(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a, b, d)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	nla, nlb, nlc := c.newRef(), c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nlc, nlb, nla)) }()
	if e = errors.Join(c.assignLit(nla, a, true), c.assignLit(nlb, b, true), c.assignLit(nlc, d, true)); e != nil {
		return tseitinNo, e
	}
	if root {
		return tseitinDone, errors.Join(c.clause(nla.get(), nlb.get()), c.clause(nla.get(), nlc.get()), c.clause(nlb.get(), nlc.get()))
	}
	k, nk := c.newRef(), c.newRef()
	la, lb, lc := c.newRef(), c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(lc, lb, la, nk, k)) }()
	if e = c.assignFresh(k, nk); e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(nk.get(), nla.get(), nlb.get()), c.clause(nk.get(), nla.get(), nlc.get()), c.clause(nk.get(), nlb.get(), nlc.get()))
	if e != nil {
		return tseitinNo, e
	}
	if e = errors.Join(c.assignInv(la, nla.get()), c.assignInv(lb, nlb.get()), c.assignInv(lc, nlc.get())); e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(k.get(), la.get(), lb.get()), c.clause(k.get(), la.get(), lc.get()), c.clause(k.get(), lb.get(), lc.get()))
	if e == nil {
		e = c.cacheResult(t, k.get())
	}
	if e == nil {
		// The else block's la..lc, k and nk end before nla..nlc.
		e = releaseRefs(lc, lb, la, nk, k)
	}
	return tseitinDone, e
}
func (c *tseitinInvocation) matchIff3(t ast.ASTHandle, first, root bool) (_ tseitinMatch, err error) {
	a, b, d, yes, e := c.iff3(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a, b, d)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	la, lb, lc := c.newRef(), c.newRef(), c.newRef()
	nla, nlb, nlc := c.newRef(), c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nlc, nlb, nla, lc, lb, la)) }()
	if e = errors.Join(c.assignLit(la, a, false), c.assignLit(lb, b, false), c.assignLit(lc, d, false)); e != nil {
		return tseitinNo, e
	}
	if e = errors.Join(c.assignInv(nla, la.get()), c.assignInv(nlb, lb.get()), c.assignInv(nlc, lc.get())); e != nil {
		return tseitinNo, e
	}
	if root {
		return tseitinDone, errors.Join(c.clause(la.get(), lb.get(), lc.get()), c.clause(la.get(), nlb.get(), nlc.get()), c.clause(nla.get(), lb.get(), nlc.get()), c.clause(nla.get(), nlb.get(), lc.get()))
	}
	k, nk := c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nk, k)) }()
	if e = c.assignFresh(k, nk); e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(nk.get(), la.get(), lb.get(), lc.get()), c.clause(nk.get(), la.get(), nlb.get(), nlc.get()), c.clause(nk.get(), nla.get(), lb.get(), nlc.get()), c.clause(nk.get(), nla.get(), nlb.get(), lc.get()),
		c.clause(k.get(), nla.get(), nlb.get(), nlc.get()), c.clause(k.get(), nla.get(), lb.get(), lc.get()), c.clause(k.get(), la.get(), nlb.get(), lc.get()), c.clause(k.get(), la.get(), lb.get(), nlc.get()))
	if e == nil {
		e = c.cacheResult(t, k.get())
	}
	if e == nil {
		e = releaseRefs(nk, k)
	}
	return tseitinDone, e
}
func (c *tseitinInvocation) matchIff(t ast.ASTHandle, first, root bool) (_ tseitinMatch, err error) {
	a, b, yes, e := c.iff(t)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		r, e := c.visitAll(true, a, b)
		if e != nil || r == tseitinContinue {
			return r, e
		}
	}
	la, lb := c.newRef(), c.newRef()
	nla, nlb := c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nlb, nla, lb, la)) }()
	if e = errors.Join(c.assignLit(la, a, false), c.assignLit(lb, b, false)); e != nil {
		return tseitinNo, e
	}
	if e = errors.Join(c.assignInv(nla, la.get()), c.assignInv(nlb, lb.get())); e != nil {
		return tseitinNo, e
	}
	if root {
		return tseitinDone, errors.Join(c.clause(la.get(), nlb.get()), c.clause(nla.get(), lb.get()))
	}
	k, nk := c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nk, k)) }()
	if e = c.assignFresh(k, nk); e != nil {
		return tseitinNo, e
	}
	e = errors.Join(c.clause(nk.get(), la.get(), nlb.get()), c.clause(nk.get(), nla.get(), lb.get()), c.clause(k.get(), nla.get(), nlb.get()), c.clause(k.get(), la.get(), lb.get()))
	if e == nil {
		e = c.cacheResult(t, k.get())
	}
	if e == nil {
		e = releaseRefs(nk, k)
	}
	return tseitinDone, e
}

func (c *tseitinInvocation) iteArgs(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	return c.isOp(h, ast.ASTOpIte)
}
func (c *tseitinInvocation) unsharedIte(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	a, y, e := c.iteArgs(h)
	if e != nil || !y {
		return nil, false, e
	}
	sh, e := c.shared(h)
	return a, !sh, e
}
func (c *tseitinInvocation) visitIteChain(t ast.ASTHandle) (bool, error) {
	visited := true
	ite := t
	for {
		a, yes, e := c.iteArgs(ite)
		if e != nil || !yes {
			return visited, e
		}
		if e = c.visit(a[0], &visited, false); e != nil {
			return visited, e
		}
		if c.owner.iteChains {
			if _, yes, e := c.unsharedIte(a[1]); e != nil {
				return visited, e
			} else if yes {
				if e = c.visit(a[2], &visited, false); e != nil {
					return visited, e
				}
				ite = a[1]
				continue
			}
			if _, yes, e := c.unsharedIte(a[2]); e != nil {
				return visited, e
			} else if yes {
				if e = c.visit(a[1], &visited, false); e != nil {
					return visited, e
				}
				ite = a[2]
				continue
			}
		}
		if e = c.visit(a[1], &visited, false); e != nil {
			return visited, e
		}
		if e = c.visit(a[2], &visited, false); e != nil {
			return visited, e
		}
		return visited, nil
	}
}
func (c *tseitinInvocation) matchIte(t ast.ASTHandle, first, root bool) (_ tseitinMatch, err error) {
	if _, yes, e := c.iteArgs(t); e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		visited, e := c.visitIteChain(t)
		if e != nil {
			return tseitinNo, e
		}
		if !visited {
			return tseitinContinue, nil
		}
	}
	ctx, exPos, exNeg := c.newBuffer(), c.newBuffer(), c.newBuffer()
	la, lb, lc := c.newRef(), c.newRef(), c.newRef()
	nla, nlb, nlc := c.newRef(), c.newRef(), c.newRef()
	k, nk := c.newRef(), c.newRef()
	defer func() {
		err = errors.Join(err, releaseRefs(nk, k, nlc, nlb, nla, lc, lb, la), exNeg.release(), exPos.release(), ctx.release())
	}()
	if !root {
		if e := c.assignFresh(k, nk); e != nil {
			return tseitinNo, e
		}
		if e := c.cacheResult(t, k.get()); e != nil {
			return tseitinNo, e
		}
	}
	// MK_ITE_CLS and MK_ITE_ROOT_CLS: the clause of ctx followed by ls.
	cls := func(ls ...ast.ASTHandle) (e error) {
		for _, l := range ls {
			if e = ctx.push(l); e != nil {
				return e
			}
		}
		e = c.clause(ctx.handles()...)
		for range ls {
			e = errors.Join(e, ctx.pop())
		}
		return e
	}
	ite := t
	for {
		a, _, x := c.iteArgs(ite)
		if x != nil {
			return tseitinNo, x
		}
		if x = c.assignLit(la, a[0], false); x != nil {
			return tseitinNo, x
		}
		if x = c.assignInv(nla, la.get()); x != nil {
			return tseitinNo, x
		}
		if c.owner.iteChains {
			if _, yes, x := c.unsharedIte(a[1]); x != nil {
				return tseitinNo, x
			} else if yes {
				if x = c.assignLit(lc, a[2], false); x != nil {
					return tseitinNo, x
				}
				if root {
					x = cls(la.get(), lc.get())
				} else {
					if x = c.assignInv(nlc, lc.get()); x != nil {
						return tseitinNo, x
					}
					x = errors.Join(cls(la.get(), lc.get(), nk.get()), cls(la.get(), nlc.get(), k.get()))
					if x == nil && c.owner.iteExtra {
						x = errors.Join(exNeg.push(lc.get()), exPos.push(nlc.get()))
					}
				}
				if x == nil {
					x = ctx.push(nla.get())
				}
				if x != nil {
					return tseitinNo, x
				}
				ite = a[1]
				continue
			}
			if _, yes, x := c.unsharedIte(a[2]); x != nil {
				return tseitinNo, x
			} else if yes {
				if x = c.assignLit(lb, a[1], false); x != nil {
					return tseitinNo, x
				}
				if root {
					x = cls(nla.get(), lb.get())
				} else {
					if x = c.assignInv(nlb, lb.get()); x != nil {
						return tseitinNo, x
					}
					x = errors.Join(cls(nla.get(), lb.get(), nk.get()), cls(nla.get(), nlb.get(), k.get()))
					if x == nil && c.owner.iteExtra {
						x = errors.Join(exNeg.push(lb.get()), exPos.push(nlb.get()))
					}
				}
				if x == nil {
					x = ctx.push(la.get())
				}
				if x != nil {
					return tseitinNo, x
				}
				ite = a[2]
				continue
			}
		}
		if x = errors.Join(c.assignLit(lb, a[1], false), c.assignLit(lc, a[2], false)); x != nil {
			return tseitinNo, x
		}
		if root {
			x = errors.Join(cls(nla.get(), lb.get()), cls(la.get(), lc.get()))
		} else {
			if x = errors.Join(c.assignInv(nlb, lb.get()), c.assignInv(nlc, lc.get())); x != nil {
				return tseitinNo, x
			}
			x = errors.Join(cls(nla.get(), lb.get(), nk.get()), cls(la.get(), lc.get(), nk.get()), cls(nla.get(), nlb.get(), k.get()), cls(la.get(), nlc.get(), k.get()))
			if x == nil && c.owner.iteExtra {
				x = errors.Join(cls(lb.get(), lc.get(), nk.get()), cls(nlb.get(), nlc.get(), k.get()))
				if x == nil {
					x = errors.Join(exNeg.push(lb.get()), exNeg.push(lc.get()), exNeg.push(nk.get()))
				}
				if x == nil {
					x = c.clause(exNeg.handles()...)
				}
				if x == nil {
					x = errors.Join(exPos.push(nlb.get()), exPos.push(nlc.get()), exPos.push(k.get()))
				}
				if x == nil {
					x = c.clause(exPos.handles()...)
				}
			}
		}
		return tseitinDone, x
	}
}

func (c *tseitinInvocation) distributiveArg(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	n, yes, e := c.notArg(h)
	if e != nil || !yes {
		return nil, false, e
	}
	a, yes, e := c.isOp(n, ast.ASTOpOr)
	if e != nil || !yes {
		return nil, false, e
	}
	sh, e := c.shared(n)
	return a, !sh, e
}
func (c *tseitinInvocation) distributiveChoices(args []ast.ASTHandle, visit bool) ([][]ast.ASTHandle, bool, error) {
	choices := make([][]ast.ASTHandle, len(args))
	blowup := uint32(1)
	used := false
	for i, a := range args {
		inner, yes, e := c.distributiveArg(a)
		if e != nil {
			return nil, false, e
		}
		n := uint32(len(inner))
		accept := c.owner.distributivity && yes && n < c.owner.distributivityBlowup && blowup*n < c.owner.distributivityBlowup && blowup < blowup*n
		if accept {
			used = true
			blowup *= n
			choices[i] = append([]ast.ASTHandle{}, inner...)
			if visit {
				continue
			}
		}
		if !accept {
			choices[i] = []ast.ASTHandle{a}
		}
	}
	return choices, used, nil
}
func (c *tseitinInvocation) matchOr(t ast.ASTHandle, first, root bool) (_ tseitinMatch, err error) {
	args, yes, e := c.isOp(t, ast.ASTOpOr)
	if e != nil || !yes {
		return tseitinNo, e
	}
	if first {
		choices, _, e := c.distributiveChoices(args, true)
		if e != nil {
			return tseitinNo, e
		}
		visited := true
		for i, a := range args {
			if len(choices[i]) > 1 {
				for _, x := range choices[i] {
					if e = c.visit(x, &visited, false); e != nil {
						return tseitinNo, e
					}
				}
			} else if e = c.visit(a, &visited, false); e != nil {
				return tseitinNo, e
			}
		}
		if !visited {
			return tseitinContinue, nil
		}
	}
	k, nk := c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nk, k)) }()
	if !root {
		if e = c.assignFresh(k, nk); e != nil {
			return tseitinNo, e
		}
		if e = c.cacheResult(t, k.get()); e != nil {
			return tseitinNo, e
		}
	}
	blowupLimit := c.owner.distributivityBlowup
	distributivity := false
	if c.owner.distributivity {
		for _, a := range args {
			inner, yes, x := c.distributiveArg(a)
			if x != nil {
				return tseitinNo, x
			}
			if yes && uint32(len(inner)) < blowupLimit {
				distributivity = true
				break
			}
		}
	}
	if !distributivity {
		lits, l := c.newBuffer(), c.newRef()
		defer func() { err = errors.Join(err, l.release(), lits.release()) }()
		for _, a := range args {
			if e = c.assignLit(l, a, false); e != nil {
				return tseitinNo, e
			}
			if e = lits.push(l.get()); e != nil {
				return tseitinNo, e
			}
		}
		if root {
			e = c.clause(lits.handles()...)
		} else {
			for _, h := range lits.handles() {
				if e = c.assignInv(l, h); e != nil {
					return tseitinNo, e
				}
				if e = c.clause(l.get(), k.get()); e != nil {
					return tseitinNo, e
				}
			}
			if e = lits.push(nk.get()); e == nil {
				e = c.clause(lits.handles()...)
			}
		}
		if e == nil {
			e = errors.Join(l.release(), lits.release())
		}
		if e == nil {
			e = releaseRefs(nk, k)
		}
		return tseitinDone, e
	}
	buffer, l, nl := c.newBuffer(), c.newRef(), c.newRef()
	defer func() { err = errors.Join(err, releaseRefs(nl, l), buffer.release()) }()
	sizes := make([]int, 0, len(args))
	offsets := make([]int, 0, len(args))
	blowup := uint32(1)
	for _, a := range args {
		offsets = append(offsets, len(buffer.hs))
		inner, expandable, x := c.distributiveArg(a)
		if x != nil {
			return tseitinNo, x
		}
		n := uint32(len(inner))
		if expandable && n < blowupLimit && blowup*n < blowupLimit && blowup < blowup*n {
			sizes = append(sizes, len(inner))
			blowup *= n
			lits := c.newBuffer()
			for _, b := range inner {
				if x = c.assignLit(nl, b, true); x == nil {
					x = buffer.push(nl.get())
				}
				if x == nil && !root {
					if x = c.assignInv(l, nl.get()); x == nil {
						x = lits.push(l.get())
					}
				}
				if x != nil {
					return tseitinNo, errors.Join(x, lits.release())
				}
			}
			if !root {
				if x = lits.push(k.get()); x == nil {
					x = c.clause(lits.handles()...)
				}
			}
			if x = errors.Join(x, lits.release()); x != nil {
				return tseitinNo, x
			}
			continue
		}
		sizes = append(sizes, 1)
		if x = c.assignLit(l, a, false); x == nil {
			x = buffer.push(l.get())
		}
		if x == nil && !root {
			if x = c.assignInv(nl, l.get()); x == nil {
				x = c.clause(nl.get(), k.get())
			}
		}
		if x != nil {
			return tseitinNo, x
		}
	}
	// product_iterator_next over the per-argument choices.
	indices := make([]int, len(sizes))
	for {
		clause := make([]ast.ASTHandle, 0, len(sizes)+1)
		for i := range sizes {
			clause = append(clause, buffer.hs[offsets[i]+indices[i]])
		}
		if !root {
			clause = append(clause, nk.get())
		}
		if e = c.clause(clause...); e != nil {
			return tseitinNo, e
		}
		pos := 0
		for ; pos < len(indices); pos++ {
			indices[pos]++
			if indices[pos] < sizes[pos] {
				break
			}
			indices[pos] = 0
		}
		if pos == len(indices) {
			break
		}
	}
	e = errors.Join(releaseRefs(nl, l), buffer.release())
	if e == nil {
		e = releaseRefs(nk, k)
	}
	return tseitinDone, e
}

func (c *tseitinInvocation) process(root ast.ASTHandle, dep ast.ASTDependency) error {
	c.currDep = dep
	visited := true
	if e := c.visit(root, &visited, true); e != nil {
		return e
	}
	if visited {
		l := c.newRef()
		if e := c.assignLit(l, root, false); e != nil {
			return errors.Join(e, l.release())
		}
		return errors.Join(c.clause(l.get()), l.release())
	}
	for len(c.frames) > 0 {
		if e := c.checkpoint(); e != nil {
			return e
		}
		i := len(c.frames) - 1
		f := c.frames[i]
		c.frames[i].first = false
		matchers := []func(ast.ASTHandle, bool, bool) (tseitinMatch, error){c.matchOr3And, c.matchOr, c.matchIff3, c.matchIff, c.matchIte, c.matchNot}
		matched := false
		for _, matcher := range matchers {
			r, e := matcher(f.term, f.first, f.term == root)
			if e != nil {
				return e
			}
			if r == tseitinContinue {
				matched = true
				break
			}
			if r == tseitinDone {
				c.frames = c.frames[:i]
				matched = true
				break
			}
		}
		if !matched {
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (t *TseitinCNFTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if sorted, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("tseitin-cnf", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = tactic.FailIfProofGeneration("tseitin-cnf", goal); err != nil {
		return err
	}
	inv, e := newTseitinInvocation(t)
	if e != nil {
		return e
	}
	defer func() { t.numAux += inv.numAux; err = errors.Join(err, inv.close()) }()
	if inv.models, e = goal.ModelsEnabled(); e != nil {
		return e
	}
	if inv.cores, e = goal.UnsatCoresEnabled(); e != nil {
		return e
	}
	if e = inv.occs.Collect(goal); e != nil {
		return e
	}
	size, e := goal.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		form, x := goal.Form(i)
		if x != nil {
			return x
		}
		dep, x := goal.Dependency(i)
		if x != nil {
			return x
		}
		if x = inv.process(form, dep); x != nil {
			return x
		}
	}
	if e = goal.Reset(); e != nil {
		return e
	}
	added := make(map[ast.ASTHandle]bool, len(inv.clauses))
	for _, row := range inv.clauses {
		h, x := t.manager.RefGet(row.ref)
		if x != nil {
			return x
		}
		if added[h] {
			continue
		}
		added[h] = true
		if x = goal.AssertWithProof(h, ast.ASTHandle{}, row.dep); x != nil {
			return x
		}
	}
	if inv.models && len(inv.fresh) > 0 {
		filter, x := tactic.NewFilterModel(t.manager)
		if x != nil {
			return x
		}
		if x = filter.IncRef(); x != nil {
			return errors.Join(x, filter.Close())
		}
		defer func() { err = errors.Join(err, filter.DecRef()) }()
		for _, v := range inv.fresh {
			d, y := t.manager.GetDecl(v)
			if y != nil {
				return y
			}
			if y = filter.Insert(d); y != nil {
				return y
			}
		}
		if x = result.SetModelConverter(filter); x != nil {
			return x
		}
	}
	if e = goal.IncDepth(); e != nil {
		return e
	}
	if e = result.AddGoal(goal); e != nil {
		return e
	}
	return tactic.ReportTacticProgress(":cnf-aux-vars", t.numAux+inv.numAux)
}
