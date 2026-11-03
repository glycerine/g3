package rel

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *TypedUDocRelation) fact(values []ast.ASTHandle) ([]*big.Int, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(values) != len(r.sorts) {
		return nil, arena.ErrBounds
	}
	result := make([]*big.Int, len(values))
	for i, value := range values {
		sort, err := r.factory.manager.GetSort(value)
		if err != nil || sort != r.sorts[i] {
			return nil, errors.Join(ast.ErrASTDomain, err)
		}
		if number, width, matched, err := r.factory.manager.BitVectorNumeralBigInt(value); err != nil {
			return nil, err
		} else if matched {
			if width != r.relation.widths[i] {
				return nil, ast.ErrASTDomain
			}
			result[i] = number
			continue
		}
		number, matched, err := r.factory.manager.DatalogNumeralValue(value)
		if err != nil || !matched {
			return nil, errors.Join(ast.ErrASTDomain, err)
		}
		result[i] = new(big.Int).SetUint64(number)
	}
	return result, nil
}

func (r *TypedUDocRelation) AddFact(values []ast.ASTHandle) (bool, error) {
	fact, err := r.fact(values)
	if err != nil {
		return false, err
	}
	return r.relation.AddBigFact(fact)
}

func (r *TypedUDocRelation) ContainsFact(values []ast.ASTHandle) (bool, error) {
	fact, err := r.fact(values)
	if err != nil {
		return false, err
	}
	document, err := r.relation.factToDoc(fact)
	if err != nil {
		return false, err
	}
	defer r.relation.manager.Deallocate(document)
	return r.relation.elems.Contains(document)
}

func (r *TypedUDocRelation) FastEmpty() bool {
	return r.open() != nil || r.relation.FastEmpty()
}

func (r *TypedUDocRelation) Empty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.relation.Empty(r.factory.manager)
}

func (r *TypedUDocRelation) Clone() (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	copy, err := r.relation.Clone()
	if err != nil {
		return nil, err
	}
	return &TypedUDocRelation{factory: r.factory, sorts: append([]ast.ASTHandle(nil), r.sorts...), relation: copy}, nil
}

func (r *TypedUDocRelation) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.relation.Reset()
}

func (r *TypedUDocRelation) Project(removed []uint32) (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	seen := make([]bool, len(r.sorts))
	previous := ^uint32(0)
	for i, column := range removed {
		if column >= uint32(len(r.sorts)) || seen[column] || (i != 0 && column <= previous) {
			return nil, arena.ErrBounds
		}
		seen[column], previous = true, column
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts)-len(removed))
	widths := make([]uint32, 0, len(sorts))
	for i, sort := range r.sorts {
		if !seen[i] {
			sorts = append(sorts, sort)
			widths = append(widths, r.relation.widths[i])
		}
	}
	destination, err := r.factory.managerFor(widths)
	if err != nil {
		return nil, err
	}
	projected, err := r.relation.Project(removed, destination)
	if err != nil {
		return nil, err
	}
	return &TypedUDocRelation{factory: r.factory, sorts: sorts, relation: projected}, nil
}

func (r *TypedUDocRelation) Join(other *TypedUDocRelation, leftColumns, rightColumns []uint32) (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil || other.factory != r.factory {
		return nil, ErrDOC
	}
	if len(leftColumns) != len(rightColumns) {
		return nil, ErrDOC
	}
	for i := range leftColumns {
		if leftColumns[i] >= uint32(len(r.sorts)) || rightColumns[i] >= uint32(len(other.sorts)) || r.sorts[leftColumns[i]] != other.sorts[rightColumns[i]] {
			return nil, ast.ErrASTDomain
		}
	}
	sorts := append(append([]ast.ASTHandle(nil), r.sorts...), other.sorts...)
	widths := append(append([]uint32(nil), r.relation.widths...), other.relation.widths...)
	destination, err := r.factory.managerFor(widths)
	if err != nil {
		return nil, err
	}
	joined, err := r.relation.Join(other.relation, leftColumns, rightColumns, destination)
	if err != nil {
		return nil, err
	}
	return &TypedUDocRelation{factory: r.factory, sorts: sorts, relation: joined}, nil
}

func (r *TypedUDocRelation) JoinProject(other *TypedUDocRelation, leftColumns, rightColumns, removed []uint32) (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil || other.factory != r.factory {
		return nil, ErrDOC
	}
	if len(leftColumns) != len(rightColumns) {
		return nil, ErrDOC
	}
	for i := range leftColumns {
		if leftColumns[i] >= uint32(len(r.sorts)) || rightColumns[i] >= uint32(len(other.sorts)) || r.sorts[leftColumns[i]] != other.sorts[rightColumns[i]] {
			return nil, ast.ErrASTDomain
		}
	}
	productWidths := append(append([]uint32(nil), r.relation.widths...), other.relation.widths...)
	productManager, err := r.factory.managerFor(productWidths)
	if err != nil {
		return nil, err
	}
	allSorts := append(append([]ast.ASTHandle(nil), r.sorts...), other.sorts...)
	removedSet := make([]bool, len(allSorts))
	previous := ^uint32(0)
	for i, column := range removed {
		if column >= uint32(len(allSorts)) || removedSet[column] || (i != 0 && column <= previous) {
			return nil, arena.ErrBounds
		}
		removedSet[column] = true
		previous = column
	}
	resultSorts := make([]ast.ASTHandle, 0, len(allSorts)-len(removed))
	resultWidths := make([]uint32, 0, len(productWidths)-len(removed))
	for i, sort := range allSorts {
		if !removedSet[i] {
			resultSorts = append(resultSorts, sort)
			resultWidths = append(resultWidths, productWidths[i])
		}
	}
	resultManager, err := r.factory.managerFor(resultWidths)
	if err != nil {
		return nil, err
	}
	joined, err := r.relation.JoinProject(other.relation, leftColumns, rightColumns, removed, productManager, resultManager)
	if err != nil {
		return nil, err
	}
	return &TypedUDocRelation{factory: r.factory, sorts: resultSorts, relation: joined}, nil
}
