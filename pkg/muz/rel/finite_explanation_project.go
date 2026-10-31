package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp project_fn and its
// project_reducer, specialized to finite columns and one explanation column.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Project removes finite columns and combines colliding rows using the
// fact-level explanation relation's first-explanation union policy.
func (p *FiniteExplanationProduct) Project(removed []int) (_ *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	flags, err := tableRelationRemovedColumns(len(p.finiteSorts), removed)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, 0, len(p.finiteSorts))
	for i, sort := range p.finiteSorts {
		if !flags[i] {
			sorts = append(sorts, sort)
		}
	}
	result, err := NewFiniteExplanationProduct(p.manager, sorts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		idx := uint64(fact[len(fact)-1])
		if idx >= uint64(len(p.inner)) || p.inner[idx] == nil {
			return nil, ErrTableRelation
		}
		data, e := p.inner[idx].Data()
		if e != nil {
			return nil, e
		}
		if len(data) != 1 {
			return nil, ErrTableRelation
		}
		projected := make(Fact, 0, len(sorts)+1)
		for i := range p.finiteSorts {
			if !flags[i] {
				projected = append(projected, fact[i])
			}
		}
		projected = append(projected, 0)
		found, e := result.table.FetchFact(projected)
		if e != nil {
			return nil, e
		}
		if !found {
			if e = result.addEncoded(projected, data[0]); e != nil {
				return nil, e
			}
		}
	}
	return result, nil
}
