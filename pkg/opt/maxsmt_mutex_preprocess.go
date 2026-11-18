package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::find_mutexes.

import (
	"errors"

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
	terms, err := decomposition.terms()
	if err != nil {
		return lbool.Undef, err
	}
	q := e.solver.rationalManager()
	if err = errors.Join(releaseMaxSMTTerms(q, e.terms), q.Copy(&e.mutexOffset, &decomposition.offset)); err != nil {
		return lbool.Undef, errors.Join(err, releaseMaxSMTTerms(q, terms))
	}
	e.terms = terms
	decomposition.transferOwner(&e.owner)
	return lbool.True, nil
}

// unscaleTerms is sortmax::operator()'s expansion check: every new_soft
// weight must be unsigned. It then fixes the integer weights, m_lower and
// the network inputs.
func (e *sortMaxEngine) unscaleTerms() error {
	q := e.solver.rationalManager()
	total := 0
	for i := range e.terms {
		weight, unsigned, err := maxSMTUnsigned(q, &e.terms[i].weight)
		if err != nil {
			return err
		}
		if !unsigned {
			return z3exception.NewDefaultException("sortmax can only handle unsigned weights. Use a different heuristic.")
		}
		e.terms[i].units = weight
		total += int(weight)
	}
	integer, err := q.IsInt64(&e.mutexOffset)
	if err != nil {
		return err
	}
	if !integer {
		return errors.New("opt: SortMax mutex offset overflow")
	}
	offset, err := q.Int64(&e.mutexOffset)
	if err != nil {
		return err
	}
	e.offset = int(offset)
	e.inputs = maxSMTExpandTerms(e.terms, total)
	return nil
}
