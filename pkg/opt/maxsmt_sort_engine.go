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
// weights; sortmax::operator() rejects a non-unsigned weight itself.
// Recorded deviation: a positive weight total above maxSortMaxInputs is
// not expanded into a sorting network here (usable is false) because the
// Go network needs far more memory than z3's for thousands of inputs.
func newSortMaxEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (*sortMaxEngine, bool, error) {
	total := new(big.Rat)
	for _, weight := range objective.softWeights {
		if weight.Sign() > 0 {
			total.Add(total, weight)
		}
	}
	if total.Cmp(big.NewRat(maxSortMaxInputs, 1)) > 0 {
		return nil, false, nil
	}
	terms, scale, err := maxSMTScaledTerms(s.manager, objective)
	if err != nil {
		return nil, false, err
	}
	engine := &sortMaxEngine{solver: s, objective: objective, index: index, terms: terms, scale: scale}
	engine.owner.manager = s.manager
	return engine, true, nil
}

func (e *sortMaxEngine) close() error {
	if e == nil {
		return nil
	}
	e.inputs, e.outputs, e.terms = nil, nil, nil
	return e.owner.close()
}
