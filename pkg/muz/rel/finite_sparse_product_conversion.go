package rel

import (
	"errors"
)

// NewFiniteSparseProductFromTableRelation embeds every table row under an
// empty inner relation, matching the source conversion for empty inner
// signatures.
func NewFiniteSparseProductFromTableRelation(source *TableRelation) (*FiniteSparseProduct, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	result, err := NewFiniteSparseProduct(source.manager, source.sorts, nil)
	if err != nil {
		return nil, err
	}
	facts, err := source.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, fact := range facts {
		key, inner := result.splitEncoded(fact)
		if err = result.addEncodedFact(key, inner); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

// ToTableRelation converts products with an empty inner signature into a
// standalone table relation.
func (p *FiniteSparseProduct) ToTableRelation() (*TableRelation, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.innerCols) != 0 {
		return nil, ErrTableRelation
	}
	result, err := NewTableRelation(p.manager, p.sorts)
	if err != nil {
		return nil, err
	}
	flat, err := p.flatten()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, fact := range facts {
		if _, err = result.AddTableFact(fact); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}
