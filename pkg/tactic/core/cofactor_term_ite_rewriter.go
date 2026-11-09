package core

// Source: src/tactic/core/cofactor_elim_term_ite.cpp, retained bottom-up path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// cofactorTermITERewriter is bottom_up_elim, built per formula; cofactor is
// its cofactor_rw (a rewriter_tpl whose cache persists across calls).
type cofactorTermITERewriter struct {
	owner    *CofactorTermITETactic
	refs     contextSimplifyRefs
	memo     map[ast.ASTHandle]ast.ASTHandle
	cfg      cofactorRWConfig
	cofactor *rewriter.Traversal
}

// cofactorRWConfig is cofactor_rw_cfg.
type cofactorRWConfig struct {
	rewriter.DefaultConfiguration
	owner   *cofactorTermITERewriter
	mkApp   *rewriter.SimplifiedApp
	atom    ast.ASTHandle
	sign    bool
	term    ast.ASTHandle
	value   ast.ASTHandle
	hasTerm bool
}

func (c *cofactorRWConfig) MaxStepsExceeded(uint32) (bool, error) {
	return false, c.owner.checkpoint()
}
func (c *cofactorRWConfig) PreVisit(ast.ASTHandle) (bool, error) { return true, nil }
func (c *cofactorRWConfig) RewritePatterns() bool                { return false }
func (c *cofactorRWConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	if proof != (ast.ASTRef{}) {
		if err := c.owner.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	return c.mkApp.MkCore(d, args, out)
}
func (c *cofactorRWConfig) GetSubst(s ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	m := c.owner.owner.manager
	if s == c.atom {
		var v ast.ASTHandle
		var err error
		if c.sign {
			v, err = m.MkFalse()
		} else {
			v, err = m.MkTrue()
		}
		return v, ast.ASTHandle{}, err == nil, err
	}
	if c.hasTerm && s == c.term && !c.value.IsNull() {
		return c.value, ast.ASTHandle{}, true, nil
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
}

// setCofactorAtom is cofactor_rw_cfg::set_cofactor_atom; m_value is left
// as it was, as in the source.
func (c *cofactorRWConfig) setCofactorAtom(t ast.ASTHandle) error {
	m := c.owner.owner.manager
	negated, argument, err := m.IsNot(t)
	if err != nil {
		return err
	}
	c.hasTerm = false
	if negated {
		c.atom, c.sign = argument, true
		return nil
	}
	c.atom, c.sign = t, false
	if !c.owner.owner.cofactorEqualities {
		return nil
	}
	equality, err := c.owner.isEquality(t)
	if err != nil || !equality {
		return err
	}
	lhs, err := m.GetArg(t, 0)
	if err != nil {
		return err
	}
	rhs, err := m.GetArg(t, 1)
	if err != nil {
		return err
	}
	if unique, e := m.IsUniqueValue(lhs); e != nil {
		return e
	} else if unique {
		c.term, c.value, c.hasTerm = rhs, lhs, true
		return nil
	}
	if unique, e := m.IsUniqueValue(rhs); e != nil {
		return e
	} else if unique {
		c.term, c.value, c.hasTerm = lhs, rhs, true
	}
	return nil
}

func newCofactorTermITERewriter(owner *CofactorTermITETactic) (*cofactorTermITERewriter, error) {
	r := &cofactorTermITERewriter{owner: owner, refs: contextSimplifyRefs{m: owner.manager}, memo: make(map[ast.ASTHandle]ast.ASTHandle)}
	r.cfg.owner = r
	var err error
	r.cfg.mkApp, err = rewriter.NewSimplifiedApp(owner.manager, owner.context, owner.globals, &owner.params)
	if err != nil {
		if r.cfg.mkApp != nil {
			err = errors.Join(err, r.cfg.mkApp.Close())
		}
		return nil, err
	}
	if r.cofactor, err = rewriter.NewTraversal(owner.manager, &r.cfg); err != nil {
		return nil, errors.Join(err, r.cfg.mkApp.Close())
	}
	return r, nil
}

func (r *cofactorTermITERewriter) Close() error {
	return errors.Join(r.cofactor.Close(), r.cfg.mkApp.Close(), r.refs.Close())
}

func (r *cofactorTermITERewriter) checkpoint() error {
	if err := r.owner.services.Cooperation.Cooperate("cofactor ite"); err != nil {
		return err
	}
	n, err := r.owner.services.Memory.AllocationSize()
	if err != nil {
		return err
	}
	if n > r.owner.maxMemory {
		return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return nil
}

func (r *cofactorTermITERewriter) pin(h ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := r.refs.newRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.owner.manager.RefGet(ref)
}

func (r *cofactorTermITERewriter) isTermITE(h ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil || op != ast.ASTOpIte {
		return false, err
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	return sort != boolean, err
}

func (r *cofactorTermITERewriter) isAtom(h ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil || sort != boolean {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return family != m.BasicFamilyID(), err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return false, err
	}
	switch op {
	case ast.ASTOpEq, ast.ASTOpDistinct:
		first, e := m.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		firstSort, e := m.GetSort(first)
		return firstSort != boolean, e
	case ast.ASTOpTrue, ast.ASTOpFalse, ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpNot,
		ast.ASTOpXor, ast.ASTOpImplies, ast.ASTOpIff, ast.ASTOpIte:
		return false, nil
	default:
		return true, nil
	}
}

func (r *cofactorTermITERewriter) isEquality(h ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	op, err := m.GetDeclKind(h)
	return op == ast.ASTOpEq, err
}

func (r *cofactorTermITERewriter) bestCondition(root ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := r.owner.manager
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	counts := make(map[ast.ASTHandle]uint32)
	var order []ast.ASTHandle
	for len(stack) != 0 {
		if err := r.checkpoint(); err != nil {
			return ast.ASTHandle{}, false, err
		}
		i := len(stack) - 1
		current := stack[i]
		stack = stack[:i]
		if seen[current] {
			continue
		}
		seen[current] = true
		yes, err := r.isTermITE(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if yes {
			condition, e := m.GetArg(current, 0)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if counts[condition] == 0 {
				order = append(order, condition)
			}
			counts[condition]++
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		n, err := m.GetNumArgs(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		for j := n; j > 0; j-- {
			child, e := m.GetArg(current, j-1)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			stack = append(stack, child)
		}
	}
	var best ast.ASTHandle
	var bestDepth, bestCount uint32
	for _, condition := range order {
		depth, err := m.GetDepth(condition)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		count := counts[condition]
		choose := best.IsNull() || depth < bestDepth || depth == bestDepth && count > bestCount
		if !choose && depth == bestDepth && count == bestCount {
			equality, e1 := r.isEquality(condition)
			bestEquality, e2 := r.isEquality(best)
			if e := errors.Join(e1, e2); e != nil {
				return ast.ASTHandle{}, false, e
			}
			choose = equality && !bestEquality
		}
		if choose {
			best, bestDepth, bestCount = condition, depth, count
		}
	}
	return best, !best.IsNull(), nil
}

// applyCofactor is m_cofactor(curr, out) after set_cofactor_atom(atom).
func (r *cofactorTermITERewriter) applyCofactor(curr, atom ast.ASTHandle, out ast.ASTRef) error {
	if err := r.cfg.setCofactorAtom(atom); err != nil {
		return err
	}
	// The source keeps the rewriter cache across set_cofactor_atom calls;
	// results cached under one substitution are wrong under the next, and
	// Go's extra holding references make more subterms cacheable, so the
	// cache is cleared for each cofactor.
	if err := r.cofactor.Reset(); err != nil {
		return err
	}
	return r.cofactor.Rewrite(curr, out)
}

// eliminateAtom is bottom_up_elim::cofactor(t, r).
func (r *cofactorTermITERewriter) eliminateAtom(atom ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	m := r.owner.manager
	curr, err := m.NewRef(atom)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&curr)) }()
	for {
		current, err := m.RefGet(curr)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		condition, found, err := r.bestCondition(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !found {
			return r.pin(current)
		}
		if err = r.cofactorStep(curr, current, condition); err != nil {
			return ast.ASTHandle{}, err
		}
	}
}

// cofactorStep is one iteration of the cofactor loop; pos_cofactor,
// neg_cofactor and neg_c are its expr_ref locals.
func (r *cofactorTermITERewriter) cofactorStep(curr ast.ASTRef, current, c ast.ASTHandle) (resultErr error) {
	m := r.owner.manager
	var locals [3]ast.ASTRef // pos_cofactor, neg_cofactor, neg_c
	for i := range locals {
		ref, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		locals[i] = ref
	}
	defer func() {
		for i := len(locals) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&locals[i]))
		}
	}()
	if err := r.applyCofactor(current, c, locals[0]); err != nil {
		return err
	}
	negated, argument, err := m.IsNot(c)
	if err != nil {
		return err
	}
	negC := argument
	if !negated {
		if negC, err = m.MkNot(c); err != nil {
			return err
		}
	}
	if err = m.SetRef(locals[2], negC); err != nil {
		return err
	}
	if err = r.applyCofactor(current, negC, locals[1]); err != nil {
		return err
	}
	pos, err := m.RefGet(locals[0])
	if err != nil {
		return err
	}
	neg, err := m.RefGet(locals[1])
	if err != nil {
		return err
	}
	var next ast.ASTHandle
	switch {
	case pos == neg:
		next = pos
	case m.IsTrue(pos) && m.IsFalse(neg):
		next = c
	case m.IsFalse(pos) && m.IsTrue(neg):
		next = negC
	default:
		if next, err = m.MkIte(c, pos, neg); err != nil {
			return err
		}
	}
	return m.SetRef(curr, next)
}

func (r *cofactorTermITERewriter) rewrite(root ast.ASTHandle) (ast.ASTHandle, error) {
	if value, ok := r.memo[root]; ok {
		return value, nil
	}
	if err := r.checkpoint(); err != nil {
		return ast.ASTHandle{}, err
	}
	m := r.owner.manager
	kind, err := m.GetKind(root)
	if err != nil || kind != ast.ASTApp {
		return root, err
	}
	n, err := m.GetNumArgs(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for i := range args {
		child, e := m.GetArg(root, uint32(i))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		args[i], e = r.rewrite(child)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		changed = changed || args[i] != child
	}
	current := root
	if changed {
		decl, e := m.GetDecl(root)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		current, e = m.MkApp(decl, args)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		current, e = r.pin(current)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	atom, err := r.isAtom(current)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if atom {
		current, err = r.eliminateAtom(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	r.memo[root] = current
	return current, nil
}
