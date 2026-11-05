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
		switch source.(type) {
		case *FormulaCheckedTable, *FormulaCheckedFinitePair:
			return emptyFiniteLike(source)
		}
		if manager.checkedFinite == nil {
			return emptyFiniteLike(source)
		}
		// Checked favourites participate only when both concrete plugins handle
		// the signature. Sparse and lazy_sparse reject the nullary signature, so
		// preserve the source-selected ordinary Hash fallback for that product.
		if !manager.checkedFinite.CanHandleSignature(signature) {
			return emptyFiniteLike(source)
		}
		if _, checked := source.(*FiniteCheckedPair); checked {
			return emptyFiniteLike(source)
		}
		// Source transfer preserves the actual selected primary (including the
		// context-owned lazy_sparse engine) while constructing an empty backing;
		// the caller copies the snapshot exactly once into both checked sides.
		primary, err := source.CloneFinite()
		if err != nil {
			return nil, err
		}
		if err = primary.Reset(); err != nil {
			return nil, errors.Join(err, primary.Close())
		}
		checker, err := manager.checkedFinite.table(manager.checkedFinite.checkerKind, signature, 0)
		if err != nil {
			return nil, errors.Join(err, primary.Close())
		}
		return adoptFiniteCheckedPair(primary, checker)
	}
	return manager.NewDefaultFiniteTable(signature)
}

// newRelationExplanationFiniteBackingSelected builds the ordinary relation
// plugin selected as the check_relation favourite. Sparse and LazySparse
// reject nullary signatures in C++, so the source relation manager falls back
// to Hash for that shape.
func newRelationExplanationFiniteBackingSelected(manager *RelationManager, signature Signature, kind string) (FiniteTable, error) {
	switch kind {
	case "":
		return manager.NewDefaultFiniteTable(signature)
	case "hashtable":
		return NewHashTable(signature), nil
	case "sparse":
		if signature.Len() == 0 {
			return NewHashTable(signature), nil
		}
		return NewSparseTable(signature, 0)
	case "lazy_sparse":
		if signature.Len() == 0 {
			return NewHashTable(signature), nil
		}
		if manager.lazyFinite == nil || !manager.lazyFinite.CanHandleSignature(signature) {
			return nil, ErrTable
		}
		return manager.lazyFinite.New(signature)
	default:
		return nil, ErrTable
	}
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

func newRelationExplanationProductWithSelectedBacking(manager *RelationManager, sorts []ast.ASTHandle, kind string) (_ *RelationExplanationProduct, resultErr error) {
	product, err := NewRelationExplanationProduct(manager, sorts)
	if err != nil || kind == "" {
		return product, err
	}
	signature, err := product.table.Signature()
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	backing, err := newRelationExplanationFiniteBackingSelected(manager, signature, kind)
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
	switch table := p.table.(type) {
	case *FiniteCheckedPair:
		primary, checker, err = table.ConcreteKinds()
		return primary, checker, true, err
	case *FormulaCheckedTable:
		return "hashtable", "formula", true, nil
	case *FormulaCheckedFinitePair:
		primary, checker, _, err = table.ConcreteKinds()
		return primary, checker, true, err
	case *HashTable:
		return "hashtable", "", false, nil
	case *SparseTable:
		return "sparse", "", false, nil
	case *LazySparseTable:
		return "lazy_sparse", "", false, nil
	default:
		return "", "", false, ErrCompilerUnsupported
	}
}

// FormulaChecked reports whether the finite sieve is guarded by the concrete
// source check_relation wrapper. The whole proof relation is never part of
// this checker.
func (p *RelationExplanationProduct) FormulaChecked() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	switch p.table.(type) {
	case *FormulaCheckedTable, *FormulaCheckedFinitePair:
		return true, nil
	default:
		return false, nil
	}
}
