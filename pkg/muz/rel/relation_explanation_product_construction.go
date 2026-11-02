package rel

// Source: src/muz/rel/dl_product_relation.cpp mk_full, converting joins and
// transform_fn, specialized to the finite and whole-explanation sieves used
// by relation-level explanations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// NewRelationExplanationTotal constructs the join identity with one nullary
// finite tuple and one undefined whole-explanation row. A zero-column proof
// row is valid and remains nonempty.
func NewRelationExplanationTotal(manager *RelationManager, proofColumns int) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil || proofColumns < 0 {
		return nil, ErrTableRelation
	}
	signature, err := NewSignature(nil)
	if err != nil {
		return nil, err
	}
	table := NewHashTable(signature)
	if _, err = table.Add(Fact{}); err != nil {
		return nil, errors.Join(err, table.Close())
	}
	ruleSort, err := manager.manager.MkDatalogRuleSort()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	proofSorts := make([]ast.ASTHandle, proofColumns)
	for index := range proofSorts {
		proofSorts[index] = ruleSort
	}
	explanation, err := NewExplanationRelation(manager.manager, proofSorts)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	if err = explanation.AddFact(make([]ast.ASTHandle, proofColumns)); err != nil {
		return nil, errors.Join(err, explanation.Close(), table.Close())
	}
	return &RelationExplanationProduct{manager: manager, table: table, explanation: explanation}, nil
}

// RelationExplanationFromFinite copies the actual finite rows into the
// concrete Hash sieve and pairs them with the full zero-proof identity.
func RelationExplanationFromFinite(manager *RelationManager, finiteSorts []ast.ASTHandle, source FiniteTable) (_ *RelationExplanationProduct, resultErr error) {
	if manager == nil || manager.open() != nil || source == nil {
		return nil, ErrTableRelation
	}
	expected, err := tableRelationSignature(manager, finiteSorts)
	if err != nil {
		return nil, err
	}
	actual, err := source.Signature()
	if err != nil || !expected.equal(actual) {
		return nil, errors.Join(err, ErrTableRelation)
	}
	table := NewHashTable(expected)
	rows, err := source.Facts()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	for _, row := range rows {
		if _, err = table.Add(row); err != nil {
			return nil, errors.Join(err, table.Close())
		}
	}
	explanation, err := NewExplanationRelation(manager.manager, nil)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	if err = explanation.AddFact(nil); err != nil {
		return nil, errors.Join(err, explanation.Close(), table.Close())
	}
	return &RelationExplanationProduct{manager: manager, finiteSorts: append([]ast.ASTHandle(nil), finiteSorts...), table: table, explanation: explanation}, nil
}

// RenameFinite applies a finite-column cycle while cloning the independent
// whole-explanation sieve.
func (p *RelationExplanationProduct) RenameFinite(cycle []int) (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	permutation, err := tableRelationCyclePermutation(len(p.finiteSorts), cycle)
	if err != nil {
		return nil, err
	}
	table, err := p.table.RenameCycle(cycle)
	if err != nil {
		return nil, err
	}
	explanation, err := p.explanation.Clone()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	sorts := make([]ast.ASTHandle, len(p.finiteSorts))
	for column, source := range permutation {
		sorts[column] = p.finiteSorts[source]
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: sorts, table: table, explanation: explanation}, nil
}

// Len reports logical rows. A product with an empty finite or explanation
// component is empty, matching product_relation::empty.
func (p *RelationExplanationProduct) Len() (int, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	empty, err := p.explanation.Empty()
	if err != nil || empty {
		return 0, err
	}
	return p.table.Len()
}

func (p *RelationExplanationProduct) Empty() (bool, error) {
	rows, err := p.Len()
	return rows == 0, err
}
