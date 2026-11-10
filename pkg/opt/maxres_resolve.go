package opt

// Source: src/opt/maxres.cpp max_resolve, mk_fresh_bool, update_model.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// freshBool is mk_fresh_bool: a fresh Boolean constant whose declaration
// the filter model converter hides.
func (e *maxResEngine) freshBool(prefix string) (ast.ASTHandle, error) {
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
	return proxy, nil
}

// assertDef is s().assert_expr(fml); m_defs.push_back(fml).
func (e *maxResEngine) assertDef(fml ast.ASTHandle) error {
	if err := e.solver.solver().Assert(fml); err != nil {
		return err
	}
	e.defs = append(e.defs, fml)
	return nil
}

// updateModel is update_model(def, value): the correction-set model
// interprets def as value.
func (e *maxResEngine) updateModel(def, value ast.ASTHandle) {
	if e.proxies == nil {
		e.proxies = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	e.proxies[def] = value
}

func (e *maxResEngine) implies(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return e.owner.hold(e.solver.manager.MkImplies(a, b))
}

// maxResolve is max_resolve:
//
//	d_0 := true
//	d_i := b_{i-1} and d_{i-1}    for i = 1...sz-1
//	soft (b_i or !d_i)
func (e *maxResEngine) maxResolve(core []ast.ASTHandle, weight *big.Rat) error {
	if len(core) == 0 || weight == nil || weight.Sign() <= 0 {
		return errMaxResState
	}
	m := e.solver.manager
	var d ast.ASTHandle
	for i := 1; i < len(core); i++ {
		bi, bi1 := core[i-1], core[i]
		switch {
		case i == 1:
			d = bi
		case i == 2:
			next, err := e.owner.hold(m.MkAnd(bi, d))
			if err != nil {
				return err
			}
			d = next
		default:
			dd, err := e.freshBool("d")
			if err != nil {
				return err
			}
			fml, err := e.implies(dd, d)
			if err == nil {
				err = e.assertDef(fml)
			}
			if err == nil {
				fml, err = e.implies(dd, bi)
			}
			if err == nil {
				err = e.assertDef(fml)
			}
			if err == nil {
				fml, err = e.owner.hold(m.MkAnd(d, bi))
			}
			if err != nil {
				return err
			}
			e.updateModel(dd, fml)
			d = dd
		}
		asum, err := e.freshBool("a")
		if err != nil {
			return err
		}
		cls, err := e.owner.hold(m.MkOr(bi1, d))
		if err != nil {
			return err
		}
		fml, err := e.implies(asum, cls)
		if err != nil {
			return err
		}
		e.updateModel(asum, cls)
		if err = e.addAssumption(asum, weight); err != nil {
			return err
		}
		if err = e.assertDef(fml); err != nil {
			return err
		}
		e.stats.Resolutions++
	}
	return nil
}
