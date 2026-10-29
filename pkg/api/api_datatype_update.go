package api

// Source: src/api/api_datatype.cpp:617-635.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) DatatypeUpdateField(accessor, value, replacement ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	m, err := c.datatypeManager()
	if err != nil {
		return result, err
	}
	valueSort, err := m.GetSort(value)
	if err != nil {
		return result, err
	}
	replacementSort, err := m.GetSort(replacement)
	if err != nil {
		return result, err
	}
	parameter, err := m.NewParameterAST(accessor)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelParameter(&parameter)) }()
	family, err := m.DatatypeFamilyID()
	if err != nil {
		return result, err
	}
	declaration, err := m.MkFamilyFuncDecl(family, ast.ASTDatatypeUpdateField, []ast.ASTParameter{parameter}, []ast.ASTHandle{valueSort, replacementSort}, ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	return m.MkApp(declaration, []ast.ASTHandle{value, replacement})
}
