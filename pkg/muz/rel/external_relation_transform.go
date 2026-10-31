package rel

// Source: src/muz/rel/dl_external_relation.cpp, project and rename functions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Sorts returns a caller-owned copy of the relation signature.
func (r *ExternalRelation) Sorts() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), r.sorts...), nil
}

func (r *ExternalRelation) reduceRelation(app ast.ASTHandle, args, sorts []ast.ASTHandle) (out *ExternalRelation, resultErr error) {
	appRef, err := r.ast.NewRef(app)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, r.ast.DelRef(&appRef))
		if resultErr != nil && out != nil {
			resultErr = errors.Join(resultErr, out.Close())
			out = nil
		}
	}()
	decl, err := r.ast.GetDecl(app)
	if err != nil {
		return nil, err
	}
	value, err := r.reduce(decl, args)
	if err != nil {
		return nil, err
	}
	ref, err := r.ast.NewRef(value)
	if err != nil {
		return nil, err
	}
	return &ExternalRelation{manager: r.manager, ast: r.ast, sorts: append([]ast.ASTHandle(nil), sorts...), callbacks: r.callbacks, relation: ref}, nil
}

// Project removes sorted, distinct columns and invokes the project reduction.
func (r *ExternalRelation) Project(removed []uint32) (*ExternalRelation, error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, ast.ErrASTDomain
	}
	sorts := make([]ast.ASTHandle, 0, len(r.sorts))
	next := 0
	for i, sort := range r.sorts {
		if next < len(removed) && removed[next] == uint32(i) {
			next++
			continue
		}
		sorts = append(sorts, sort)
	}
	if next != len(removed) {
		return nil, ast.ErrASTDomain
	}
	app, err := r.ast.MkDatalogProject(expression, removed)
	if err != nil {
		return nil, err
	}
	return r.reduceRelation(app, []ast.ASTHandle{expression}, sorts)
}

// Rename rotates the source signature and invokes the rename reduction.
func (r *ExternalRelation) Rename(cycle []uint32) (*ExternalRelation, error) {
	expression, err := r.Relation()
	if err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return nil, ast.ErrASTDomain
	}
	seen := make([]bool, len(r.sorts))
	for _, column := range cycle {
		if uint64(column) >= uint64(len(seen)) || seen[column] {
			return nil, ast.ErrASTDomain
		}
		seen[column] = true
	}
	sorts := append([]ast.ASTHandle(nil), r.sorts...)
	for i, column := range cycle {
		sorts[column] = r.sorts[cycle[(i+1)%len(cycle)]]
	}
	app, err := r.ast.MkDatalogRename(expression, cycle)
	if err != nil {
		return nil, err
	}
	return r.reduceRelation(app, []ast.ASTHandle{expression}, sorts)
}
