package rel

// Source: src/muz/rel/dl_product_relation.cpp mk_full, converting joins and
// transform_fn, specialized to the finite and whole-explanation sieves used
// by relation-level explanations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func newRelationExplanationFiniteBacking(manager *RelationManager, signature Signature, source FiniteTable) (FiniteTable, error) {
	if manager == nil || manager.open() != nil {
		return nil, ErrTableRelation
	}
	if source != nil {
		if manager.checkedFinite == nil {
			return emptyFiniteLike(source)
		}
		if _, checked := source.(*FiniteCheckedPair); checked {
			return emptyFiniteLike(source)
		}
		primary := ""
		switch source.(type) {
		case *HashTable:
			primary = "hashtable"
		case *SparseTable:
			primary = "sparse"
		}
		if primary != "" {
			factory, err := NewFiniteCheckedFactory(primary, manager.checkedFinite.checkerKind)
			if err != nil {
				return nil, err
			}
			return factory.New(signature)
		}
		return nil, ErrCompilerUnsupported
	}
	return manager.NewDefaultFiniteTable(signature)
}

func newRelationExplanationProductWithBacking(manager *RelationManager, sorts []ast.ASTHandle, source FiniteTable) (_ *RelationExplanationProduct, resultErr error) {
	product, err := NewRelationExplanationProduct(manager, sorts)
	if err != nil || source == nil {
		return product, err
	}
	signature, err := product.table.Signature()
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	backing, err := newRelationExplanationFiniteBacking(manager, signature, source)
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	if err = product.table.Close(); err != nil {
		return nil, errors.Join(err, backing.Close(), product.Close())
	}
	product.table = backing
	return product, nil
}

// EmptyLike preserves the selected finite representation for union targets
// and deltas while constructing an empty proof component of identical width.
func (p *RelationExplanationProduct) EmptyLike() (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	table, err := emptyFiniteLike(p.table)
	if err != nil {
		return nil, err
	}
	proofSorts, err := p.explanation.signature()
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	explanation, err := NewExplanationRelation(p.manager.manager, proofSorts)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: append([]ast.ASTHandle(nil), p.finiteSorts...), table: table, explanation: explanation}, nil
}

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
	table, err := manager.NewDefaultFiniteTable(signature)
	if err != nil {
		return nil, err
	}
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
	table, err := newRelationExplanationFiniteBacking(manager, expected, source)
	if err != nil {
		return nil, err
	}
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
	table, err := finiteTableRename(p.table, cycle)
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

// CheckedConcreteKinds reports the concrete primary/checker pair used by the
// finite component. Ordinary products return checked=false.
func (p *RelationExplanationProduct) CheckedConcreteKinds() (primary, checker string, checked bool, err error) {
	if err = p.open(); err != nil {
		return "", "", false, err
	}
	pair, ok := p.table.(*FiniteCheckedPair)
	if !ok {
		return "", "", false, nil
	}
	primary, checker, err = pair.ConcreteKinds()
	return primary, checker, true, err
}
