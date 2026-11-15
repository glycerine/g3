package opt

// Source: src/opt/sortmax.cpp sortmax state and construction.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
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
	// scale turns the rational soft weights into the integers terms holds
	// until unscaleTerms divides it out again; scaledOffset is
	// find_mutexes' m_lower times scale.
	scale, scaledOffset *big.Int
}

// maxsmt::operator() builds sortmax for maxsat_engine=sortmax whatever the
// weights or their total; sortmax::operator() rejects a non-unsigned weight
// itself.
func newSortMaxEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (*sortMaxEngine, error) {
	terms, scale, err := maxSMTScaledTerms(s.manager, objective)
	if err != nil {
		return nil, err
	}
	engine := &sortMaxEngine{solver: s, objective: objective, index: index, terms: terms, scale: scale}
	engine.owner.manager = s.manager
	return engine, nil
}

func (e *sortMaxEngine) close() error {
	if e == nil {
		return nil
	}
	e.inputs, e.outputs, e.terms = nil, nil, nil
	return e.owner.close()
}
