package rel

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *TypedUDocRelation) sameShape(other *TypedUDocRelation) bool {
	if r == nil || other == nil || r.open() != nil || other.open() != nil || r.factory != other.factory || len(r.sorts) != len(other.sorts) {
		return false
	}
	for i := range r.sorts {
		if r.sorts[i] != other.sorts[i] {
			return false
		}
	}
	return true
}

func (r *TypedUDocRelation) UnionFrom(source, delta *TypedUDocRelation) error {
	if !r.sameShape(source) || delta != nil && !r.sameShape(delta) {
		return ErrDOC
	}
	var deltaCore *UDocRelation
	if delta != nil {
		deltaCore = delta.relation
	}
	return r.relation.UnionFrom(source.relation, deltaCore)
}

func (r *TypedUDocRelation) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if column >= uint32(len(r.sorts)) {
		return arena.ErrBounds
	}
	sort, err := r.factory.manager.GetSort(value)
	if err != nil || sort != r.sorts[column] {
		return ast.ErrASTDomain
	}
	if number, width, matched, err := r.factory.manager.BitVectorNumeralBigInt(value); err != nil {
		return err
	} else if matched {
		if width != r.relation.widths[column] {
			return ast.ErrASTDomain
		}
		return r.relation.FilterEqualBig(column, number)
	}
	number, matched, err := r.factory.manager.DatalogNumeralValue(value)
	if err != nil || !matched {
		return ast.ErrASTDomain
	}
	return r.relation.FilterEqual(column, number)
}

func (r *TypedUDocRelation) FilterIdentical(columns []uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	if len(columns) == 0 {
		return ErrDOC
	}
	for _, column := range columns {
		if column >= uint32(len(r.sorts)) || r.sorts[column] != r.sorts[columns[0]] {
			return ast.ErrASTDomain
		}
	}
	return r.relation.FilterIdentical(columns)
}

func (r *TypedUDocRelation) FilterInterpreted(condition ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.relation.FilterGuard(r.factory.manager, condition)
}

func (r *TypedUDocRelation) Rename(cycle []uint32) (*TypedUDocRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(cycle) == 0 {
		return r.Clone()
	}
	sorts := append([]ast.ASTHandle(nil), r.sorts...)
	seen := make([]bool, len(sorts))
	for _, column := range cycle {
		if column >= uint32(len(sorts)) || seen[column] {
			return nil, arena.ErrBounds
		}
		seen[column] = true
	}
	for i, source := range cycle {
		destination := cycle[(i+1)%len(cycle)]
		sorts[destination] = r.sorts[source]
	}
	renamed, err := r.relation.Rename(cycle)
	if err != nil {
		return nil, err
	}
	return &TypedUDocRelation{factory: r.factory, sorts: sorts, relation: renamed}, nil
}
