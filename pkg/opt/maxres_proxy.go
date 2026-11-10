package opt

// Source: src/opt/maxres.cpp add_soft and max_resolve fresh assumptions.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) freshProxy(formula ast.ASTHandle, equivalent bool, prefix string) (ast.ASTHandle, error) {
	boolean, err := e.solver.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proxy, err := e.owner.hold(e.solver.manager.MkFreshConst(prefix, boolean))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.filterProxy(proxy); err != nil {
		return ast.ASTHandle{}, err
	}
	// add_soft: fml = m.mk_iff(asum, e); s().assert_expr(fml); fml is a
	// local, so only the solver keeps the definition (until its pop).
	fml := &maxResLocal{m: e.solver.manager}
	var definition ast.ASTHandle
	if equivalent {
		definition, err = fml.set(e.solver.manager.MkIff(proxy, formula))
	} else {
		definition, err = fml.set(e.solver.manager.MkImplies(proxy, formula))
	}
	if err == nil {
		err = e.solver.solver().Assert(definition)
	}
	if err = errors.Join(err, fml.release()); err != nil {
		return ast.ASTHandle{}, err
	}
	if e.proxies == nil {
		e.proxies = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	e.proxies[proxy] = formula
	return proxy, nil
}

func (e *maxResEngine) addSoft(expression ast.ASTHandle, weight *big.Rat) error {
	literal, err := maxResLiteral(e.solver.manager, expression)
	if err != nil {
		return err
	}
	if !literal {
		expression, err = e.freshProxy(expression, true, "soft")
		if err != nil {
			return err
		}
	}
	return e.addAssumption(expression, weight)
}
