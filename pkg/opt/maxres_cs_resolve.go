package opt

// Source: src/opt/maxres.cpp cs_max_resolve.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// correctionResolve is cs_max_resolve for a correction set:
//
//	d_0 := false
//	d_i := b_{i-1} or d_{i-1}    for i = 1...sz-1
//	soft (b_i and d_i)
//
// The closing hard disjunction of the set is asserted by the caller.
func (e *maxResEngine) correctionResolve(correction []ast.ASTHandle, weight *big.Rat) error {
	if len(correction) == 0 {
		return errMaxResState
	}
	m := e.solver.manager
	d, err := m.MkFalse()
	if err != nil {
		return err
	}
	for i := 1; i < len(correction); i++ {
		bi, bi1 := correction[i-1], correction[i]
		cls, err := e.owner.hold(m.MkOr(bi, d))
		if err != nil {
			return err
		}
		if i > 2 {
			d, err = e.freshBool("d")
			if err != nil {
				return err
			}
			fml, err := e.implies(d, cls)
			if err != nil {
				return err
			}
			e.updateModel(d, cls)
			if err = e.assertDef(fml); err != nil {
				return err
			}
		} else {
			d = cls
		}
		asum, err := e.freshBool("a")
		if err != nil {
			return err
		}
		fml, err := e.implies(asum, bi1)
		if err == nil {
			err = e.assertDef(fml)
		}
		if err == nil {
			fml, err = e.implies(asum, cls)
		}
		if err == nil {
			err = e.assertDef(fml)
		}
		if err == nil {
			err = e.addAssumption(asum, weight)
		}
		if err == nil {
			fml, err = e.owner.hold(m.MkAnd(bi1, cls))
		}
		if err != nil {
			return err
		}
		e.updateModel(asum, fml)
		e.stats.CorrectionResolutions++
	}
	return nil
}
