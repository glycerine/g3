package core

// Source: src/tactic/core/dom_simplify_tactic.cpp:176-441 (dom_simplify_tactic).
//
// Every expr_ref / expr_ref_vector of the source is an ast.ASTRef here and is
// released where the source releases it, so nodes die (and their ids are
// recycled) at the same points.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// releaseRefs is expr_ref_vector::reset / ~expr_ref_vector: dec_range_ref
// walks the vector front to back.
func (t *DomSimplifyTactic) releaseRefs(refs []ast.ASTRef) error {
	var err error
	for i := range refs {
		err = errors.Join(err, t.manager.DelRef(&refs[i]))
	}
	return err
}

// domCleanup is dom_simplify_tactic::cleanup.
func (t *DomSimplifyTactic) domCleanup() error {
	err := t.releaseRefs(t.domTrail) // m_trail.reset()
	t.domTrail = nil
	err = errors.Join(err, t.releaseRefs(t.domArgs)) // m_args.reset()
	t.domArgs = nil
	t.domCache = nil                              // m_result.reset()
	return errors.Join(err, t.dominators.reset()) // m_dominators.reset()
}

// domInit is dom_simplify_tactic::init.
func (t *DomSimplifyTactic) domInit(g *tactic.Goal) (compiled bool, err error) {
	n, err := g.Size()
	if err != nil {
		return false, err
	}
	args := make([]ast.ASTRef, 0, n) // expr_ref_vector args(m)
	defer func() { err = errors.Join(err, t.releaseRefs(args)) }()
	forms := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		if forms[i], err = g.Form(i); err != nil {
			return false, fmt.Errorf("dom goal form %d: %w", i, err)
		}
		ref, err := t.manager.NewRef(forms[i])
		if err != nil {
			return false, err
		}
		args = append(args, ref)
	}
	root, err := domMkAndArgs(t.manager, forms)
	if err != nil {
		return false, fmt.Errorf("dom goal conjunction: %w", err)
	}
	fml, err := t.manager.NewRef(root) // expr_ref fml = mk_and(args)
	if err != nil {
		return false, err
	}
	// fml is declared after args, so it is released first.
	defer func() { err = errors.Join(err, t.manager.DelRef(&fml)) }()
	t.domCache = make(map[ast.ASTHandle]ast.ASTHandle) // m_result.reset()
	if err = t.releaseRefs(t.domTrail); err != nil {   // m_trail.reset()
		return false, err
	}
	t.domTrail = t.domTrail[:0]
	compiled, err = t.dominators.compile(root)
	if err != nil {
		return false, fmt.Errorf("dom compile: %w", err)
	}
	if compiled {
		t.traceDominatorTree()
	}
	return compiled, nil
}

// applyDomSimplifier is dom_simplify_tactic::simplify_goal followed by the
// inc_depth / push_back of operator().
func (t *DomSimplifyTactic) applyDomSimplifier(g *tactic.Goal, out *tactic.TacticResult) error {
	finish := func() error {
		if err := g.IncDepth(); err != nil {
			return err
		}
		return out.AddGoal(g)
	}
	m := t.manager
	change := true
	for change {
		change = false
		for _, forward := range [2]bool{true, false} {
			t.forward = forward
			compiled, err := t.domInit(g)
			if err != nil {
				return err
			}
			if !compiled {
				return finish()
			}
			sz, err := g.Size()
			if err != nil {
				return err
			}
			proofs, err := g.ProofsEnabled()
			if err != nil {
				return err
			}
			for k := uint32(0); k < sz; k++ {
				i := k
				if !forward {
					i = sz - 1 - k
				}
				inconsistent, err := g.Inconsistent()
				if err != nil {
					return err
				}
				if inconsistent {
					break
				}
				old, err := g.Form(i)
				if err != nil {
					return err
				}
				r, next, err := t.domRec(old, 0) // expr_ref r = simplify_rec(g.form(i))
				if err != nil {
					return fmt.Errorf("dom recurse goal form %d: %w", i, err)
				}
				err = t.domGoalStep(g, i, sz, forward, proofs, old, next, &change)
				if err = errors.Join(err, m.DelRef(&r)); err != nil {
					return err
				}
			}
			if err = t.domSimp.Pop(t.domSimp.ScopeLevel()); err != nil {
				return err
			}
		}
	}
	return finish()
}

// domGoalStep is the body of the simplify_goal loops after simplify_rec.
func (t *DomSimplifyTactic) domGoalStep(g *tactic.Goal, i, sz uint32, forward, proofs bool, old, next ast.ASTHandle, change *bool) error {
	m := t.manager
	dep, err := g.Dependency(i)
	if err != nil {
		return err
	}
	if (forward && i+1 < sz || !forward && i > 0) && !m.IsTrue(next) && !m.IsFalse(next) && dep.IsNull() && !proofs {
		ok, err := t.domSimp.AssertExpression(next, false)
		if err != nil {
			return err
		}
		if !ok {
			if next, err = m.MkFalse(); err != nil {
				return err
			}
		}
	}
	if next != old {
		t.traceRewrite(old, next)
		*change = true
	}
	pr, err := g.Proof(i)
	if err != nil {
		return err
	}
	if proofs {
		rw, err := m.MkProofRewrite(old, next)
		if err != nil {
			return err
		}
		if pr, err = m.MkProofModusPonens(pr, rw); err != nil {
			return err
		}
	}
	if err = g.Update(i, next, pr, dep); err != nil {
		return fmt.Errorf("dom update goal form %d: %w", i, err)
	}
	return nil
}

// domMkAndArgs is ast_util.h mk_and(expr_ref_vector const&).
func domMkAndArgs(m *ast.ASTManager, args []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return m.MkTrue()
	case 1:
		return args[0], nil
	default:
		return m.MkAndArgs(args)
	}
}

// domMkOrArgs is ast_util.h mk_or(expr_ref_vector const&).
func domMkOrArgs(m *ast.ASTManager, args []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return m.MkFalse()
	case 1:
		return args[0], nil
	default:
		return m.MkOrArgs(args)
	}
}

// domCached is get_cached without the expr_ref.
func (t *DomSimplifyTactic) domCached(h ast.ASTHandle) ast.ASTHandle {
	if r, ok := t.domCache[h]; ok {
		return r
	}
	return h
}

// domArg is simplify_arg. The caller owns the returned reference.
func (t *DomSimplifyTactic) domArg(e ast.ASTHandle) (ast.ASTRef, ast.ASTHandle, error) {
	m := t.manager
	r, err := m.NewRef(t.domCached(e))
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, err
	}
	if err = t.domSimp.Rewrite(r); err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, errors.Join(fmt.Errorf("dom callback rewrite: %w", err), m.DelRef(&r))
	}
	h, err := m.RefGet(r)
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, errors.Join(err, m.DelRef(&r))
	}
	return r, h, nil
}

// domRec is simplify_rec. The caller owns the returned reference.
func (t *DomSimplifyTactic) domRec(e0 ast.ASTHandle, depth uint32) (ast.ASTRef, ast.ASTHandle, error) {
	m := t.manager
	none := func(err error) (ast.ASTRef, ast.ASTHandle, error) { return ast.ASTRef{}, ast.ASTHandle{}, err }
	e := t.domCached(e0)
	var r ast.ASTRef
	var err error
	ite, and, or := false, false, false
	if depth < t.maxDepth {
		if ite, err = domOp(m, e, ast.ASTOpIte); err != nil {
			return none(err)
		}
		if and, err = domOp(m, e, ast.ASTOpAnd); err != nil {
			return none(err)
		}
		if or, err = domOp(m, e, ast.ASTOpOr); err != nil {
			return none(err)
		}
	}
	switch {
	case depth >= t.maxDepth:
		r, err = m.NewRef(e)
	case ite:
		r, err = t.domITE(e, depth)
	case and:
		r, err = t.domAndOr(e, depth, true)
	case or:
		r, err = t.domAndOr(e, depth, false)
	default:
		// The root is its own immediate dominator, so tree(root) holds the
		// root, and the source recurses on it until m_max_depth. That
		// re-simplifies the root under facts derived from the root itself
		// and is unsound in z3 4.7.1 (AUDIT.md FUZZ_B-F6), so the self edge
		// is skipped here, as display() skips it. Owner decision pending.
		for _, child := range t.dominators.tree[e] {
			if child == e {
				continue
			}
			cr, _, x := t.domRec(child, depth+1)
			if x != nil {
				return none(x)
			}
			if x = m.DelRef(&cr); x != nil {
				return none(x)
			}
		}
		kind, x := m.GetKind(e)
		if x != nil {
			return none(x)
		}
		if kind != ast.ASTApp {
			r, err = m.NewRef(e)
			break
		}
		if err = t.releaseRefs(t.domArgs); err != nil { // m_args.reset()
			return none(err)
		}
		t.domArgs = t.domArgs[:0]
		n, x := m.GetNumArgs(e)
		if x != nil {
			return none(x)
		}
		args := make([]ast.ASTHandle, n)
		for i := uint32(0); i < n; i++ {
			a, x := m.GetArg(e, i)
			if x != nil {
				return none(x)
			}
			ar, ah, x := t.domArg(a)
			if x != nil {
				return none(x)
			}
			t.domArgs = append(t.domArgs, ar) // m_args.push_back(simplify_arg(arg))
			args[i] = ah
		}
		decl, x := m.GetDecl(e)
		if x != nil {
			return none(x)
		}
		h, x := m.MkApp(decl, args)
		if x != nil {
			return none(x)
		}
		r, err = m.NewRef(h)
	}
	if err != nil {
		return none(err)
	}
	if err = t.domSimp.Rewrite(r); err != nil { // (*m_simplifier)(r)
		return none(errors.Join(fmt.Errorf("dom callback rewrite: %w", err), m.DelRef(&r)))
	}
	h, err := m.RefGet(r)
	if err != nil {
		return none(errors.Join(err, m.DelRef(&r)))
	}
	// cache(e0, r): m_result.insert(e0, r); m_trail.push_back(r).
	tr, err := m.NewRef(h)
	if err != nil {
		return none(errors.Join(err, m.DelRef(&r)))
	}
	t.domCache[e0] = h
	t.domTrail = append(t.domTrail, tr)
	return r, h, nil
}

// domSubexpr is is_subexpr. m_subexpr_cache only memoizes it.
func (t *DomSimplifyTactic) domSubexpr(a, b ast.ASTHandle) (bool, error) {
	for {
		if a == b {
			return true, nil
		}
		ad, e := t.manager.GetDepth(a)
		if e != nil {
			return false, e
		}
		bd, e := t.manager.GetDepth(b)
		if e != nil {
			return false, e
		}
		if ad >= bd {
			return false, nil
		}
		p, ok, e := t.dominators.idom(a)
		if e != nil {
			return false, e
		}
		if !ok || p == a {
			return false, ast.ErrASTDomain
		}
		a = p
	}
}

// domExclusive is the is_subexpr_arg lambda of simplify_and_or.
func (t *DomSimplifyTactic) domExclusive(child, except ast.ASTHandle, args []ast.ASTHandle) (bool, error) {
	ok, e := t.domSubexpr(child, except)
	if e != nil || !ok {
		return ok, e
	}
	for _, arg := range args {
		if arg == except {
			continue
		}
		yes, x := t.domSubexpr(child, arg)
		if x != nil {
			return false, x
		}
		if yes {
			return false, nil
		}
	}
	return true, nil
}

// domAndOr is simplify_and_or. The caller owns the returned reference.
func (t *DomSimplifyTactic) domAndOr(e ast.ASTHandle, depth uint32, and bool) (r ast.ASTRef, err error) {
	m := t.manager
	n, err := m.GetNumArgs(e)
	if err != nil {
		return ast.ASTRef{}, err
	}
	orig := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		if orig[i], err = m.GetArg(e, i); err != nil {
			return ast.ASTRef{}, err
		}
	}
	old := t.domSimp.ScopeLevel()
	held := make([]ast.ASTRef, 0, n) // expr_ref_vector args(m)
	args := make([]ast.ASTHandle, 0, n)
	defer func() {
		err = errors.Join(err, t.releaseRefs(held))
		if err != nil && r != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&r))
		}
	}()
	for k := uint32(0); k < n; k++ {
		i := k
		if !t.forward {
			i = n - 1 - k
		}
		arg := orig[i]
		for _, child := range t.dominators.tree[arg] {
			yes, x := t.domExclusive(child, arg, orig)
			if x != nil {
				return ast.ASTRef{}, x
			}
			if yes {
				cr, _, x := t.domRec(child, depth+1)
				if x != nil {
					return ast.ASTRef{}, x
				}
				if x = m.DelRef(&cr); x != nil {
					return ast.ASTRef{}, x
				}
			}
		}
		ar, ah, x := t.domArg(arg)
		if x != nil {
			return ast.ASTRef{}, x
		}
		held = append(held, ar)
		args = append(args, ah)
		ok, x := t.domSimp.AssertExpression(ah, !and)
		if x != nil {
			return ast.ASTRef{}, x
		}
		if !ok {
			var h ast.ASTHandle
			if and {
				h, x = m.MkFalse()
			} else {
				h, x = m.MkTrue()
			}
			if x != nil {
				return ast.ASTRef{}, x
			}
			return m.NewRef(h)
		}
	}
	if !t.forward { // args.reverse()
		for i, j := 0, len(args)-1; i < j; i, j = i+1, j-1 {
			args[i], args[j] = args[j], args[i]
			held[i], held[j] = held[j], held[i]
		}
	}
	if err = t.domSimp.Pop(t.domSimp.ScopeLevel() - old); err != nil {
		return ast.ASTRef{}, err
	}
	var h ast.ASTHandle
	if and {
		h, err = domMkAndArgs(m, args)
	} else {
		h, err = domMkOrArgs(m, args)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(h)
}

// domITE is simplify_ite. The caller owns the returned reference.
func (t *DomSimplifyTactic) domITE(ite ast.ASTHandle, depth uint32) (r ast.ASTRef, err error) {
	m := t.manager
	c, err := m.GetArg(ite, 0)
	if err != nil {
		return ast.ASTRef{}, err
	}
	th, err := m.GetArg(ite, 1)
	if err != nil {
		return ast.ASTRef{}, err
	}
	el, err := m.GetArg(ite, 2)
	if err != nil {
		return ast.ASTRef{}, err
	}
	old := t.domSimp.ScopeLevel()
	newC, nc, err := t.domArg(c) // expr_ref new_c
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() {
		err = errors.Join(err, m.DelRef(&newC))
		if err != nil && r != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&r))
		}
	}()
	if m.IsTrue(nc) {
		r, _, err = t.domArg(th)
		return r, err
	}
	if m.IsFalse(nc) {
		r, _, err = t.domArg(el)
		return r, err
	}
	ok, err := t.domSimp.AssertExpression(nc, false)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !ok {
		r, _, err = t.domArg(el)
		return r, err
	}
	branch := func(in, out ast.ASTHandle) error {
		for _, child := range t.dominators.tree[ite] {
			a, x := t.domSubexpr(child, in)
			if x != nil {
				return x
			}
			if !a {
				continue
			}
			b, x := t.domSubexpr(child, out)
			if x != nil {
				return x
			}
			if b {
				continue
			}
			cr, _, x := t.domRec(child, depth+1)
			if x != nil {
				return x
			}
			if x = m.DelRef(&cr); x != nil {
				return x
			}
		}
		return t.domSimp.Pop(t.domSimp.ScopeLevel() - old)
	}
	if err = branch(th, el); err != nil {
		return ast.ASTRef{}, err
	}
	newT, nt, err := t.domArg(th) // expr_ref new_t
	if err != nil {
		return ast.ASTRef{}, err
	}
	ok, err = t.domSimp.AssertExpression(nc, true)
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&newT))
	}
	if !ok {
		return newT, nil
	}
	if err = branch(el, th); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&newT))
	}
	newE, ne, err := t.domArg(el) // expr_ref new_e
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&newT))
	}
	h := ite
	switch {
	case c == nc && th == nt && el == ne:
	case nt == ne:
		h = nt
	default:
		h, err = m.MkIte(nc, nt, ne)
	}
	if err == nil {
		r, err = m.NewRef(h)
	}
	// new_e, then new_t, leave scope before new_c.
	err = errors.Join(err, m.DelRef(&newE), m.DelRef(&newT))
	return r, err
}
