package opt

// Source: src/opt/maxres.cpp add_soft and max_resolve fresh assumptions.

import (
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
	definition := ast.ASTHandle{}
	if equivalent {
		definition, err = e.owner.hold(e.solver.manager.MkIff(proxy, formula))
	} else {
		definition, err = e.owner.hold(e.solver.manager.MkImplies(proxy, formula))
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = e.solver.solver().Assert(definition); err != nil {
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
