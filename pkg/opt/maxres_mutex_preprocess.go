package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::find_mutexes and
// src/opt/maxres.cpp init_local.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// preprocessMutexes is maxsmt_solver_base::find_mutexes. A status other than
// l_true from s().find_mutexes is returned as is (init_local / wmax return
// it from operator()).
// The caller owns the returned terms' weights (releaseWeightedTerms).
func (e *maxResEngine) preprocessMutexes(objective *arithmeticObjective) (_ []maxResWeightedTerm, _ lbool.LBool, err error) {
	expressions := make([]ast.ASTHandle, len(objective.softTerms))
	newSoft := newSoftWeightMap(e.solver.manager)
	q := e.q()
	weight := numeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&weight), newSoft.release()) }()
	for i := range objective.softTerms {
		expression, err := e.solver.manager.RefGet(objective.softTerms[i])
		if err != nil {
			return nil, lbool.Undef, err
		}
		expressions[i] = expression
		// maxsmt::add already merged duplicate soft constraints.
		if err = q.Copy(&weight, &objective.softWeights[i]); err != nil {
			return nil, lbool.Undef, err
		}
		old, err := newSoft.find(expression)
		if err != nil {
			return nil, lbool.Undef, err
		}
		if old != nil {
			if err = q.Add(&weight, old, &weight); err != nil {
				return nil, lbool.Undef, err
			}
		}
		if err = newSoft.insert(expression, &weight); err != nil {
			return nil, lbool.Undef, err
		}
	}
	status, mutexes, err := e.solver.solver().FindMutexes(expressions)
	if err != nil {
		return nil, lbool.Undef, fmt.Errorf("maxres mutex discovery: %w", err)
	}
	if status != lbool.True {
		return nil, status, nil
	}
	for _, mutex := range mutexes {
		if err = e.transformMutex(mutex, newSoft); err != nil {
			return nil, lbool.Undef, fmt.Errorf("maxres mutex transform: %w", err)
		}
	}
	entries, err := newSoft.entries()
	if err != nil {
		return nil, lbool.Undef, err
	}
	var terms []maxResWeightedTerm
	for _, term := range entries {
		if e.sign(&term.weight) > 0 {
			terms = append(terms, term)
		} else {
			err = errors.Join(err, q.Del(&term.weight))
		}
	}
	if err = errors.Join(err, e.ratErr); err != nil {
		return nil, lbool.Undef, errors.Join(err, releaseWeightedTerms(q, terms))
	}
	return terms, lbool.True, nil
}
