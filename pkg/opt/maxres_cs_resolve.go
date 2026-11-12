package opt

// Source: src/opt/maxres.cpp cs_max_resolve.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// correctionResolve is cs_max_resolve for a correction set:
//
//	d_0 := false
//	d_i := b_{i-1} or d_{i-1}    for i = 1...sz-1
//	soft (b_i and d_i)
//
// fml, asum, cls, d are the source's locals. The closing hard disjunction of
// the set is asserted by the caller.
func (e *maxResEngine) correctionResolve(correction []ast.ASTHandle, weight *big.Rat) (resultErr error) {
	if len(correction) == 0 {
		return errMaxResState
	}
	m := e.solver.manager
	fml, asum, cls, d := &maxResLocal{m: m}, &maxResLocal{m: m}, &maxResLocal{m: m}, &maxResLocal{m: m}
	defer func() {
		for _, l := range []*maxResLocal{d, cls, asum, fml} {
			resultErr = errors.Join(resultErr, l.release())
		}
	}()
	// m_B.reset(); m_B.append(cs)
	e.lastCore = append(e.lastCore[:0], correction...)
	if _, err := d.set(m.MkFalse()); err != nil {
		return err
	}
	for i := 1; i < len(correction); i++ {
		bi, bi1 := correction[i-1], correction[i]
		current, err := m.RefGet(d.ref)
		if err != nil {
			return err
		}
		clause, err := cls.set(m.MkOr(bi, current))
		if err != nil {
			return err
		}
		if i > 2 {
			fresh, err := e.freshBool("d", d)
			var h ast.ASTHandle
			if err == nil {
				h, err = fml.set(e.implies(fresh, clause))
			}
			if err == nil {
				err = e.updateModel(fresh, clause)
			}
			if err == nil {
				err = e.assertDef(h)
			}
			if err != nil {
				return err
			}
		} else if _, err = d.set(clause, nil); err != nil {
			return err
		}
		fresh, err := e.freshBool("a", asum)
		var h ast.ASTHandle
		if err == nil {
			h, err = fml.set(e.implies(fresh, bi1))
		}
		if err == nil {
			err = e.assertDef(h)
		}
		if err == nil {
			h, err = fml.set(e.implies(fresh, clause))
		}
		if err == nil {
			err = e.assertDef(h)
		}
		if err == nil {
			err = e.addAssumption(fresh, weight)
		}
		if err == nil {
			h, err = fml.set(m.MkAnd(bi1, clause))
		}
		if err == nil {
			err = e.updateModel(fresh, h)
		}
		if err != nil {
			return err
		}
		e.stats.CorrectionResolutions++
	}
	return nil
}
