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
	return f.command.QueryFixedpointWithParams(declaration, f.queryParameters())
}

// QueryExpression evaluates a Boolean fixedpoint query.
func (f *APIFixedpoint) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpointExpressionWithParams(query, f.queryParameters())
}

func (f *APIFixedpoint) QueryRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpointRelationsWithParams(declarations, f.queryParameters())
}
