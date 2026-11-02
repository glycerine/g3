package rel

// Source: src/muz/rel/dl_product_relation.cpp transform_fn and
// src/muz/rel/dl_sieve_relation.cpp mk_project_fn. Projecting finite columns
// is an identity on the independent whole-relation explanation sieve.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Project removes finite columns and preserves an independent copy of the
// whole-relation explanation row.
func (p *RelationExplanationProduct) Project(removed []int) (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	flags, err := tableRelationRemovedColumns(len(p.finiteSorts), removed)
	if err != nil {
		return nil, err
	}
	table, err := p.table.Project(removed)
	if err != nil {
		return nil, err
	}
	explanation, err := p.explanation.Clone()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	sorts := make([]ast.ASTHandle, 0, len(p.finiteSorts)-len(removed))
	for column, sort := range p.finiteSorts {
		if !flags[column] {
			sorts = append(sorts, sort)
		}
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: sorts, table: table, explanation: explanation}, nil
}
