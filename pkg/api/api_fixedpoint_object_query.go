package api

// Source: src/api/api_datalog.cpp:277-327.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (f *APIFixedpoint) queryParameters() *uparams.ParamsRef {
	if f.params == nil {
		return nil
	}
	return &f.params.value
}

func (f *APIFixedpoint) Query(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	status, err := f.command.QueryFixedpointWithParams(declaration, f.queryParameters())
	if err == nil {
		f.lastQuery, f.hasLastQuery = declaration, true
	}
	return status, err
}

// QueryExpression evaluates a Boolean fixedpoint query.
func (f *APIFixedpoint) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	status, err := f.command.QueryFixedpointExpressionWithParams(query, f.queryParameters())
	if err != nil {
		return status, err
	}
	manager, err := f.command.Manager()
	if err != nil {
		return status, err
	}
	kind, err := manager.GetKind(query)
	if err != nil {
		return status, err
	}
	if kind != ast.ASTApp {
		return status, nil
	}
	arity, err := manager.GetNumArgs(query)
	if err != nil || arity != 0 {
		return status, err
	}
	decl, err := manager.GetDecl(query)
	if err != nil {
		return status, err
	}
	predicates, err := f.command.FixedpointPredicates()
	if err != nil {
		return status, err
	}
	for _, predicate := range predicates {
		if predicate == decl {
			f.lastQuery, f.hasLastQuery = decl, true
			break
		}
	}
	return status, nil
}

func (f *APIFixedpoint) QueryRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	status, err := f.command.QueryFixedpointRelationsWithParams(declarations, f.queryParameters())
	if err == nil && len(declarations) == 1 {
		f.lastQuery, f.hasLastQuery = declarations[0], true
	}
	return status, err
}
