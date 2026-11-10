package opt

// Source: src/opt/maxres.cpp max_resolve, mk_fresh_bool, update_model.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
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

// assertDef is s().assert_expr(fml); m_defs.push_back(fml). The caller has
// already given fml to the engine owner (m_defs keeps it).
func (e *maxResEngine) assertDef(fml ast.ASTHandle) error {
	if err := e.solver.solver().Assert(fml); err != nil {
		return err
	}
	e.defs = append(e.defs, fml)
	return nil
}

// updateModel is update_model(def, value): with a correction-set model,
// value is evaluated there (model completion on) and registered as def's
// interpretation. Without one nothing happens, and value is not retained.
func (e *maxResEngine) updateModel(def, value ast.ASTHandle) (resultErr error) {
	md := e.correctionModel
	if md == nil {
		return nil
	}
	m := e.solver.manager
	evaluator, err := model.NewModelEvaluator(md, e.solver.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	ref, err := evaluator.EvaluateValue(value)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&ref)) }()
	val, err := m.RefGet(ref)
	if err != nil {
		return err
	}
	decl, err := m.GetDecl(def)
	if err != nil {
		return err
	}
	return md.RegisterConstant(decl, val)
}

func (e *maxResEngine) implies(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return e.owner.hold(e.solver.manager.MkImplies(a, b))
}

// maxResLocal is an expr_ref/app_ref local: assignment takes the new value
// before releasing the old one, and release ends the function's scope.
type maxResLocal struct {
	m   *ast.ASTManager
	ref ast.ASTRef
}

func (l *maxResLocal) set(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := l.m.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	old := l.ref
	l.ref = ref
	if old != (ast.ASTRef{}) {
		if err = l.m.DelRef(&old); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return h, nil
}

func (l *maxResLocal) release() error {
	if l.ref == (ast.ASTRef{}) {
		return nil
	}
	return l.m.DelRef(&l.ref)
}

// maxResolve is max_resolve:
//
//	d_0 := true
//	d_i := b_{i-1} and d_{i-1}    for i = 1...sz-1
//	soft (b_i or !d_i)
//
// fml, asum, cls, d and dd are the source's locals; only m_defs, m_trail
// and m_asms keep terms past the call.
func (e *maxResEngine) maxResolve(core []ast.ASTHandle, weight *big.Rat) (resultErr error) {
	if len(core) == 0 || weight == nil || weight.Sign() <= 0 {
		return errMaxResState
	}
	m := e.solver.manager
	fml, asum, cls := &maxResLocal{m: m}, &maxResLocal{m: m}, &maxResLocal{m: m}
	d, dd := &maxResLocal{m: m}, &maxResLocal{m: m}
	defer func() {
		// Destruction runs in reverse declaration order: dd, d, cls, asum, fml.
		for _, l := range []*maxResLocal{dd, d, cls, asum, fml} {
			resultErr = errors.Join(resultErr, l.release())
		}
	}()
	for i := 1; i < len(core); i++ {
		bi, bi1 := core[i-1], core[i]
		switch {
		case i == 1:
			if _, err := d.set(bi, nil); err != nil {
				return err
			}
		case i == 2:
			current, err := m.RefGet(d.ref)
			if err != nil {
				return err
			}
			next, err := d.set(m.MkAnd(bi, current))
			if err != nil {
				return err
			}
			if _, err = e.owner.hold(next, nil); err != nil { // m_trail
				return err
			}
		default:
			fresh, err := e.freshBool("d")
			if err == nil {
				_, err = dd.set(fresh, nil)
			}
			if err != nil {
				return err
			}
			current, err := m.RefGet(d.ref)
			if err != nil {
				return err
			}
			h, err := fml.set(e.implies(fresh, current))
			if err == nil {
				err = e.assertDef(h)
			}
			if err == nil {
				h, err = fml.set(e.implies(fresh, bi))
			}
			if err == nil {
				err = e.assertDef(h)
			}
			if err == nil {
				h, err = fml.set(m.MkAnd(current, bi))
			}
			if err == nil {
				err = e.updateModel(fresh, h)
			}
			if err == nil {
				_, err = d.set(fresh, nil)
			}
			if err != nil {
				return err
			}
		}
		fresh, err := e.freshBool("a")
		if err == nil {
			_, err = asum.set(fresh, nil)
		}
		if err != nil {
			return err
		}
		current, err := m.RefGet(d.ref)
		if err != nil {
			return err
		}
		clause, err := cls.set(m.MkOr(bi1, current))
		if err != nil {
			return err
		}
		h, err := fml.set(e.implies(fresh, clause))
		if err == nil {
			err = e.updateModel(fresh, clause)
		}
		if err == nil {
			err = e.addAssumption(fresh, weight)
		}
		if err == nil {
			err = e.assertDef(h)
		}
		if err != nil {
			return err
		}
		e.stats.Resolutions++
	}
	return nil
}
