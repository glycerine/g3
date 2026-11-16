package rel

// Source: src/muz/rel/check_relation.h/.cpp check_relation and
// check_relation_plugin: every operation runs on the base plugin's
// relation and its result's formula is checked against the formula the
// operation should produce, with an SMT kernel (check_equiv).

import (
	"errors"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// astScope holds the C++ locals of one function or full-expression: the
// expr_refs, expr_ref_vectors and var_substs it declares, destroyed in
// reverse declaration order by close, as the C++ destructors run.
type astScope struct {
	m     *ast.ASTManager
	items []astScopeItem
	err   error
}

type astScopeItem struct {
	ref    ast.ASTRef
	vec    *astVec
	closer io.Closer
}

// astVec is an expr_ref_vector local.
type astVec struct {
	hs []ast.ASTHandle
}

// child opens the scope of a callee's locals or of a full-expression's
// temporaries; a failure already recorded in s carries over.
func (s *astScope) child() *astScope {
	return &astScope{m: s.m, err: s.err}
}

// end closes the child scope c (the end of a C++ function body or
// full-expression) and folds its failure into s.
func (s *astScope) end(c *astScope) {
	s.fail(c.close(nil))
}

func (s *astScope) fail(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

// local declares an expr_ref local (null).
func (s *astScope) local() ast.ASTRef {
	ref, err := s.m.NewRef(ast.ASTHandle{})
	if err != nil {
		s.fail(err)
		return ast.ASTRef{}
	}
	s.items = append(s.items, astScopeItem{ref: ref})
	return ref
}

// set is the expr_ref assignment r = h.
func (s *astScope) setRef(r ast.ASTRef, h ast.ASTHandle, err error) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	if err = s.m.SetRef(r, h); err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	return h
}

// get is the expr* of r.
func (s *astScope) get(r ast.ASTRef) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	h, err := s.m.RefGet(r)
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	return h
}

// take assigns the formula ref (an out-parameter's value) to r and
// releases ref: to_formula(r).
func (s *astScope) takeRef(r ast.ASTRef, ref ast.ASTRef, err error) ast.ASTHandle {
	if err != nil {
		s.fail(err)
		return ast.ASTHandle{}
	}
	if s.err != nil {
		_ = s.m.DelRef(&ref)
		return ast.ASTHandle{}
	}
	h, err := s.m.RefGet(ref)
	if err == nil {
		err = s.m.SetRef(r, h)
	}
	err = errors.Join(err, s.m.DelRef(&ref))
	if err != nil {
		s.fail(err)
		return ast.ASTHandle{}
	}
	return h
}

// holdRef declares an expr_ref local that owns ref.
func (s *astScope) holdRef(ref ast.ASTRef, err error) ast.ASTHandle {
	if s.err != nil {
		if err == nil {
			_ = s.m.DelRef(&ref)
		}
		return ast.ASTHandle{}
	}
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	s.items = append(s.items, astScopeItem{ref: ref})
	return s.get(ref)
}

// vector declares an expr_ref_vector local.
func (s *astScope) vector() *astVec {
	v := &astVec{}
	s.items = append(s.items, astScopeItem{vec: v})
	return v
}

// push is v.push_back(h).
func (s *astScope) pushVec(v *astVec, h ast.ASTHandle, err error) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	if err = s.m.IncRef(h); err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	v.hs = append(v.hs, h)
	return h
}

// varSubst declares a var_subst sub(m, false) local.
func (s *astScope) varSubst() *astrewriter.VarSubstitution {
	if s.err != nil {
		return nil
	}
	sub, err := astrewriter.NewVarSubstitution(s.m, false)
	if err != nil {
		s.err = err
		return nil
	}
	s.items = append(s.items, astScopeItem{closer: sub})
	return sub
}

// subst is sub(fml, vars.size(), vars.c_ptr(), out).
func (s *astScope) subst(sub *astrewriter.VarSubstitution, fml ast.ASTHandle, vars *astVec, out ast.ASTRef) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	if err := sub.Substitute(fml, vars.hs, out); err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	return s.get(out)
}

// astSlot is an expr_ref of a scope, the target of an assignment.
type astSlot struct {
	s *astScope
	r ast.ASTRef
}

func (s *astScope) at(r ast.ASTRef) astSlot { return astSlot{s, r} }

// set is the expr_ref assignment r = h.
func (x astSlot) set(h ast.ASTHandle, err error) ast.ASTHandle { return x.s.setRef(x.r, h, err) }

// take is to_formula(r): the formula ref is assigned to r and released.
func (x astSlot) take(ref ast.ASTRef, err error) ast.ASTHandle { return x.s.takeRef(x.r, ref, err) }

// astPushTarget is an expr_ref_vector of a scope.
type astPushTarget struct {
	s *astScope
	v *astVec
}

func (s *astScope) to(v *astVec) astPushTarget { return astPushTarget{s, v} }

// push is v.push_back(h).
func (x astPushTarget) push(h ast.ASTHandle, err error) ast.ASTHandle {
	return x.s.pushVec(x.v, h, err)
}

func (s *astScope) close(result error) error {
	for i := len(s.items) - 1; i >= 0; i-- {
		it := &s.items[i]
		switch {
		case it.vec != nil:
			n, err := s.m.DecRefEach(it.vec.hs)
			if err != nil {
				n++
			}
			it.vec.hs = it.vec.hs[n:]
			result = errors.Join(result, err)
		case it.closer != nil:
			result = errors.Join(result, it.closer.Close())
		default:
			result = errors.Join(result, s.m.DelRef(&it.ref))
		}
	}
	s.items = nil
	return errors.Join(result, s.err)
}

// CheckRelationPlugin is check_relation_plugin.
type CheckRelationPlugin struct {
	RelationPluginDefaults
	rm   *RelationManager
	m    *ast.ASTManager
	base RelationPlugin // m_base
}

var _ RelationPlugin = (*CheckRelationPlugin)(nil)

func NewCheckRelationPlugin(rm *RelationManager) (*CheckRelationPlugin, error) {
	if err := rm.open(); err != nil {
		return nil, err
	}
	return &CheckRelationPlugin{rm: rm, m: rm.manager}, nil
}

func (*CheckRelationPlugin) Name() string                { return "check_relation" }
func (p *CheckRelationPlugin) Manager() *RelationManager { return p.rm }

// SetPlugin is set_plugin(p).
func (p *CheckRelationPlugin) SetPlugin(base RelationPlugin) { p.base = base }

// CheckRelation is check_relation.
type CheckRelation struct {
	plugin   *CheckRelationPlugin
	sorts    []ast.ASTHandle
	relation RelationBase // m_relation
	fml      ast.ASTRef   // m_fml
	closed   bool
}

var _ RelationBase = (*CheckRelation)(nil)

func checkGet(r RelationBase) (*CheckRelation, bool) {
	c, ok := r.(*CheckRelation)
	return c, ok
}

// newCheckRelation is check_relation::check_relation(p, sig, r).
func (p *CheckRelationPlugin) newCheckRelation(sorts []ast.ASTHandle, r RelationBase) (*CheckRelation, error) {
	fml, err := r.ToFormula()
	if err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return &CheckRelation{plugin: p, sorts: append([]ast.ASTHandle(nil), sorts...), relation: r, fml: fml}, nil
}

func (r *CheckRelation) fmlHandle() (ast.ASTHandle, error) { return r.plugin.m.RefGet(r.fml) }

// setFml is m_fml = f (the expr_ref assignment).
func (r *CheckRelation) setFml(f ast.ASTHandle) error {
	return r.plugin.m.SetRef(r.fml, f)
}

// refreshFml is r.rb().to_formula(r.m_fml).
func (r *CheckRelation) refreshFml() error {
	f, err := r.relation.ToFormula()
	if err != nil {
		return err
	}
	h, err := r.plugin.m.RefGet(f)
	if err == nil {
		err = r.setFml(h)
	}
	return errors.Join(err, r.plugin.m.DelRef(&f))
}

func (r *CheckRelation) RelationSignature() []ast.ASTHandle { return r.sorts }
func (r *CheckRelation) RelationPlugin() RelationPlugin     { return r.plugin }

// Base is rb().
func (r *CheckRelation) Base() RelationBase { return r.relation }

func (r *CheckRelation) Signature() (Signature, error) { return r.relation.Signature() }
func (r *CheckRelation) Facts() ([]Fact, error)        { return nil, ErrRelationNotTable }
func (r *CheckRelation) Len() (int, error)             { return r.relation.Len() }
func (r *CheckRelation) SizeEstimateRows() (uint32, error) {
	return r.relation.SizeEstimateRows()
}

// SizeEstimateBytes is relation_base's UINT_MAX (check_relation does not
// override get_size_estimate_bytes).
func (r *CheckRelation) SizeEstimateBytes() (uint32, error) { return ^uint32(0), nil }
func (r *CheckRelation) IsPrecise() bool                    { return r.relation.IsPrecise() }

// Close is ~check_relation: m_relation->deallocate().
func (r *CheckRelation) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	return errors.Join(r.relation.Close(), r.plugin.m.DelRef(&r.fml))
}

// mkEq is check_relation::mk_eq(f): its expr_ref result is built into out
// before the local conjs vector dies.
func (r *CheckRelation) mkEq(s *astScope, values []ast.ASTHandle, out ast.ASTRef) ast.ASTHandle {
	m := r.plugin.m
	c := s.child()
	conjs := c.vector()
	for i, sort := range r.sorts {
		if c.err != nil {
			break
		}
		v, err := m.MkVar(uint32(i), sort)
		if err != nil {
			c.fail(err)
			break
		}
		c.to(conjs).push(m.MkEq(v, values[i]))
	}
	var h ast.ASTHandle
	if c.err == nil {
		h = c.at(out).set(udocMakeAnd(m, conjs.hs))
	}
	s.end(c)
	return h
}

// factValues decodes the table elements of f to relation elements.
func (r *CheckRelation) factValues(s *astScope, f Fact) []ast.ASTHandle {
	values := make([]ast.ASTHandle, len(f))
	for i, e := range f {
		values[i] = s.holdRef(r.plugin.rm.DecodeValue(r.sorts[i], e))
	}
	return values
}

// Add is check_relation::add_fact.
func (r *CheckRelation) Add(f Fact) (_ bool, result error) {
	p := r.plugin
	m := p.m
	s := &astScope{m: m}
	defer func() { result = s.close(result) }()
	values := r.factValues(s, f)
	if s.err != nil {
		return false, nil
	}
	fml1 := s.local()
	inserted, err := r.relation.Add(f)
	if err != nil {
		return false, err
	}
	s.at(fml1).take(r.relation.ToFormula())
	// m_fml = m.mk_or(m_fml, mk_eq(f)): the mk_eq temporary dies after
	// the assignment.
	t := s.child()
	eq := r.mkEq(t, values, t.local())
	if t.err == nil {
		t.at(r.fml).set(m.MkOr(t.get(r.fml), eq))
	}
	s.end(t)
	// check_equiv("add_fact", ground(m_fml), ground(fml1)).
	t = s.child()
	g1 := p.ground(t, r, t.get(r.fml), t.local())
	g2 := p.ground(t, r, t.get(fml1), t.local())
	if t.err == nil {
		t.fail(p.checkEquiv("add_fact", g1, g2))
	}
	s.end(t)
	s.at(r.fml).set(s.get(fml1), nil)
	return inserted, nil
}

// Empty is check_relation::empty.
func (r *CheckRelation) Empty() (bool, error) {
	result, err := r.relation.Empty()
	if err != nil {
		return false, err
	}
	return result, r.checkEmpty(result, "empty")
}

// checkEmpty is "if (result && !m.is_false(m_fml))
// check_equiv(objective, m.mk_false(), ground(m_fml))".
func (r *CheckRelation) checkEmpty(result bool, objective string) error {
	m := r.plugin.m
	fml, err := r.fmlHandle()
	if err != nil {
		return err
	}
	if !result || m.IsFalse(fml) {
		return nil
	}
	f, err := m.MkFalse()
	if err != nil {
		return err
	}
	s := &astScope{m: m}
	g := r.plugin.ground(s, r, fml, s.local())
	if s.err == nil {
		s.fail(r.plugin.checkEquiv(objective, f, g))
	}
	return s.close(nil)
}

// FastEmpty is check_relation::fast_empty. Its check_equiv cannot fail
// for a correct base relation; a failure panics like the uncaught throw.
func (r *CheckRelation) FastEmpty() bool {
	result := r.relation.FastEmpty()
	if err := r.checkEmpty(result, "fast_empty"); err != nil {
		panic(err)
	}
	return result
}

// Reset is check_relation::reset.
func (r *CheckRelation) Reset() error {
	if err := r.relation.Reset(); err != nil {
		return err
	}
	f, err := r.plugin.m.MkFalse()
	if err != nil {
		return err
	}
	return r.setFml(f)
}

// Contains is check_relation::contains_fact.
func (r *CheckRelation) Contains(f Fact) (_ bool, resultErr error) {
	p := r.plugin
	m := p.m
	result, err := r.relation.Contains(f)
	if err != nil {
		return false, err
	}
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	values := r.factValues(s, f)
	fml1, fml2 := s.local(), s.local()
	t := s.child()
	t.at(fml1).set(r.mkEq(t, values, t.local()), nil)
	s.end(t)
	if s.err != nil {
		return false, nil
	}
	s.at(fml2).set(m.MkAnd(s.get(r.fml), s.get(fml1)))
	if s.err != nil {
		return false, nil
	}
	if result {
		t = s.child()
		g1 := p.ground(t, r, t.get(fml1), t.local())
		g2 := p.ground(t, r, t.get(fml2), t.local())
		if t.err == nil {
			t.fail(p.checkEquiv("contains fact", g1, g2))
		}
		s.end(t)
	} else if !m.IsFalse(s.get(r.fml)) {
		t = s.child()
		g2 := p.ground(t, r, t.get(fml2), t.local())
		if t.err == nil {
			f, err := m.MkFalse()
			if err == nil {
				err = p.checkEquiv("contains fact", g2, f)
			}
			t.fail(err)
		}
		s.end(t)
	}
	return result, nil
}

// CloneRelation is check_relation::clone.
func (r *CheckRelation) CloneRelation() (RelationBase, error) {
	return r.clone()
}

func (r *CheckRelation) clone() (_ *CheckRelation, resultErr error) {
	p := r.plugin
	// mk_empty(get_signature()), then its relation is replaced by a clone.
	empty, err := p.MkEmpty(r.sorts)
	if err != nil {
		return nil, err
	}
	result, _ := checkGet(empty)
	if err = result.relation.Close(); err != nil {
		result.closed = true
		return nil, errors.Join(err, p.m.DelRef(&result.fml))
	}
	inner, err := r.relation.CloneRelation()
	if err != nil {
		result.closed = true
		return nil, errors.Join(err, p.m.DelRef(&result.fml))
	}
	result.relation = inner
	if err = result.refreshFml(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	a, err := r.fmlHandle()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	b, err := result.fmlHandle()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if a != b {
		s := &astScope{m: p.m}
		g1 := p.ground(s, r, a, s.local())
		g2 := p.ground(s, r, b, s.local())
		if s.err == nil {
			s.fail(p.checkEquiv("clone", g1, g2))
		}
		if err = s.close(nil); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

func (r *CheckRelation) CloneFinite() (FiniteTable, error) { return r.clone() }

// ToFormula is check_relation::to_formula: m_fml.
func (r *CheckRelation) ToFormula() (ast.ASTRef, error) {
	return r.plugin.m.CopyRef(r.fml)
}

// Display is check_relation::display.
func (r *CheckRelation) Display(out io.Writer) error {
	if err := r.relation.Display(out); err != nil {
		return err
	}
	fml, err := r.fmlHandle()
	if err != nil {
		return err
	}
	text, err := relMkPP(r.plugin.m, fml)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text+"\n")
	return err
}

// ---------------------------------------------------------------------

// CanHandleSignature is check_relation_plugin::can_handle_signature.
func (p *CheckRelationPlugin) CanHandleSignature(sorts []ast.ASTHandle) (bool, error) {
	if p.base == nil {
		return false, nil
	}
	return p.base.CanHandleSignature(sorts)
}

// MkEmpty is check_relation_plugin::mk_empty.
func (p *CheckRelationPlugin) MkEmpty(sorts []ast.ASTHandle) (RelationBase, error) {
	r, err := p.base.MkEmpty(sorts)
	if err != nil {
		return nil, err
	}
	result, err := p.newCheckRelation(sorts, r)
	if err != nil {
		return nil, err
	}
	return result, p.checkConstant(result, false, "mk_empty")
}

// MkFull is check_relation_plugin::mk_full.
func (p *CheckRelationPlugin) MkFull(predicate ast.ASTHandle, sorts []ast.ASTHandle) (RelationBase, error) {
	r, err := p.base.MkFull(predicate, sorts)
	if err != nil {
		return nil, err
	}
	result, err := p.newCheckRelation(sorts, r)
	if err != nil {
		return nil, err
	}
	return result, p.checkConstant(result, true, "mk_full")
}

// checkConstant is "if (result->m_fml != m.mk_false()/mk_true())
// check_equiv(objective, result->ground(result->m_fml), constant)".
func (p *CheckRelationPlugin) checkConstant(r *CheckRelation, value bool, objective string) (resultErr error) {
	m := p.m
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, r.Close())
		}
	}()
	var c ast.ASTHandle
	var err error
	if value {
		c, err = m.MkTrue()
	} else {
		c, err = m.MkFalse()
	}
	if err != nil {
		return err
	}
	fml, err := r.fmlHandle()
	if err != nil {
		return err
	}
	if fml == c {
		return nil
	}
	s := &astScope{m: m}
	g := p.ground(s, r, fml, s.local())
	if s.err == nil {
		s.fail(p.checkEquiv(objective, g, c))
	}
	return s.close(nil)
}

// ground is check_relation_plugin::ground(dst, fml): its result is
// built into out before the locals vars and sub die.
func (p *CheckRelationPlugin) ground(s *astScope, dst RelationBase, fml ast.ASTHandle, out ast.ASTRef) ast.ASTHandle {
	g := s.child()
	sub := g.varSubst()
	vars := g.vector()
	for i, sort := range dst.RelationSignature() {
		if g.err != nil {
			break
		}
		g.to(vars).push(p.m.MkConst(symbol.Numerical(uint32(i)), sort))
	}
	h := g.subst(sub, fml, vars, out)
	s.end(g)
	return h
}

// groundRelation is check_relation_plugin::ground(dst).
func (p *CheckRelationPlugin) groundRelation(s *astScope, dst RelationBase, out ast.ASTRef) ast.ASTHandle {
	g := s.child()
	fml := g.local()
	f := g.at(fml).take(dst.ToFormula())
	h := ast.ASTHandle{}
	if g.err == nil {
		h = p.ground(g, dst, f, out)
	}
	s.end(g)
	return h
}

// checkEquiv is check_relation_plugin::check_equiv: the local tmp dies
// before the smt::kernel.
func (p *CheckRelationPlugin) checkEquiv(objective string, fml1, fml2 ast.ASTHandle) error {
	m := p.m
	if err := m.EmitTrace("doc", "check_equiv", "../src/muz/rel/check_relation.cpp", 420, func(out io.Writer) error {
		t1, err := relMkPP(m, fml1)
		if err != nil {
			return err
		}
		t2, err := relMkPP(m, fml2)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, t1+"\n"+t2+"\n")
		return err
	}); err != nil {
		return err
	}
	kernel, err := smtcore.NewDefaultParamsKernel(m)
	if err != nil {
		return err
	}
	s := &astScope{m: m}
	tmp := s.local()
	if s.err == nil {
		eq, err := m.MkEq(fml1, fml2)
		if err != nil {
			s.fail(err)
		} else {
			s.at(tmp).set(m.MkNot(eq))
		}
	}
	if s.err == nil {
		s.fail(p.checkEquivSolve(kernel, objective, s.get(tmp), fml1, fml2))
	}
	return errors.Join(s.close(nil), kernel.Close())
}

func (p *CheckRelationPlugin) checkEquivSolve(kernel *smtcore.DefaultParamsKernel, objective string, tmp, fml1, fml2 ast.ASTHandle) error {
	m := p.m
	if err := kernel.Assert(tmp, ast.ASTHandle{}); err != nil {
		return err
	}
	res, err := kernel.Check()
	if err != nil {
		return err
	}
	switch res {
	case lbool.False:
		if diagnostic.VerboseProgress {
			if err = diagnostic.ProgressVerbose(3, objective+" verified\n"); err != nil {
				return err
			}
		}
	case lbool.True:
		if diagnostic.VerboseProgress {
			t1, err1 := relMkPP(m, fml1)
			t2, err2 := relMkPP(m, fml2)
			if err = errors.Join(err1, err2); err != nil {
				return err
			}
			if err = diagnostic.ProgressVerbose(0, "NOT verified true\n"+t1+"\n"+t2+"\n"); err != nil {
				return err
			}
		}
		return z3exception.NewDefaultException("operation was not verified")
	}
	return nil
}

// checkContains is check_relation_plugin::check_contains.
func (p *CheckRelationPlugin) checkContains(objective string, fml1, fml2 ast.ASTHandle) error {
	s := &astScope{m: p.m}
	fml0 := s.local()
	s.at(fml0).set(p.m.MkAnd(fml1, fml2))
	if s.err == nil {
		s.fail(p.checkEquiv(objective, s.get(fml0), fml2))
	}
	return s.close(nil)
}

// mkProject is check_relation_plugin::mk_project; out is its named
// result fml1.
func (p *CheckRelationPlugin) mkProject(s *astScope, sorts []ast.ASTHandle, fml ast.ASTHandle, removed []int, out ast.ASTRef) ast.ASTHandle {
	m := p.m
	g := s.child()
	var bound []ast.ASTHandle
	var names []symbol.Symbol
	vars := g.vector()
	rmCnt := len(removed)
	for i, j, k := 0, 0, 0; i < len(sorts) && g.err == nil; i++ {
		if j < rmCnt && removed[j] == i {
			name, err := m.Symbols.Intern("x" + utoa(uint64(j)))
			if err != nil {
				g.fail(err)
				break
			}
			bound = append(bound, sorts[i])
			names = append(names, name)
			g.to(vars).push(m.MkVar(uint32(j), sorts[i]))
			j++
		} else {
			g.to(vars).push(m.MkVar(uint32(k+rmCnt), sorts[i]))
			k++
		}
	}
	sub := g.varSubst()
	h := g.subst(sub, fml, vars, out)
	for i, j := 0, len(bound)-1; i < j; i, j = i+1, j-1 {
		bound[i], bound[j] = bound[j], bound[i]
	}
	if len(bound) != 0 && g.err == nil {
		h = g.at(out).set(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: bound, Names: names, Body: h}))
	}
	// mk_quantifier with no bound variables returns the body.
	s.end(g)
	return h
}

// mkJoin is check_relation_plugin::mk_join; out is its named result fml1.
func (p *CheckRelationPlugin) mkJoin(s *astScope, t1, t2 RelationBase, cols1, cols2 []int, out ast.ASTRef) ast.ASTHandle {
	m := p.m
	g := s.child()
	sig1, sig2 := t1.RelationSignature(), t2.RelationSignature()
	fml2 := g.local()
	var1, var2 := g.local(), g.local()
	g.at(out).take(t1.ToFormula())
	g.at(fml2).take(t2.ToFormula())
	sub := g.varSubst()
	vars := g.vector()
	for i, sort := range sig2 {
		if g.err != nil {
			break
		}
		g.to(vars).push(m.MkVar(uint32(i+len(sig1)), sort))
	}
	f2 := g.subst(sub, g.get(fml2), vars, fml2)
	var f1 ast.ASTHandle
	if g.err == nil {
		f1 = g.at(out).set(m.MkAnd(g.get(out), f2))
	}
	for i := 0; i < len(cols1) && g.err == nil; i++ {
		v1, v2 := cols1[i], cols2[i]
		a := g.at(var1).set(m.MkVar(uint32(v1), sig1[v1]))
		b := g.at(var2).set(m.MkVar(uint32(v2+len(sig1)), sig2[v2]))
		if g.err != nil {
			break
		}
		eq, err := m.MkEq(a, b)
		if err != nil {
			g.fail(err)
			break
		}
		f1 = g.at(out).set(m.MkAnd(eq, f1))
	}
	s.end(g)
	return f1
}

// assign is "r = f(...)" for a callee f returning expr_ref: the callee's
// result temporary dies at the end of the statement, after the assignment.
func (s *astScope) assign(r ast.ASTRef, f func(t *astScope, tmp ast.ASTRef) ast.ASTHandle) ast.ASTHandle {
	t := s.child()
	h := f(t, t.local())
	t.setRef(r, h, nil)
	s.end(t)
	return h
}

// verifyJoin is check_relation_plugin::verify_join.
func (p *CheckRelationPlugin) verifyJoin(t1, t2, t RelationBase, cols1, cols2 []int) error {
	s := &astScope{m: p.m}
	// expr_ref fml1 = ground(t, mk_join(...)): the mk_join temporary dies
	// once ground has built fml1.
	fml1 := s.local()
	j := s.child()
	joined := p.mkJoin(j, t1, t2, cols1, cols2, j.local())
	if j.err == nil {
		p.ground(j, t, joined, fml1)
	}
	s.end(j)
	fml2 := s.local()
	if s.err == nil {
		p.groundRelation(s, t, fml2)
	}
	if s.err == nil {
		s.fail(p.checkEquiv("join", s.get(fml1), s.get(fml2)))
	}
	return s.close(nil)
}

// verifyJoinProject is check_relation_plugin::verify_join_project.
func (p *CheckRelationPlugin) verifyJoinProject(t1, t2, t RelationBase, cols1, cols2, removed []int) error {
	s := &astScope{m: p.m}
	sig1 := joinSorts(t1.RelationSignature(), t2.RelationSignature())
	fml1 := s.local()
	p.mkJoin(s, t1, t2, cols1, cols2, fml1)
	s.assign(fml1, func(x *astScope, tmp ast.ASTRef) ast.ASTHandle {
		return p.mkProject(x, sig1, x.get(fml1), removed, tmp)
	})
	s.assign(fml1, func(x *astScope, tmp ast.ASTRef) ast.ASTHandle {
		return p.ground(x, t, x.get(fml1), tmp)
	})
	fml2 := s.local()
	s.at(fml2).take(t.ToFormula())
	s.assign(fml2, func(x *astScope, tmp ast.ASTRef) ast.ASTHandle {
		return p.ground(x, t, x.get(fml2), tmp)
	})
	if s.err == nil {
		s.fail(p.checkEquiv("join_project", s.get(fml1), s.get(fml2)))
	}
	return s.close(nil)
}

// verifyProjectFormulas is verify_project(src, f1, dst, f2, removed).
func (p *CheckRelationPlugin) verifyProjectFormulas(src RelationBase, f1 ast.ASTHandle, dst RelationBase, f2 ast.ASTHandle, removed []int) error {
	s := &astScope{m: p.m}
	// expr_ref fml1 = ground(dst, mk_project(...)): the mk_project
	// temporary dies once ground has built fml1.
	fml1 := s.local()
	j := s.child()
	projected := p.mkProject(j, src.RelationSignature(), f1, removed, j.local())
	if j.err == nil {
		p.ground(j, dst, projected, fml1)
	}
	s.end(j)
	fml2 := s.local()
	if s.err == nil {
		p.ground(s, dst, f2, fml2)
	}
	if s.err == nil {
		s.fail(p.checkEquiv("project", s.get(fml1), s.get(fml2)))
	}
	return s.close(nil)
}

// verifyProject is verify_project(src, dst, removed).
func (p *CheckRelationPlugin) verifyProject(src, dst RelationBase, removed []int) error {
	s := &astScope{m: p.m}
	fml1, fml2 := s.local(), s.local()
	f1 := s.at(fml1).take(src.ToFormula())
	f2 := s.at(fml2).take(dst.ToFormula())
	if s.err == nil {
		s.fail(p.verifyProjectFormulas(src, f1, dst, f2, removed))
	}
	return s.close(nil)
}

// verifyFilterProject is check_relation_plugin::verify_filter_project.
func (p *CheckRelationPlugin) verifyFilterProject(src, dst RelationBase, cond ast.ASTHandle, removed []int) error {
	s := &astScope{m: p.m}
	fml1, fml2 := s.local(), s.local()
	f1 := s.at(fml1).take(src.ToFormula())
	f2 := s.at(fml2).take(dst.ToFormula())
	if s.err == nil {
		f1 = s.at(fml1).set(p.m.MkAnd(cond, f1))
	}
	if s.err == nil {
		s.fail(p.verifyProjectFormulas(src, f1, dst, f2, removed))
	}
	return s.close(nil)
}

// verifyPermutation is check_relation_plugin::verify_permutation.
func (p *CheckRelationPlugin) verifyPermutation(src, dst RelationBase, cycle []int) error {
	m := p.m
	sig1, sig2 := src.RelationSignature(), dst.RelationSignature()
	perm := make([]int, len(sig1))
	for i := range perm {
		perm[i] = i
	}
	for i := range cycle {
		j := (i + 1) % len(cycle)
		perm[cycle[j]] = cycle[i]
	}
	s := &astScope{m: m}
	sub := s.vector()
	for i := 0; i < len(perm) && s.err == nil; i++ {
		s.to(sub).push(m.MkVar(uint32(perm[i]), sig1[i]))
	}
	subst := s.varSubst()
	fml1, fml2 := s.local(), s.local()
	s.at(fml1).take(src.ToFormula())
	s.at(fml2).take(dst.ToFormula())
	s.subst(subst, s.get(fml1), sub, fml1)
	vars := s.vector()
	for i := 0; i < len(sig2) && s.err == nil; i++ {
		s.to(vars).push(m.MkConst(symbol.Numerical(uint32(i)), sig2[i]))
	}
	s.subst(subst, s.get(fml1), vars, fml1)
	s.subst(subst, s.get(fml2), vars, fml2)
	if s.err == nil {
		s.fail(p.checkEquiv("permutation", s.get(fml1), s.get(fml2)))
	}
	return s.close(nil)
}

// namedConstants pushes m.mk_const(symbol("x<i>"), sig[i]) onto vars.
func (p *CheckRelationPlugin) namedConstants(s *astScope, vars *astVec, sorts []ast.ASTHandle) {
	for i, sort := range sorts {
		if s.err != nil {
			return
		}
		name, err := p.m.Symbols.Intern("x" + utoa(uint64(i)))
		if err != nil {
			s.fail(err)
			return
		}
		s.to(vars).push(p.m.MkConst(name, sort))
	}
}

// verifyFilter is check_relation_plugin::verify_filter.
func (p *CheckRelationPlugin) verifyFilter(fml0 ast.ASTHandle, t RelationBase, cond ast.ASTHandle) error {
	m := p.m
	s := &astScope{m: m}
	fml1, fml2 := s.local(), s.local()
	s.at(fml1).set(m.MkAnd(fml0, cond))
	s.at(fml2).take(t.ToFormula())
	vars := s.vector()
	sub := s.varSubst()
	p.namedConstants(s, vars, t.RelationSignature())
	s.subst(sub, s.get(fml1), vars, fml1)
	s.subst(sub, s.get(fml2), vars, fml2)
	if s.err == nil {
		s.fail(p.checkEquiv("filter", s.get(fml1), s.get(fml2)))
	}
	return s.close(nil)
}

// verifyUnion is check_relation_plugin::verify_union.
func (p *CheckRelationPlugin) verifyUnion(dst0 ast.ASTHandle, src, dst RelationBase, delta0 ast.ASTHandle, delta RelationBase) error {
	m := p.m
	s := &astScope{m: m}
	fml1, fml2 := s.local(), s.local()
	s.at(fml1).take(src.ToFormula())
	s.at(fml2).take(dst.ToFormula())
	if s.err == nil {
		s.at(fml1).set(m.MkOr(s.get(fml1), dst0))
	}
	vars := s.vector()
	sub := s.varSubst()
	p.namedConstants(s, vars, dst.RelationSignature())
	s.subst(sub, s.get(fml1), vars, fml1)
	s.subst(sub, s.get(fml2), vars, fml2)
	if s.err == nil {
		s.fail(p.checkEquiv("union", s.get(fml1), s.get(fml2)))
	}
	if delta != nil && s.err == nil {
		b := s.child()
		p.verifyUnionDelta(b, sub, vars, dst0, s.get(fml2), delta0, delta)
		s.end(b)
	}
	return s.close(nil)
}

// verifyUnionDelta is the "if (delta)" block of verify_union, with its
// locals d0, d, fml4 and fml5 in b.
func (p *CheckRelationPlugin) verifyUnionDelta(b *astScope, sub *astrewriter.VarSubstitution, vars *astVec, dst0, fml2, delta0 ast.ASTHandle, delta RelationBase) {
	m := p.m
	d0, d := b.local(), b.local()
	dh := b.at(d).take(delta.ToFormula())
	if b.err != nil {
		return
	}
	if diagnostic.VerboseProgress {
		if enabled, err := diagnostic.ProcessVerboseEnabled(3); err != nil {
			b.fail(err)
			return
		} else if enabled {
			text, err := relMkPP(m, dh)
			if err == nil {
				err = diagnostic.ProgressVerbose(3, "verify delta "+text+"\n")
			}
			if err != nil {
				b.fail(err)
				return
			}
		}
	}
	fml4, fml5 := b.local(), b.local()
	notDst0, err := m.MkNot(dst0)
	if err != nil {
		b.fail(err)
		return
	}
	b.at(fml4).set(m.MkAnd(fml2, notDst0))
	b.subst(sub, b.get(fml4), vars, fml4)
	b.subst(sub, b.get(d), vars, d)
	if b.err == nil {
		b.fail(p.checkContains("union_delta low", b.get(d), b.get(fml4)))
	}
	b.subst(sub, delta0, vars, d0)
	if b.err == nil {
		b.fail(p.checkContains("union delta0", b.get(d), b.get(d0)))
	}
	if b.err == nil {
		b.at(fml4).set(m.MkOr(fml2, delta0))
	}
	if b.err == nil {
		b.at(fml5).set(m.MkOr(b.get(d), dst0))
	}
	b.subst(sub, b.get(fml4), vars, fml4)
	b.subst(sub, b.get(fml5), vars, fml5)
	if b.err == nil {
		b.fail(p.checkEquiv("union no overflow", b.get(fml4), b.get(fml5)))
	}
}

// verifyFilterByNegation is check_relation_plugin::verify_filter_by_negation.
func (p *CheckRelationPlugin) verifyFilterByNegation(dst0 ast.ASTHandle, dst, neg RelationBase, cols1, cols2 []int) error {
	m := p.m
	s := &astScope{m: m}
	sig1, sig2 := dst.RelationSignature(), neg.RelationSignature()
	dstf, negf := s.local(), s.local()
	eqs := s.vector()
	s.at(dstf).take(dst.ToFormula())
	s.at(negf).take(neg.ToFormula())
	if s.err == nil {
		s.to(eqs).push(s.get(negf), nil)
	}
	for i := 0; i < len(cols1) && s.err == nil; i++ {
		// var_ref v1, v2 live for one iteration.
		it := s.child()
		v1, v2 := it.local(), it.local()
		c1, c2 := cols1[i], cols2[i]
		a := it.at(v1).set(m.MkVar(uint32(len(sig2)+c1), sig1[c1]))
		b := it.at(v2).set(m.MkVar(uint32(c2), sig2[c2]))
		if it.err == nil {
			s.to(eqs).push(m.MkEq(a, b))
		}
		s.end(it)
	}
	if s.err == nil {
		s.at(negf).set(udocMakeAnd(m, eqs.hs))
	}
	rev := make([]ast.ASTHandle, len(sig2))
	for i := range sig2 {
		rev[i] = sig2[len(sig2)-1-i]
	}
	names := make([]symbol.Symbol, len(sig2))
	for i := range names {
		names[i] = symbol.Numerical(uint32(i))
	}
	if len(rev) != 0 && s.err == nil {
		s.at(negf).set(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: rev, Names: names, Body: s.get(negf)}))
	}
	if s.err == nil {
		notNeg, err := m.MkNot(s.get(negf))
		if err != nil {
			s.fail(err)
		} else {
			s.at(negf).set(m.MkAnd(dst0, notNeg))
		}
	}
	s.assign(negf, func(x *astScope, tmp ast.ASTRef) ast.ASTHandle {
		return p.ground(x, dst, x.get(negf), tmp)
	})
	s.assign(dstf, func(x *astScope, tmp ast.ASTRef) ast.ASTHandle {
		return p.ground(x, dst, x.get(dstf), tmp)
	})
	if s.err == nil {
		s.fail(p.checkEquiv("filter by negation", s.get(dstf), s.get(negf)))
	}
	return s.close(nil)
}

// --------------------------------------------------------------------
// Operations.

func checkBases(rs ...RelationBase) ([]RelationBase, bool) {
	result := make([]RelationBase, len(rs))
	for i, r := range rs {
		if r == nil {
			continue
		}
		c, ok := checkGet(r)
		if !ok {
			return nil, false
		}
		result[i] = c.relation
	}
	return result, true
}

// MkJoinFn is check_relation_plugin::mk_join_fn / join_fn.
func (p *CheckRelationPlugin) MkJoinFn(t1, t2 RelationBase, cols1, cols2 []int) (RelationJoinFn, error) {
	b, ok := checkBases(t1, t2)
	if !ok {
		return nil, nil
	}
	j, err := p.base.MkJoinFn(b[0], b[1], cols1, cols2)
	if err != nil || j == nil {
		return nil, err
	}
	c1 := append([]int(nil), cols1...)
	c2 := append([]int(nil), cols2...)
	return func(r1, r2 RelationBase) (RelationBase, error) {
		t1, _ := checkGet(r1)
		t2, _ := checkGet(r2)
		r, err := j(t1.relation, t2.relation)
		if err != nil {
			return nil, err
		}
		if err = p.verifyJoin(r1, r2, r, c1, c2); err != nil {
			return nil, errors.Join(err, r.Close())
		}
		return p.newCheckRelation(r.RelationSignature(), r)
	}, nil
}

// MkJoinProjectFn is check_relation_plugin::mk_join_project_fn.
func (p *CheckRelationPlugin) MkJoinProjectFn(t1, t2 RelationBase, cols1, cols2, removed []int) (RelationJoinFn, error) {
	b, ok := checkBases(t1, t2)
	if !ok {
		return nil, nil
	}
	j, err := p.base.MkJoinProjectFn(b[0], b[1], cols1, cols2, removed)
	if err != nil || j == nil {
		return nil, err
	}
	c1 := append([]int(nil), cols1...)
	c2 := append([]int(nil), cols2...)
	rm := append([]int(nil), removed...)
	return func(r1, r2 RelationBase) (RelationBase, error) {
		t1, _ := checkGet(r1)
		t2, _ := checkGet(r2)
		r, err := j(t1.relation, t2.relation)
		if err != nil {
			return nil, err
		}
		if err = p.verifyJoinProject(r1, r2, r, c1, c2, rm); err != nil {
			return nil, errors.Join(err, r.Close())
		}
		return p.newCheckRelation(r.RelationSignature(), r)
	}, nil
}

func (p *CheckRelationPlugin) mkUnion(target, source, delta RelationBase, widen bool) (RelationUnionFn, error) {
	b, ok := checkBases(target, source, delta)
	if !ok {
		return nil, nil
	}
	var u RelationUnionFn
	var err error
	if widen {
		u, err = p.base.MkWidenFn(b[0], b[1], b[2])
	} else {
		u, err = p.base.MkUnionFn(b[0], b[1], b[2])
	}
	if err != nil || u == nil {
		return nil, err
	}
	return func(rb, srcb, deltab RelationBase) (resultErr error) {
		m := p.m
		s := &astScope{m: m}
		defer func() { resultErr = s.close(resultErr) }()
		r, _ := checkGet(rb)
		src, _ := checkGet(srcb)
		var d *CheckRelation
		var dRB RelationBase
		fml0 := s.holdRef(m.CopyRef(r.fml))
		var delta0 ast.ASTHandle
		if deltab != nil {
			d, _ = checkGet(deltab)
			dRB = d.relation
			delta0 = s.holdRef(d.ToFormula())
		} else {
			// expr_ref delta0(m) stays null without a delta.
		}
		if s.err != nil {
			return nil
		}
		if err := u(r.relation, src.relation, dRB); err != nil {
			return err
		}
		if err := p.verifyUnion(fml0, src.relation, r.relation, delta0, dRB); err != nil {
			return err
		}
		if err := r.refreshFml(); err != nil {
			return err
		}
		if d != nil {
			return d.refreshFml()
		}
		return nil
	}, nil
}

// MkUnionFn is check_relation_plugin::mk_union_fn.
func (p *CheckRelationPlugin) MkUnionFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.mkUnion(target, source, delta, false)
}

// MkWidenFn is check_relation_plugin::mk_widen_fn.
func (p *CheckRelationPlugin) MkWidenFn(target, source, delta RelationBase) (RelationUnionFn, error) {
	return p.mkUnion(target, source, delta, true)
}

// MkFilterIdenticalFn is check_relation_plugin::mk_filter_identical_fn.
func (p *CheckRelationPlugin) MkFilterIdenticalFn(t RelationBase, cols []int) (RelationMutatorFn, error) {
	b, ok := checkBases(t)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkFilterIdenticalFn(b[0], cols)
	if err != nil || f == nil {
		return nil, err
	}
	mCols := append([]int(nil), cols...)
	return func(rb RelationBase) (resultErr error) {
		m := p.m
		s := &astScope{m: m}
		defer func() { resultErr = s.close(resultErr) }()
		r, _ := checkGet(rb)
		sig := r.sorts
		cond := s.local()
		conds := s.vector()
		c1 := mCols[0]
		for i := 1; i < len(mCols) && s.err == nil; i++ {
			c2 := mCols[i]
			v1, err1 := m.MkVar(uint32(c1), sig[c1])
			v2, err2 := m.MkVar(uint32(c2), sig[c2])
			if err := errors.Join(err1, err2); err != nil {
				return err
			}
			s.to(conds).push(m.MkEq(v1, v2))
		}
		if s.err == nil {
			s.at(cond).set(udocMakeAnd(m, conds.hs))
		}
		if s.err != nil {
			return nil
		}
		if err := r.consistentFormula(); err != nil {
			return err
		}
		if err := f(r.relation); err != nil {
			return err
		}
		fml, err := r.fmlHandle()
		if err != nil {
			return err
		}
		if err = p.verifyFilter(fml, r.relation, s.get(cond)); err != nil {
			return err
		}
		return r.refreshFml()
	}, nil
}

// consistentFormula is check_relation::consistent_formula.
func (r *CheckRelation) consistentFormula() (resultErr error) {
	f, err := r.relation.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.plugin.m.DelRef(&f)) }()
	a, err := r.plugin.m.RefGet(f)
	if err != nil {
		return err
	}
	b, err := r.fmlHandle()
	if err != nil {
		return err
	}
	if a != b && diagnostic.VerboseProgress {
		var out strings.Builder
		out.WriteString("relation does not have a consistent formula")
		if err = r.Display(&out); err != nil {
			return err
		}
		return diagnostic.ProgressVerbose(0, out.String())
	}
	return nil
}

// MkFilterInterpretedFn is check_relation_plugin::mk_filter_interpreted_fn.
func (p *CheckRelationPlugin) MkFilterInterpretedFn(t RelationBase, condition ast.ASTHandle) (RelationMutatorFn, error) {
	b, ok := checkBases(t)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkFilterInterpretedFn(b[0], condition)
	if err != nil || f == nil {
		return nil, err
	}
	return func(rb RelationBase) (resultErr error) {
		r, _ := checkGet(rb)
		s := &astScope{m: p.m}
		defer func() { resultErr = s.close(resultErr) }()
		fml := s.holdRef(p.m.CopyRef(r.fml))
		if s.err != nil {
			return nil
		}
		if err := f(r.relation); err != nil {
			return err
		}
		if err := p.verifyFilter(fml, r.relation, condition); err != nil {
			return err
		}
		return r.refreshFml()
	}, nil
}

// MkProjectFn is check_relation_plugin::mk_project_fn.
func (p *CheckRelationPlugin) MkProjectFn(t RelationBase, removed []int) (RelationTransformerFn, error) {
	b, ok := checkBases(t)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkProjectFn(b[0], removed)
	if err != nil || f == nil {
		return nil, err
	}
	rm := append([]int(nil), removed...)
	return func(tb RelationBase) (RelationBase, error) {
		t, _ := checkGet(tb)
		r, err := f(t.relation)
		if err != nil {
			return nil, err
		}
		if err = p.verifyProject(tb, r, rm); err != nil {
			return nil, errors.Join(err, r.Close())
		}
		return p.newCheckRelation(r.RelationSignature(), r)
	}, nil
}

// MkRenameFn is check_relation_plugin::mk_rename_fn.
func (p *CheckRelationPlugin) MkRenameFn(t RelationBase, cycle []int) (RelationTransformerFn, error) {
	b, ok := checkBases(t)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkRenameFn(b[0], cycle)
	if err != nil || f == nil {
		return nil, err
	}
	c := append([]int(nil), cycle...)
	sig := renameSorts(t.RelationSignature(), cycle)
	return func(tb RelationBase) (RelationBase, error) {
		t, _ := checkGet(tb)
		r, err := f(t.relation)
		if err != nil {
			return nil, err
		}
		if err = p.verifyPermutation(t.relation, r, c); err != nil {
			return nil, errors.Join(err, r.Close())
		}
		return p.newCheckRelation(sig, r)
	}, nil
}

// MkFilterEqualFn is check_relation_plugin::mk_filter_equal_fn.
func (p *CheckRelationPlugin) MkFilterEqualFn(t RelationBase, value ast.ASTHandle, col int) (RelationMutatorFn, error) {
	b, ok := checkBases(t)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkFilterEqualFn(b[0], value, col)
	if err != nil || f == nil {
		return nil, err
	}
	return func(tb RelationBase) (resultErr error) {
		m := p.m
		s := &astScope{m: m}
		defer func() { resultErr = s.close(resultErr) }()
		t, _ := checkGet(tb)
		if err := f(t.relation); err != nil {
			return err
		}
		// expr_ref fml = t.m_fml;
		fml := s.local()
		s.at(fml).set(s.get(t.fml), nil)
		if s.err != nil {
			return nil
		}
		if err := t.refreshFml(); err != nil {
			return err
		}
		v, err := m.MkVar(uint32(col), t.sorts[col])
		if err != nil {
			return err
		}
		eq, err := m.MkEq(v, value)
		if err != nil {
			return err
		}
		s.at(fml).set(m.MkAnd(s.get(fml), eq))
		x := s.child()
		g1 := p.ground(x, t, x.get(fml), x.local())
		g2 := p.ground(x, t, x.get(t.fml), x.local())
		if x.err == nil {
			x.fail(p.checkEquiv("filter_equal", g1, g2))
		}
		s.end(x)
		return nil
	}, nil
}

// MkFilterByNegationFn is check_relation_plugin::mk_filter_by_negation_fn.
func (p *CheckRelationPlugin) MkFilterByNegationFn(t, neg RelationBase, tCols, negCols []int) (RelationIntersectionFilterFn, error) {
	b, ok := checkBases(t, neg)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkFilterByNegationFn(b[0], b[1], tCols, negCols)
	if err != nil || f == nil {
		return nil, err
	}
	c1 := append([]int(nil), tCols...)
	c2 := append([]int(nil), negCols...)
	return func(tb, negb RelationBase) (resultErr error) {
		s := &astScope{m: p.m}
		defer func() { resultErr = s.close(resultErr) }()
		t, _ := checkGet(tb)
		n, _ := checkGet(negb)
		dst0 := s.holdRef(t.ToFormula())
		if s.err != nil {
			return nil
		}
		if err := f(t.relation, n.relation); err != nil {
			return err
		}
		if err := t.refreshFml(); err != nil {
			return err
		}
		return p.verifyFilterByNegation(dst0, t.relation, n.relation, c1, c2)
	}, nil
}

// MkFilterInterpretedAndProjectFn is
// check_relation_plugin::mk_filter_interpreted_and_project_fn.
func (p *CheckRelationPlugin) MkFilterInterpretedAndProjectFn(t RelationBase, condition ast.ASTHandle, removed []int) (RelationTransformerFn, error) {
	b, ok := checkBases(t)
	if !ok {
		return nil, nil
	}
	f, err := p.base.MkFilterInterpretedAndProjectFn(b[0], condition, removed)
	if err != nil || f == nil {
		return nil, err
	}
	rm := append([]int(nil), removed...)
	sig := projectSorts(t.RelationSignature(), removed)
	return func(tb RelationBase) (RelationBase, error) {
		t, _ := checkGet(tb)
		r, err := f(t.relation)
		if err != nil {
			return nil, err
		}
		if err = p.verifyFilterProject(t.relation, r, condition, rm); err != nil {
			return nil, errors.Join(err, r.Close())
		}
		return p.newCheckRelation(sig, r)
	}, nil
}
