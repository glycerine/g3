package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::find_mutexes.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func (e *sortMaxEngine) preprocessMutexes() (status lbool.LBool, resultErr error) {
	expressions := make([]ast.ASTHandle, len(e.terms))
	for i := range e.terms {
		expressions[i] = e.terms[i].expression
	}
	status, mutexes, err := e.solver.solver().FindMutexes(expressions)
	if err != nil || status != lbool.True {
		return status, err
	}
	decomposition, err := newMaxSMTMutexDecomposition(e.solver.manager, e.terms)
	defer func() { resultErr = errors.Join(resultErr, decomposition.close()) }()
	if err != nil {
		return lbool.Undef, err
	}
	for _, mutex := range mutexes {
		if err = decomposition.transform(mutex); err != nil {
			return lbool.Undef, err
		}
		e.stats.Mutexes++
		e.stats.MutexTerms += uint32(len(mutex))
	}
	e.terms = decomposition.terms()
	e.scaledOffset = decomposition.lowerOffset()
	decomposition.transferOwner(&e.owner)
	return lbool.True, nil
}

// unscaleTerms is sortmax::operator()'s expansion check: every new_soft
// weight must be unsigned. It then fixes the integer weights, m_lower and
// the network inputs.
func (e *sortMaxEngine) unscaleTerms() error {
	total := 0
	for i := range e.terms {
		weight, unsigned := maxSMTUnscaleUnsigned(&e.terms[i].weight, e.scale)
		if !unsigned {
			return z3exception.NewDefaultException("sortmax can only handle unsigned weights. Use a different heuristic.")
		}
		e.terms[i].weight.SetUint64(uint64(weight))
		total += int(weight)
	}
	var offset, remainder big.Int
	offset.QuoRem(e.scaledOffset, e.scale, &remainder)
	if remainder.Sign() != 0 || !offset.IsInt64() {
		return errors.New("opt: SortMax mutex offset overflow")
	}
	e.offset = int(offset.Int64())
	e.inputs = maxSMTExpandTerms(e.terms, total)
	return nil
}
