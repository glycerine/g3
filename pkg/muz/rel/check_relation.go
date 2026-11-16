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

// astScope holds references to the expressions an operation builds
// (the expr_refs of the C++), released by close.
type astScope struct {
	m    *ast.ASTManager
	refs []ast.ASTRef
	err  error
}

func (s *astScope) hold(h ast.ASTHandle, err error) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	ref, err := s.m.NewRef(h)
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	s.refs = append(s.refs, ref)
	return h
}

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
	s.refs = append(s.refs, ref)
	h, err := s.m.RefGet(ref)
	if err != nil {
		s.err = err
	}
	return h
}

func (s *astScope) close(result error) error {
	for i := len(s.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, s.m.DelRef(&s.refs[i]))
	}
	s.refs = nil
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

// mkEq is check_relation::mk_eq(f).
func (r *CheckRelation) mkEq(s *astScope, values []ast.ASTHandle) ast.ASTHandle {
	m := r.plugin.m
	conjs := make([]ast.ASTHandle, 0, len(r.sorts))
	for i, sort := range r.sorts {
		v := s.hold(m.MkVar(uint32(i), sort))
		conjs = append(conjs, s.hold(m.MkEq(v, values[i])))
	}
	if s.err != nil {
		return ast.ASTHandle{}
	}
	return s.hold(udocMakeAnd(m, conjs))
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
	inserted, err := r.relation.Add(f)
	if err != nil {
		return false, err
	}
	fml1 := s.holdRef(r.relation.ToFormula())
	old, err := r.fmlHandle()
	if err != nil {
		return false, err
	}
	eq := r.mkEq(s, values)
	or := s.hold(m.MkOr(old, eq))
	if s.err != nil {
		return false, nil
	}
	if err = r.setFml(or); err != nil {
		return false, err
	}
	g1 := p.ground(s, r, or)
	g2 := p.ground(s, r, fml1)
	if s.err != nil {
		return false, nil
	}
	if err = p.checkEquiv("add_fact", g1, g2); err != nil {
		return false, err
	}
	return inserted, r.setFml(fml1)
}

// Empty is check_relation::empty.
func (r *CheckRelation) Empty() (bool, error) {
	result, err := r.relation.Empty()
	if err != nil {
		return false, err
	}
	return result, r.checkEmpty(result, "empty")
}

func (r *CheckRelation) checkEmpty(result bool, objective string) (resultErr error) {
	m := r.plugin.m
	fml, err := r.fmlHandle()
	if err != nil {
		return err
	}
	if !result || m.IsFalse(fml) {
		return nil
	}
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	f := s.hold(m.MkFalse())
	g := r.plugin.ground(s, r, fml)
	if s.err != nil {
		return nil
	}
	return r.plugin.checkEquiv(objective, f, g)
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
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	result, err := r.relation.Contains(f)
	if err != nil {
		return false, err
	}
	values := r.factValues(s, f)
	if s.err != nil {
		return false, nil
	}
	fml, err := r.fmlHandle()
	if err != nil {
		return false, err
	}
	fml1 := r.mkEq(s, values)
	fml2 := s.hold(m.MkAnd(fml, fml1))
	if s.err != nil {
		return false, nil
	}
	if result {
		g1, g2 := p.ground(s, r, fml1), p.ground(s, r, fml2)
		if s.err == nil {
			err = p.checkEquiv("contains fact", g1, g2)
		}
	} else if !m.IsFalse(fml) {
		g2 := p.ground(s, r, fml2)
		f := s.hold(m.MkFalse())
		if s.err == nil {
			err = p.checkEquiv("contains fact", g2, f)
		}
	}
	return result, err
}

// CloneRelation is check_relation::clone.
func (r *CheckRelation) CloneRelation() (RelationBase, error) {
	return r.clone()
}

func (r *CheckRelation) clone() (_ *CheckRelation, resultErr error) {
	p := r.plugin
	inner, err := r.relation.CloneRelation()
	if err != nil {
		return nil, err
	}
	// mk_empty(get_signature()) followed by replacing its relation.
	empty, err := p.MkEmpty(r.sorts)
	if err != nil {
		return nil, errors.Join(err, inner.Close())
	}
	result, _ := checkGet(empty)
	if err = result.relation.Close(); err != nil {
		return nil, errors.Join(err, inner.Close())
	}
	result.relation = inner
	if err = result.refreshFml(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	a, err := r.fmlHandle()
	if err != nil {
		return nil, err
	}
	b, err := result.fmlHandle()
	if err != nil {
		return nil, err
	}
	if a != b {
		s := &astScope{m: p.m}
		g1, g2 := p.ground(s, r, a), p.ground(s, r, b)
		if s.err == nil {
			err = p.checkEquiv("clone", g1, g2)
		}
		if err = s.close(err); err != nil {
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
	s := &astScope{m: m}
	defer func() {
		resultErr = s.close(resultErr)
		if resultErr != nil {
			resultErr = errors.Join(resultErr, r.Close())
		}
	}()
	var c ast.ASTHandle
	if value {
		c = s.hold(m.MkTrue())
	} else {
		c = s.hold(m.MkFalse())
	}
	fml, err := r.fmlHandle()
	if err != nil || s.err != nil {
		return err
	}
	if fml == c {
		return nil
	}
	g := p.ground(s, r, fml)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv(objective, g, c)
}

// varSubst is var_subst sub(m, false) applied to fml.
func (p *CheckRelationPlugin) varSubst(s *astScope, fml ast.ASTHandle, vars []ast.ASTHandle) ast.ASTHandle {
	if s.err != nil {
		return ast.ASTHandle{}
	}
	sub, err := astrewriter.NewVarSubstitution(p.m, false)
	if err != nil {
		s.err = err
		return ast.ASTHandle{}
	}
	out, err := p.m.NewRef(ast.ASTHandle{})
	if err == nil {
		err = sub.Substitute(fml, vars, out)
	}
	err = errors.Join(err, sub.Close())
	return s.holdRef(out, err)
}

// groundConstants is the vector of m.mk_const(symbol(i), sig[i]).
func (p *CheckRelationPlugin) groundConstants(s *astScope, sorts []ast.ASTHandle) []ast.ASTHandle {
	vars := make([]ast.ASTHandle, len(sorts))
	for i, sort := range sorts {
		vars[i] = s.hold(p.m.MkConst(symbol.Numerical(uint32(i)), sort))
	}
	return vars
}

// namedConstants is the vector of m.mk_const(symbol("x<i>"), sig[i]).
func (p *CheckRelationPlugin) namedConstants(s *astScope, sorts []ast.ASTHandle) []ast.ASTHandle {
	vars := make([]ast.ASTHandle, len(sorts))
	for i, sort := range sorts {
		name, err := p.m.Symbols.Intern("x" + utoa(uint64(i)))
		if err != nil {
			s.err = err
			return vars
		}
		vars[i] = s.hold(p.m.MkConst(name, sort))
	}
	return vars
}

// ground is check_relation_plugin::ground(dst, fml).
func (p *CheckRelationPlugin) ground(s *astScope, dst RelationBase, fml ast.ASTHandle) ast.ASTHandle {
	vars := p.groundConstants(s, dst.RelationSignature())
	return p.varSubst(s, fml, vars)
}

// groundRelation is check_relation_plugin::ground(dst).
func (p *CheckRelationPlugin) groundRelation(s *astScope, dst RelationBase) ast.ASTHandle {
	fml := s.holdRef(dst.ToFormula())
	if s.err != nil {
		return ast.ASTHandle{}
	}
	return p.ground(s, dst, fml)
}

// checkEquiv is check_relation_plugin::check_equiv.
func (p *CheckRelationPlugin) checkEquiv(objective string, fml1, fml2 ast.ASTHandle) (resultErr error) {
	m := p.m
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
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
	defer func() { resultErr = errors.Join(resultErr, kernel.Close()) }()
	eq := s.hold(m.MkEq(fml1, fml2))
	tmp := s.hold(m.MkNot(eq))
	if s.err != nil {
		return nil
	}
	if err = kernel.Assert(tmp, ast.ASTHandle{}); err != nil {
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
func (p *CheckRelationPlugin) checkContains(objective string, fml1, fml2 ast.ASTHandle) (resultErr error) {
	s := &astScope{m: p.m}
	defer func() { resultErr = s.close(resultErr) }()
	fml0 := s.hold(p.m.MkAnd(fml1, fml2))
	if s.err != nil {
		return nil
	}
	return p.checkEquiv(objective, fml0, fml2)
}

// mkProject is check_relation_plugin::mk_project.
func (p *CheckRelationPlugin) mkProject(s *astScope, sorts []ast.ASTHandle, fml ast.ASTHandle, removed []int) ast.ASTHandle {
	m := p.m
	var bound []ast.ASTHandle
	var names []symbol.Symbol
	vars := make([]ast.ASTHandle, 0, len(sorts))
	rmCnt := len(removed)
	for i, j, k := 0, 0, 0; i < len(sorts); i++ {
		if j < rmCnt && removed[j] == i {
			name, err := m.Symbols.Intern("x" + utoa(uint64(j)))
			if err != nil {
				s.err = err
				return ast.ASTHandle{}
			}
			bound = append(bound, sorts[i])
			names = append(names, name)
			vars = append(vars, s.hold(m.MkVar(uint32(j), sorts[i])))
			j++
		} else {
			vars = append(vars, s.hold(m.MkVar(uint32(k+rmCnt), sorts[i])))
			k++
		}
	}
	fml1 := p.varSubst(s, fml, vars)
	if s.err != nil {
		return ast.ASTHandle{}
	}
	for i, j := 0, len(bound)-1; i < j; i, j = i+1, j-1 {
		bound[i], bound[j] = bound[j], bound[i]
	}
	if len(bound) == 0 {
		// mk_quantifier with no bound variables returns the body.
		return fml1
	}
	return s.hold(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: bound, Names: names, Body: fml1}))
}

// mkJoin is check_relation_plugin::mk_join.
func (p *CheckRelationPlugin) mkJoin(s *astScope, t1, t2 RelationBase, cols1, cols2 []int) ast.ASTHandle {
	m := p.m
	sig1, sig2 := t1.RelationSignature(), t2.RelationSignature()
	fml1 := s.holdRef(t1.ToFormula())
	fml2 := s.holdRef(t2.ToFormula())
	vars := make([]ast.ASTHandle, len(sig2))
	for i, sort := range sig2 {
		vars[i] = s.hold(m.MkVar(uint32(i+len(sig1)), sort))
	}
	fml2 = p.varSubst(s, fml2, vars)
	if s.err != nil {
		return ast.ASTHandle{}
	}
	fml1 = s.hold(m.MkAnd(fml1, fml2))
	for i := range cols1 {
		v1, v2 := cols1[i], cols2[i]
		var1 := s.hold(m.MkVar(uint32(v1), sig1[v1]))
		var2 := s.hold(m.MkVar(uint32(v2+len(sig1)), sig2[v2]))
		eq := s.hold(m.MkEq(var1, var2))
		if s.err != nil {
			return ast.ASTHandle{}
		}
		fml1 = s.hold(m.MkAnd(eq, fml1))
	}
	return fml1
}

// verifyJoin is check_relation_plugin::verify_join.
func (p *CheckRelationPlugin) verifyJoin(t1, t2, t RelationBase, cols1, cols2 []int) (resultErr error) {
	s := &astScope{m: p.m}
	defer func() { resultErr = s.close(resultErr) }()
	fml1 := p.ground(s, t, p.mkJoin(s, t1, t2, cols1, cols2))
	fml2 := p.groundRelation(s, t)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("join", fml1, fml2)
}

// verifyJoinProject is check_relation_plugin::verify_join_project.
func (p *CheckRelationPlugin) verifyJoinProject(t1, t2, t RelationBase, cols1, cols2, removed []int) (resultErr error) {
	s := &astScope{m: p.m}
	defer func() { resultErr = s.close(resultErr) }()
	sig1 := joinSorts(t1.RelationSignature(), t2.RelationSignature())
	fml1 := p.mkJoin(s, t1, t2, cols1, cols2)
	fml1 = p.mkProject(s, sig1, fml1, removed)
	fml1 = p.ground(s, t, fml1)
	fml2 := p.groundRelation(s, t)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("join_project", fml1, fml2)
}

// verifyProjectFormulas is verify_project(src, f1, dst, f2, removed).
func (p *CheckRelationPlugin) verifyProjectFormulas(s *astScope, src RelationBase, f1 ast.ASTHandle, dst RelationBase, f2 ast.ASTHandle, removed []int) error {
	fml1 := p.ground(s, dst, p.mkProject(s, src.RelationSignature(), f1, removed))
	fml2 := p.ground(s, dst, f2)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("project", fml1, fml2)
}

// verifyProject is verify_project(src, dst, removed).
func (p *CheckRelationPlugin) verifyProject(src, dst RelationBase, removed []int) (resultErr error) {
	s := &astScope{m: p.m}
	defer func() { resultErr = s.close(resultErr) }()
	fml1 := s.holdRef(src.ToFormula())
	fml2 := s.holdRef(dst.ToFormula())
	if s.err != nil {
		return nil
	}
	return p.verifyProjectFormulas(s, src, fml1, dst, fml2, removed)
}

// verifyFilterProject is check_relation_plugin::verify_filter_project.
func (p *CheckRelationPlugin) verifyFilterProject(src, dst RelationBase, cond ast.ASTHandle, removed []int) (resultErr error) {
	s := &astScope{m: p.m}
	defer func() { resultErr = s.close(resultErr) }()
	fml1 := s.holdRef(src.ToFormula())
	fml2 := s.holdRef(dst.ToFormula())
	if s.err != nil {
		return nil
	}
	fml1 = s.hold(p.m.MkAnd(cond, fml1))
	if s.err != nil {
		return nil
	}
	return p.verifyProjectFormulas(s, src, fml1, dst, fml2, removed)
}

// verifyPermutation is check_relation_plugin::verify_permutation.
func (p *CheckRelationPlugin) verifyPermutation(src, dst RelationBase, cycle []int) (resultErr error) {
	m := p.m
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	sig1, sig2 := src.RelationSignature(), dst.RelationSignature()
	perm := make([]int, len(sig1))
	for i := range perm {
		perm[i] = i
	}
	for i := range cycle {
		j := (i + 1) % len(cycle)
		perm[cycle[j]] = cycle[i]
	}
	sub := make([]ast.ASTHandle, len(perm))
	for i := range perm {
		sub[i] = s.hold(m.MkVar(uint32(perm[i]), sig1[i]))
	}
	fml1 := s.holdRef(src.ToFormula())
	fml2 := s.holdRef(dst.ToFormula())
	fml1 = p.varSubst(s, fml1, sub)
	vars := p.groundConstants(s, sig2)
	fml1 = p.varSubst(s, fml1, vars)
	fml2 = p.varSubst(s, fml2, vars)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("permutation", fml1, fml2)
}

// verifyFilter is check_relation_plugin::verify_filter.
func (p *CheckRelationPlugin) verifyFilter(fml0 ast.ASTHandle, t RelationBase, cond ast.ASTHandle) (resultErr error) {
	m := p.m
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	fml1 := s.hold(m.MkAnd(fml0, cond))
	fml2 := s.holdRef(t.ToFormula())
	vars := p.namedConstants(s, t.RelationSignature())
	fml1 = p.varSubst(s, fml1, vars)
	fml2 = p.varSubst(s, fml2, vars)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("filter", fml1, fml2)
}

// verifyUnion is check_relation_plugin::verify_union.
func (p *CheckRelationPlugin) verifyUnion(dst0 ast.ASTHandle, src, dst RelationBase, delta0 ast.ASTHandle, delta RelationBase) (resultErr error) {
	m := p.m
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	fml1 := s.holdRef(src.ToFormula())
	fml2 := s.holdRef(dst.ToFormula())
	fml1 = s.hold(m.MkOr(fml1, dst0))
	vars := p.namedConstants(s, dst.RelationSignature())
	fml1 = p.varSubst(s, fml1, vars)
	fml2 = p.varSubst(s, fml2, vars)
	if s.err != nil {
		return nil
	}
	if err := p.checkEquiv("union", fml1, fml2); err != nil {
		return err
	}
	if delta == nil {
		return nil
	}
	d := s.holdRef(delta.ToFormula())
	if s.err != nil {
		return nil
	}
	if diagnostic.VerboseProgress {
		if enabled, err := diagnostic.ProcessVerboseEnabled(3); err != nil {
			return err
		} else if enabled {
			text, err := relMkPP(m, d)
			if err != nil {
				return err
			}
			if err = diagnostic.ProgressVerbose(3, "verify delta "+text+"\n"); err != nil {
				return err
			}
		}
	}
	notDst0 := s.hold(m.MkNot(dst0))
	fml4 := s.hold(m.MkAnd(fml2, notDst0))
	fml4 = p.varSubst(s, fml4, vars)
	d = p.varSubst(s, d, vars)
	if s.err != nil {
		return nil
	}
	if err := p.checkContains("union_delta low", d, fml4); err != nil {
		return err
	}
	d0 := p.varSubst(s, delta0, vars)
	if s.err != nil {
		return nil
	}
	if err := p.checkContains("union delta0", d, d0); err != nil {
		return err
	}
	fml4 = s.hold(m.MkOr(fml2, delta0))
	fml5 := s.hold(m.MkOr(d, dst0))
	fml4 = p.varSubst(s, fml4, vars)
	fml5 = p.varSubst(s, fml5, vars)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("union no overflow", fml4, fml5)
}

// verifyFilterByNegation is check_relation_plugin::verify_filter_by_negation.
func (p *CheckRelationPlugin) verifyFilterByNegation(dst0 ast.ASTHandle, dst, neg RelationBase, cols1, cols2 []int) (resultErr error) {
	m := p.m
	s := &astScope{m: m}
	defer func() { resultErr = s.close(resultErr) }()
	sig1, sig2 := dst.RelationSignature(), neg.RelationSignature()
	dstf := s.holdRef(dst.ToFormula())
	negf := s.holdRef(neg.ToFormula())
	eqs := []ast.ASTHandle{negf}
	for i := range cols1 {
		c1, c2 := cols1[i], cols2[i]
		v1 := s.hold(m.MkVar(uint32(len(sig2)+c1), sig1[c1]))
		v2 := s.hold(m.MkVar(uint32(c2), sig2[c2]))
		eqs = append(eqs, s.hold(m.MkEq(v1, v2)))
	}
	if s.err != nil {
		return nil
	}
	negf = s.hold(udocMakeAnd(m, eqs))
	rev := make([]ast.ASTHandle, len(sig2))
	for i := range sig2 {
		rev[i] = sig2[len(sig2)-1-i]
	}
	names := make([]symbol.Symbol, len(sig2))
	for i := range names {
		names[i] = symbol.Numerical(uint32(i))
	}
	if len(rev) != 0 && s.err == nil {
		negf = s.hold(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: rev, Names: names, Body: negf}))
	}
	notNeg := s.hold(m.MkNot(negf))
	negf = s.hold(m.MkAnd(dst0, notNeg))
	negf = p.ground(s, dst, negf)
	dstf = p.ground(s, dst, dstf)
	if s.err != nil {
		return nil
	}
	return p.checkEquiv("filter by negation", dstf, negf)
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
		conds := make([]ast.ASTHandle, 0, len(mCols))
		c1 := mCols[0]
		for i := 1; i < len(mCols); i++ {
			c2 := mCols[i]
			v1 := s.hold(m.MkVar(uint32(c1), sig[c1]))
			v2 := s.hold(m.MkVar(uint32(c2), sig[c2]))
			conds = append(conds, s.hold(m.MkEq(v1, v2)))
		}
		if s.err != nil {
			return nil
		}
		cond := s.hold(udocMakeAnd(m, conds))
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
		if err = p.verifyFilter(fml, r.relation, cond); err != nil {
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
		fml := s.holdRef(m.CopyRef(t.fml))
		if err := t.refreshFml(); err != nil {
			return err
		}
		v := s.hold(m.MkVar(uint32(col), t.sorts[col]))
		eq := s.hold(m.MkEq(v, value))
		fml = s.hold(m.MkAnd(fml, eq))
		tf, err := t.fmlHandle()
		if err != nil {
			return err
		}
		g1, g2 := p.ground(s, t, fml), p.ground(s, t, tf)
		if s.err != nil {
			return nil
		}
		return p.checkEquiv("filter_equal", g1, g2)
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
