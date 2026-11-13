package clp

// Source: src/muz/clp/clp_context.cpp clp::imp::search, one rule iteration.
// tmp is the iteration's expr_ref: each assignment releases its previous
// value, and it dies after the scope is popped.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// setTmp assigns tmp, retaining the new value before releasing the old one.
func (o *ruleAttemptOwner) setTmp(value ast.ASTHandle) error {
	if len(o.refs) == 0 {
		ref, err := o.manager.NewRef(value)
		if err != nil {
			return err
		}
		o.take(ref)
		return nil
	}
	return o.manager.SetRef(o.refs[0], value)
}

// groundTmp is ground(tmp): m_var_subst(e, m_ground, e).
func (e *Engine) groundTmp(owner *ruleAttemptOwner) (ast.ASTHandle, error) {
	current, err := e.manager.RefGet(owner.refs[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	grounded, err := e.state.ground.ground(current)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := e.manager.RefGet(grounded)
	if err == nil {
		err = owner.setTmp(value)
	}
	if cleanup := e.manager.DelRef(&grounded); err == nil {
		err = cleanup
	}
	return value, err
}

func (e *Engine) tryRule(rule *base.Rule, depth uint32, index int, owner *ruleAttemptOwner) (result lbool.LBool, resultErr error) {
	if owner == nil {
		return lbool.Undef, ErrCLP
	}
	if err := e.state.resetGround(); err != nil {
		return lbool.Undef, err
	}
	head, err := rule.Head()
	if err != nil {
		return lbool.Undef, err
	}
	if err = owner.setTmp(head); err != nil {
		return lbool.Undef, err
	}
	if err = e.verboseRule(index, head); err != nil {
		return lbool.Undef, err
	}
	gh, err := e.groundTmp(owner)
	if err != nil {
		return lbool.Undef, err
	}
	goal, err := e.state.goal(index)
	if err != nil {
		return lbool.Undef, err
	}
	n, err := e.manager.GetNumArgs(goal)
	if err != nil {
		return lbool.Undef, err
	}
	for i := uint32(0); i < n; i++ {
		a, x := e.manager.GetArg(goal, i)
		if x != nil {
			return lbool.Undef, x
		}
		b, x := e.manager.GetArg(gh, i)
		if x != nil {
			return lbool.Undef, x
		}
		eq, x := e.manager.MkEq(a, b)
		if x != nil {
			return lbool.Undef, x
		}
		eqRef, x := e.manager.NewRef(eq)
		if x != nil {
			return lbool.Undef, x
		}
		x = e.solver.Assert(eq)
		if cleanup := e.manager.DelRef(&eqRef); x == nil {
			x = cleanup
		}
		if x != nil {
			return lbool.Undef, x
		}
	}
	u, err := rule.UninterpretedTailSize()
	if err != nil {
		return lbool.Undef, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return lbool.Undef, err
	}
	for i := u; i < total; i++ {
		tail, _, x := rule.Tail(i)
		if x == nil {
			x = owner.setTmp(tail)
		}
		var g ast.ASTHandle
		if x == nil {
			g, x = e.groundTmp(owner)
		}
		if x == nil {
			x = e.solver.Assert(g)
		}
		if x != nil {
			return lbool.Undef, x
		}
	}
	sat, err := e.solver.CheckSat(nil)
	if err != nil {
		return sat, err
	}
	if sat == lbool.Undef {
		return sat, ErrSolverUnknown
	}
	if sat != lbool.True {
		return sat, nil
	}
	if depth == 1 && (index+1 > len(e.state.goals) || u > 0) {
		return lbool.Undef, nil
	}
	for i := 0; i < u; i++ {
		tail, _, x := rule.Tail(i)
		if x == nil {
			x = owner.setTmp(tail)
		}
		var g ast.ASTHandle
		if x == nil {
			g, x = e.groundTmp(owner)
		}
		var ref ast.ASTRef
		if x == nil {
			ref, x = e.manager.NewRef(g)
		}
		if x != nil {
			return lbool.Undef, x
		}
		e.state.goals = append(e.state.goals, ref)
	}
	return e.search(depth-1, index+1)
}
