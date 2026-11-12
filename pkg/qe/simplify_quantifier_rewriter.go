package qe

// Source: src/qe/qe.cpp, simplify_rewriter_cfg::reduce_quantifier/pre_visit.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// SimplifyQuantifiers performs the source QE simplification used by Datalog
// rule normalization: dl_rule.cpp constructs a local simplify_rewriter_star
// for one rewrite. The caller owns the returned reference.
func SimplifyQuantifiers(m *ast.ASTManager, config Config, expression ast.ASTHandle) (result ast.ASTRef, err error) {
	rw, err := newSimplifyRewriterStar(m, config)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	return rw.rewrite(expression)
}

// simplifyRewriterStar is qe::simplify_rewriter_star. Its configuration owns
// one simplify_solver_context, constructed with the rewriter; rewriter_core
// owns the cache stack (level 0 lives with the rewriter, deeper levels are
// reset when a quantifier's scope ends).
type simplifyRewriterStar struct {
	manager *ast.ASTManager
	config  Config
	ctx     *simplifySolverContext
	caches  []*ast.ActivityCache
	level   int
	root    ast.ASTHandle
}

func newSimplifyRewriterStar(m *ast.ASTManager, config Config) (r *simplifyRewriterStar, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	r = &simplifyRewriterStar{manager: m, config: config}
	// rewriter_tpl (with rewriter_core's init_cache_stack) is the base,
	// constructed before the m_cfg member.
	cache, err := ast.NewActivityCache(m, nil)
	if err != nil {
		return nil, err
	}
	r.caches = append(r.caches, cache)
	if r.ctx, err = newSimplifySolverContext(m, config); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

// Close is ~simplify_rewriter_star: the m_cfg member (its solver context),
// then rewriter_core's cache stack in level order.
func (r *simplifyRewriterStar) Close() error {
	if r == nil {
		return nil
	}
	if r.ctx != nil {
		if err := r.ctx.Close(); err != nil {
			return err
		}
		r.ctx = nil
	}
	for len(r.caches) > 0 {
		if err := r.caches[0].Close(); err != nil {
			return err
		}
		r.caches = r.caches[1:]
	}
	return nil
}

// rewrite is rewriter_tpl::operator(): m_root is the input.
func (r *simplifyRewriterStar) rewrite(expression ast.ASTHandle) (ast.ASTRef, error) {
	if r == nil || r.ctx == nil || len(r.caches) == 0 {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	r.root = expression
	r.level = 0
	return r.visit(expression)
}

// mustCache is rewriter_core::must_cache.
func (r *simplifyRewriterStar) mustCache(t ast.ASTHandle) (bool, error) {
	m := r.manager
	if t == r.root {
		return false, nil
	}
	rc, err := m.GetRefCount(t)
	if err != nil || rc <= 1 {
		return false, err
	}
	kind, err := m.GetKind(t)
	if err != nil {
		return false, err
	}
	switch kind {
	case ast.ASTQuantifier:
		return true, nil
	case ast.ASTApp:
		n, err := m.GetNumArgs(t)
		return n > 0, err
	}
	return false, nil
}

// visit is rewriter_tpl::visit with the cache lookup, then the rewrite of t
// and cache_result.
func (r *simplifyRewriterStar) visit(t ast.ASTHandle) (result ast.ASTRef, err error) {
	m := r.manager
	c, err := r.mustCache(t)
	if err != nil {
		return result, err
	}
	if c {
		cached, err := r.caches[r.level].Find(t)
		if err != nil {
			return result, err
		}
		if !cached.IsNull() {
			return m.NewRef(cached)
		}
	}
	if c {
		// pre_visit refusing t pushes no frame, so nothing is cached.
		if c, err = r.preVisit(t); err != nil {
			return result, err
		}
	}
	result, err = simplifyQEExpression(m, r, t)
	if err != nil || !c {
		return result, err
	}
	value, err := m.RefGet(result)
	if err == nil {
		err = r.caches[r.level].Insert(t, value)
	}
	if err != nil {
		return result, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

// preVisit is simplify_rewriter_cfg::pre_visit.
func (r *simplifyRewriterStar) preVisit(t ast.ASTHandle) (bool, error) {
	m := r.manager
	kind, err := m.GetKind(t)
	if err != nil || kind != ast.ASTQuantifier {
		return true, err
	}
	patterns, err := m.GetNumPatterns(t)
	if err != nil {
		return false, err
	}
	noPatterns, err := m.GetNumNoPatterns(t)
	return patterns == 0 && noPatterns == 0, err
}

// beginScope is rewriter_core::begin_scope plus m_root = q->get_expr().
func (r *simplifyRewriterStar) beginScope(body ast.ASTHandle) (oldRoot ast.ASTHandle, err error) {
	oldRoot = r.root
	r.level++
	if r.level == len(r.caches) {
		cache, err := ast.NewActivityCache(r.manager, nil)
		if err != nil {
			r.level--
			return oldRoot, err
		}
		r.caches = append(r.caches, cache)
	}
	if err = r.caches[r.level].Reset(); err != nil {
		return oldRoot, err
	}
	r.root = body
	return oldRoot, nil
}

// endScope is rewriter_core::end_scope.
func (r *simplifyRewriterStar) endScope(oldRoot ast.ASTHandle) error {
	err := r.caches[r.level].Reset()
	r.root = oldRoot
	r.level--
	return err
}

// simplifyQEExpression visits unannotated quantifiers bottom-up and simplifies
// their bodies without requiring every quantified variable to be eliminated.
func simplifyQEExpression(m *ast.ASTManager, rw *simplifyRewriterStar, expression ast.ASTHandle) (result ast.ASTRef, err error) {
	if m == nil {
		return result, ast.ErrASTHandle
	}
	canceled, err := m.Canceled()
	if err != nil {
		return result, err
	}
	if canceled {
		return result, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	kind, err := m.GetKind(expression)
	if err != nil {
		return result, err
	}
	switch kind {
	case ast.ASTQuantifier:
		patterns, e := m.GetNumPatterns(expression)
		if e != nil {
			return result, e
		}
		noPatterns, e := m.GetNumNoPatterns(expression)
		if e != nil {
			return result, e
		}
		if patterns != 0 || noPatterns != 0 {
			return m.NewRef(expression)
		}
		body, e := m.GetQuantifierBody(expression)
		if e != nil {
			return result, e
		}
		oldRoot, e := rw.beginScope(body)
		if e != nil {
			return result, e
		}
		scoped := true
		defer func() {
			if scoped {
				err = errors.Join(err, rw.endScope(oldRoot))
			}
		}()
		child, e := rw.visit(body)
		if e != nil {
			return result, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&child)) }()
		body, e = m.RefGet(child)
		if e != nil {
			return result, e
		}
		updated, e := m.UpdateQuantifierBody(expression, body)
		if e != nil {
			return result, e
		}
		updatedRef, e := m.NewRef(updated)
		if e != nil {
			return result, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&updatedRef)) }()
		result, err = simplifyQEQuantifier(m, rw.ctx, updated)
		if err != nil {
			return result, err
		}
		scoped = false
		return result, rw.endScope(oldRoot)
	case ast.ASTApp:
		n, e := m.GetNumArgs(expression)
		if e != nil {
			return result, e
		}
		args := make([]ast.ASTHandle, n)
		refs := make([]ast.ASTRef, 0, n)
		defer func() {
			for i := len(refs) - 1; i >= 0; i-- {
				err = errors.Join(err, m.DelRef(&refs[i]))
			}
		}()
		changed := false
		for i := range args {
			old, x := m.GetArg(expression, uint32(i))
			if x != nil {
				return result, x
			}
			ref, x := rw.visit(old)
			if x != nil {
				return result, x
			}
			refs = append(refs, ref)
			args[i], x = m.RefGet(ref)
			if x != nil {
				return result, x
			}
			changed = changed || args[i] != old
		}
		if changed {
			decl, x := m.GetDecl(expression)
			if x != nil {
				return result, x
			}
			expression, x = m.MkApp(decl, args)
			if x != nil {
				return result, x
			}
		}
		return m.NewRef(expression)
	default:
		return m.NewRef(expression)
	}
}

// simplifyQEQuantifier is simplify_rewriter_cfg::impl::reduce_quantifier.
// The source threads one expr_ref, result, through every step, so each
// assignment releases the previous formula; cur is that slot.
func simplifyQEQuantifier(m *ast.ASTManager, ctx *simplifySolverContext, quantifier ast.ASTHandle) (result ast.ASTRef, err error) {
	extracted, err := extractExprQuantifier(m, quantifier)
	if err != nil {
		return result, closeFailedExprQuantifierExtract(extracted, err)
	}
	defer func() { err = errors.Join(err, extracted.Close()) }()
	cur := extracted.body
	extracted.body = ast.ASTRef{}
	defer func() {
		if err != nil {
			err = errors.Join(err, m.DelRef(&cur))
		}
	}()
	vars := make([]ast.ASTHandle, len(extracted.variables))
	for i, ref := range extracted.variables {
		vars[i], err = m.RefGet(ref)
		if err != nil {
			return result, err
		}
	}
	body, err := m.RefGet(cur)
	if err != nil {
		return result, err
	}
	forall, err := m.IsForall(quantifier)
	if err != nil {
		return result, err
	}
	if forall {
		body, err = m.MkNot(body)
		if err == nil {
			err = m.SetRef(cur, body)
		}
		if err != nil {
			return result, err
		}
	}
	remaining, err := ctx.solve(vars, cur)
	defer func() {
		for i := range remaining {
			err = errors.Join(err, m.DelRef(&remaining[i]))
		}
	}()
	if err != nil {
		return result, err
	}
	body, err = m.RefGet(cur)
	if err != nil {
		return result, err
	}
	if forall {
		var isNot bool
		var arg ast.ASTHandle
		isNot, arg, err = m.IsNot(body)
		if err != nil {
			return result, err
		}
		if isNot {
			body = arg
		} else {
			body, err = m.MkNot(body)
			if err != nil {
				return result, err
			}
		}
		if err = m.SetRef(cur, body); err != nil {
			return result, err
		}
	}
	if len(remaining) == 0 {
		// The source still shifts and abstracts over zero variables, which
		// leave the formula unchanged.
		result, cur = cur, ast.ASTRef{}
		return result, nil
	}
	shifter, err := rewriter.NewVarShifter(m)
	if shifter != nil {
		defer func() { err = errors.Join(err, shifter.Close()) }()
	}
	if err != nil {
		return result, err
	}
	if err = shifter.ShiftBy(body, uint32(len(remaining)), cur); err != nil {
		return result, err
	}
	body, err = m.RefGet(cur)
	if err != nil {
		return result, err
	}
	bound := make([]ast.ASTHandle, len(remaining))
	sorts := make([]ast.ASTHandle, len(bound))
	names := make([]symbol.Symbol, len(bound))
	for i, ref := range remaining {
		bound[i], err = m.RefGet(ref)
		if err != nil {
			return result, err
		}
		sorts[i], err = m.GetSort(bound[i])
		if err != nil {
			return result, err
		}
		decl, e := m.GetDecl(bound[i])
		if e != nil {
			return result, e
		}
		names[i], err = m.GetASTName(decl)
		if err != nil {
			return result, err
		}
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	if err = abstractor.Abstract(0, bound, body, cur); err != nil {
		return result, err
	}
	body, err = m.RefGet(cur)
	if err != nil {
		return result, err
	}
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: body, Weight: 1})
	if err != nil {
		return result, err
	}
	if err = m.SetRef(cur, q); err != nil {
		return result, err
	}
	result, cur = cur, ast.ASTRef{}
	return result, nil
}
