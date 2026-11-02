package transforms_test

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func sparseTransformRelation(manager *rel.RelationManager, predicate ast.ASTHandle, sparse bool) (rel.FiniteTable, error) {
	if !sparse {
		return manager.GetRelation(predicate)
	}
	_, signature, err := manager.PredicateSignature(predicate)
	if err != nil {
		return nil, err
	}
	table, err := rel.NewSparseTable(signature, 0)
	if err != nil {
		return nil, err
	}
	if err = manager.StoreFiniteRelation(predicate, table); err != nil {
		return nil, errors.Join(err, table.Close())
	}
	return table, nil
}
