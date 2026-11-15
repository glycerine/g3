package nlsatsmt

// Source: src/tactic/nlsat_smt/nl_purify_tactic.cpp, rw_cfg Boolean mode.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// nlPredicateRewriter abstracts arithmetic atoms into Boolean interface
// predicates, adding the polarity-directed clauses to the nonlinear goal.
type nlPredicateRewriter struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	real       ast.ASTHandle
	nonlinear  *tactic.Goal
	polarities map[ast.ASTHandle]polarity
	// rw: the rewriter_tpl<rw_cfg> in Boolean-predicate mode.
	config        nlPredicateConfig
	traversal     *rewriter.Traversal
	out           ast.ASTRef
	refs          []ast.ASTRef
	newPredicates []ast.ASTHandle
}

func newNLPredicateRewriter(m *ast.ASTManager, nonlinear *tactic.Goal, polarities map[ast.ASTHandle]polarity) (*nlPredicateRewriter, error) {
	if m == nil || nonlinear == nil || nonlinear.Manager() != m || polarities == nil {
		return nil, ast.ErrASTDomain
	}
	if err := nonlinear.CheckOwner(); err != nil {
		return nil, err
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	real, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		return nil, err
	}
	r := &nlPredicateRewriter{manager: m, arithmetic: family, real: real, nonlinear: nonlinear, polarities: polarities}
	r.config.owner = r
	if r.out, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return nil, err
	}
	if r.traversal, err = rewriter.NewTraversal(m, &r.config); err != nil {
		return nil, errors.Join(err, r.Close())
	}
	return r, nil
}

// nlPredicateConfig is rw_cfg in mode_bool_preds.
type nlPredicateConfig struct {
	rewriter.DefaultConfiguration
	owner *nlPredicateRewriter
}

func (c *nlPredicateConfig) ReduceQuantifier(ast.ASTHandle, ast.ASTHandle, ast.AppArguments, ast.AppArguments, ast.ASTRef, ast.ASTRef) (bool, error) {
	return false, tactic.NewTacticException("quantifiers are not supported in mixed-mode nlsat engine")
}

// ReduceApp is reduce_app_bool.
func (c *nlPredicateConfig) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	r := c.owner
	m := r.manager
	family, err := m.GetFamilyID(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(d)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	match := family == r.arithmetic && op >= 2 && op <= 5
	if family == m.BasicFamilyID() {
		match = false
		if op == ast.ASTOpEq && args.Count() > 0 {
			first, e := args.At(0)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			sort, e := m.GetSort(first)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			match = sort == r.real
		}
	}
	if !match {
		return rewriter.RewriteFailed, nil
	}
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		if values[i], err = args.At(uint32(i)); err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	// expr_ref old_pred(m.mk_app(f, num, args)): polarity is looked up on
	// the rebuilt application.
	atom, err := m.MkApp(d, values)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = r.hold(atom); err != nil {
		return rewriter.RewriteFailed, err
	}
	value, err := r.abstract(atom)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, m.SetRef(out, value)
}

func (r *nlPredicateRewriter) hold(h ast.ASTHandle) error {
	ref, err := r.manager.NewRef(h)
	if err == nil {
		r.refs = append(r.refs, ref)
	}
	return err
}

func (r *nlPredicateRewriter) abstract(atom ast.ASTHandle) (ast.ASTHandle, error) {
	pol, ok := r.polarities[atom]
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	boolean, err := r.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pred, err := r.manager.MkFreshConstNullable(nil, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = r.hold(pred); err != nil {
		return ast.ASTHandle{}, err
	}
	if pol != negative {
		notPred, e := r.manager.MkNot(pred)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.hold(notPred); e != nil {
			return ast.ASTHandle{}, e
		}
		clause, e := r.manager.MkOr(notPred, atom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.nonlinear.AssertWithProof(clause, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return ast.ASTHandle{}, e
		}
	}
	if pol != positive {
		notAtom, e := r.manager.MkNot(atom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.hold(notAtom); e != nil {
			return ast.ASTHandle{}, e
		}
		clause, e := r.manager.MkOr(pred, notAtom)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if e = r.nonlinear.AssertWithProof(clause, ast.ASTHandle{}, ast.ASTDependency{}); e != nil {
			return ast.ASTHandle{}, e
		}
	}
	r.polarities[pred] = pol
	r.newPredicates = append(r.newPredicates, pred)
	return pred, nil
}

func (r *nlPredicateRewriter) rewrite(term ast.ASTHandle) (ast.ASTHandle, error) {
	if r == nil || r.manager == nil || r.traversal == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if err := r.traversal.Rewrite(term, r.out); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := r.manager.RefGet(r.out)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if value != term {
		if err = r.hold(value); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return value, nil
}

func (r *nlPredicateRewriter) rewriteGoal(g *tactic.Goal) error {
	if r == nil || r.manager == nil || g == nil || g == r.nonlinear || g.Manager() != r.manager {
		return ast.ErrASTDomain
	}
	if err := g.CheckOwner(); err != nil {
		return err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		return tactic.NewTacticException("qfufnra-purify does not support proof generation")
	}
	// rewrite_goal starts with r.reset(): the cache is cleared.
	if err = r.traversal.Reset(); err != nil {
		return err
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		old, e := g.Form(i)
		if e != nil {
			return e
		}
		value, e := r.rewrite(old)
		if e != nil {
			return e
		}
		dependency, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, value, ast.ASTHandle{}, dependency); e != nil {
			return e
		}
	}
	return nil
}

func (r *nlPredicateRewriter) Close() (result error) {
	if r == nil || r.manager == nil {
		return nil
	}
	if r.traversal != nil {
		result = r.traversal.Close()
		r.traversal = nil
	}
	if r.out != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.out))
	}
	for i := len(r.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.refs[i]))
	}
	r.manager = nil
	r.refs = nil
	return result
}
