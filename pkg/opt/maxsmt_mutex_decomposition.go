package opt

// Source: src/opt/maxsmt.cpp find_mutexes/process_mutex retained shared state.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type maxSMTMutexDecomposition struct {
	owner   maxSMTASTOwner
	q       *numeral.RationalManager
	weights map[ast.ASTHandle]*numeral.Rational // owned
	// soft mirrors the obj_map<expr, rational> new_soft's key layout:
	// sortmax builds its network inputs and initial outputs in its
	// hash-table iteration order.
	soft *ast.ObjMap
	// offset is find_mutexes' m_lower increment (owned).
	offset numeral.Rational
}

func newMaxSMTMutexDecomposition(manager *ast.ASTManager, terms []maxSMTWeightedTerm) (*maxSMTMutexDecomposition, error) {
	q := manager.NumeralService().Manager
	d := &maxSMTMutexDecomposition{q: q, weights: make(map[ast.ASTHandle]*numeral.Rational, len(terms)), soft: ast.NewObjMap(manager), offset: numeral.NewRational()}
	d.owner.manager = manager
	for i := range terms {
		if prior := d.weights[terms[i].expression]; prior != nil {
			if err := q.Add(prior, &terms[i].weight, prior); err != nil {
				return d, err
			}
			continue
		}
		weight := numeral.NewRational()
		if err := q.Copy(&weight, &terms[i].weight); err != nil {
			return d, err
		}
		d.weights[terms[i].expression] = &weight
		if err := d.soft.Insert(terms[i].expression, [2]ast.ASTHandle{}); err != nil {
			return d, err
		}
	}
	return d, nil
}

func (d *maxSMTMutexDecomposition) sign(r *numeral.Rational) (int, error) {
	s, err := d.q.Sign(r)
	return int(s), err
}
