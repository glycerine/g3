package api

// Source: src/api/api_datatype.cpp:446-554.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) DatatypeIsRecursive(sort ast.ASTHandle) (bool, error) {
	m, err := c.datatypeManager()
	if err != nil {
		return false, err
	}
	return m.DatatypeIsRecursive(sort)
}

func (c *SMTContext) DatatypeConstructorCount(sort ast.ASTHandle) (uint32, error) {
	m, err := c.datatypeManager()
	if err != nil {
		return 0, err
	}
	return m.DatatypeNumConstructors(sort)
}

func (c *SMTContext) DatatypeConstructor(sort ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	m, err := c.datatypeManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	constructors, err := m.DatatypeConstructors(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(index) >= uint64(len(constructors)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return constructors[index], nil
}

func (c *SMTContext) DatatypeRecognizer(sort ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	constructor, err := c.DatatypeConstructor(sort, index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m, err := c.datatypeManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.DatatypeConstructorRecognizer(constructor)
}

func (c *SMTContext) datatypeConstructorAccessors(constructor ast.ASTHandle) ([]ast.ASTHandle, error) {
	m, err := c.datatypeManager()
	if err != nil {
		return nil, err
	}
	return m.DatatypeConstructorAccessors(constructor)
}

func (c *SMTContext) DatatypeAccessor(sort ast.ASTHandle, constructorIndex, accessorIndex uint32) (ast.ASTHandle, error) {
	constructor, err := c.DatatypeConstructor(sort, constructorIndex)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	accessors, err := c.datatypeConstructorAccessors(constructor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(accessorIndex) >= uint64(len(accessors)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return accessors[accessorIndex], nil
}
