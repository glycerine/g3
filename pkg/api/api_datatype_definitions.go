package api

// Source: src/api/api_datatype.cpp:220-444.

import (
	"github.com/glycerine/g3/pkg/ast"
)

type APIDatatypeAccessor struct {
	Name           string
	Sort           ast.ASTHandle
	Recursive      bool
	RecursiveIndex uint32
}

type APIDatatypeConstructor struct {
	Name, Recognizer string
	Accessors        []APIDatatypeAccessor
}

type APIDatatypeDefinition struct {
	Name         string
	Parameters   []ast.ASTHandle
	Constructors []APIDatatypeConstructor
}

func (c *SMTContext) datatypeManager() (*ast.ASTManager, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, err
	}
	if _, err = m.RegisterDatatypePlugin(); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *SMTContext) DatatypeSorts(definitions []APIDatatypeDefinition) ([]ast.ASTHandle, error) {
	m, err := c.datatypeManager()
	if err != nil {
		return nil, err
	}
	converted := make([]ast.DatatypeDefinition, len(definitions))
	for i := range definitions {
		converted[i].Name, err = m.Symbols.Intern(definitions[i].Name)
		if err != nil {
			return nil, err
		}
		converted[i].Parameters = append(converted[i].Parameters, definitions[i].Parameters...)
		for _, sourceConstructor := range definitions[i].Constructors {
			constructor := ast.DatatypeConstructorDefinition{}
			constructor.Name, err = m.Symbols.Intern(sourceConstructor.Name)
			if err != nil {
				return nil, err
			}
			constructor.Recognizer, err = m.Symbols.Intern(sourceConstructor.Recognizer)
			if err != nil {
				return nil, err
			}
			for _, sourceAccessor := range sourceConstructor.Accessors {
				accessor := ast.DatatypeAccessorDefinition{Range: sourceAccessor.Sort, Recursive: sourceAccessor.Recursive, RecursiveIndex: sourceAccessor.RecursiveIndex}
				accessor.Name, err = m.Symbols.Intern(sourceAccessor.Name)
				if err != nil {
					return nil, err
				}
				constructor.Accessors = append(constructor.Accessors, accessor)
			}
			converted[i].Constructors = append(converted[i].Constructors, constructor)
		}
	}
	return m.MkDatatypes(converted)
}

func (c *SMTContext) DatatypeSort(definition APIDatatypeDefinition) (ast.ASTHandle, error) {
	sorts, err := c.DatatypeSorts([]APIDatatypeDefinition{definition})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(sorts) != 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return sorts[0], nil
}
