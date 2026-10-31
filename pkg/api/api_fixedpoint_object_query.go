package api

// Source: src/api/api_datalog.cpp:277-327.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *APIFixedpoint) Query(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpoint(declaration)
}

// QueryExpression evaluates a Boolean fixedpoint query.
func (f *APIFixedpoint) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpointExpression(query)
}

func (f *APIFixedpoint) QueryRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpointRelations(declarations)
}
