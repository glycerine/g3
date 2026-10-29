package clp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *Engine) tryRule(rule *base.Rule, goals []ast.ASTHandle, depth uint32, index int) (result lbool.LBool, resultErr error) {
	head, err := rule.Head()
	if err != nil {
		return lbool.Undef, err
	}
	ground, err := e.ground(head)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&ground)) }()
	gh, err := e.manager.RefGet(ground)
	if err != nil {
		return lbool.Undef, err
	}
	n, err := e.manager.GetNumArgs(gh)
	if err != nil {
		return lbool.Undef, err
	}
	for i := uint32(0); i < n; i++ {
		a, x := e.manager.GetArg(goals[index], i)
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
		if x = e.solver.Assert(eq); x != nil {
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
	next := append([]ast.ASTHandle(nil), goals...)
	owned := []ast.ASTRef{}
	defer func() {
		for i := range owned {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&owned[i]))
		}
	}()
	for i := u; i < total; i++ {
		tail, _, x := rule.Tail(i)
		if x != nil {
			return lbool.Undef, x
		}
		g, x := e.ground(tail)
		if x != nil {
			return lbool.Undef, x
		}
		owned = append(owned, g)
		h, x := e.manager.RefGet(owned[len(owned)-1])
		if x != nil {
			return lbool.Undef, x
		}
		if x = e.solver.Assert(h); x != nil {
			return lbool.Undef, x
		}
	}
	sat, err := e.solver.CheckSat(nil)
	if err != nil || sat != lbool.True {
		return sat, err
	}
	if depth == 1 && u > 0 {
		return lbool.Undef, nil
	}
	for i := 0; i < u; i++ {
		tail, _, x := rule.Tail(i)
		if x != nil {
			return lbool.Undef, x
		}
		g, x := e.ground(tail)
		if x != nil {
			return lbool.Undef, x
		}
		owned = append(owned, g)
		h, x := e.manager.RefGet(owned[len(owned)-1])
		if x != nil {
			return lbool.Undef, x
		}
		next = append(next, h)
	}
	return e.search(next, depth-1, index+1)
}
