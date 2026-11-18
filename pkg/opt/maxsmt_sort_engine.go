package opt

// Source: src/opt/sortmax.cpp sortmax state and construction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type sortMaxEngine struct {
	solver    *ArithmeticSolver
	objective *arithmeticObjective
	index     int
	owner     maxSMTASTOwner
	terms     []maxSMTWeightedTerm
	inputs    []ast.ASTHandle
	outputs   []ast.ASTHandle
	offset    int
	stats     sortMaxStatistics
	// mutexOffset is find_mutexes' increment of m_lower (owned).
	mutexOffset numeral.Rational
}

// maxsmt::operator() builds sortmax for maxsat_engine=sortmax whatever the
// weights or their total; sortmax::operator() rejects a non-unsigned weight
// itself.
func newSortMaxEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (*sortMaxEngine, error) {
	terms, err := maxSMTRationalTerms(s.manager, objective)
	if err != nil {
		return nil, err
	}
	engine := &sortMaxEngine{solver: s, objective: objective, index: index, terms: terms, mutexOffset: numeral.NewRational()}
	engine.owner.manager = s.manager
	return engine, nil
}

func (e *sortMaxEngine) close() error {
	if e == nil {
		return nil
	}
	q := e.solver.rationalManager()
	err := errors.Join(releaseMaxSMTTerms(q, e.terms), q.Del(&e.mutexOffset))
	e.inputs, e.outputs, e.terms = nil, nil, nil
	return errors.Join(err, e.owner.close())
}
