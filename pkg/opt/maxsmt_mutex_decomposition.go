package opt

// Source: src/opt/maxsmt.cpp find_mutexes/process_mutex retained shared state.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type maxSMTMutexDecomposition struct {
	owner   maxSMTASTOwner
	weights map[ast.ASTHandle]*big.Int
	// soft mirrors the obj_map<expr, rational> new_soft's key layout:
	// sortmax builds its network inputs and initial outputs in its
	// hash-table iteration order.
	soft   *ast.ObjMap
	offset big.Int
}

func newMaxSMTMutexDecomposition(manager *ast.ASTManager, terms []maxSMTWeightedTerm) (*maxSMTMutexDecomposition, error) {
	d := &maxSMTMutexDecomposition{weights: make(map[ast.ASTHandle]*big.Int, len(terms)), soft: ast.NewObjMap(manager)}
	d.owner.manager = manager
	for i := range terms {
		if prior := d.weights[terms[i].expression]; prior != nil {
			prior.Add(prior, &terms[i].weight)
			continue
		}
		d.weights[terms[i].expression] = new(big.Int).Set(&terms[i].weight)
		if err := d.soft.Insert(terms[i].expression, [2]ast.ASTHandle{}); err != nil {
			return d, err
		}
	}
	return d, nil
}
