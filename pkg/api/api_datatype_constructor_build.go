package api

// Source: src/api/api_datatype.cpp:314-370,397-437.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) DatatypeSortFromConstructors(name string, constructors ...*APIDatatypeConstructorObject) (ast.ASTHandle, error) {
	list, err := c.NewDatatypeConstructorList(constructors...)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer list.Close()
	sorts, err := c.DatatypeSortsFromConstructorLists([]string{name}, []*APIDatatypeConstructorList{list})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(sorts) != 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return sorts[0], nil
}

func (c *SMTContext) DatatypeSortsFromConstructorLists(names []string, lists []*APIDatatypeConstructorList) ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if len(names) == 0 || len(names) != len(lists) {
		return nil, ast.ErrASTDomain
	}
	definitions := make([]APIDatatypeDefinition, len(names))
	for i, list := range lists {
		if list == nil || list.owner != c.smtContextState {
			return nil, ErrSMTContextOwner
		}
		if err := list.open(); err != nil {
			return nil, err
		}
		if names[i] == "" || len(list.constructors) == 0 {
			return nil, ast.ErrASTDomain
		}
		definitions[i].Name = names[i]
		for _, constructor := range list.constructors {
			definition, err := constructor.apiDefinition(c.smtContextState, uint32(len(names)))
			if err != nil {
				return nil, err
			}
			definitions[i].Constructors = append(definitions[i].Constructors, definition)
		}
	}
	sorts, err := c.DatatypeSorts(definitions)
	if err != nil {
		return nil, err
	}
	for i, sort := range sorts {
		for j, constructor := range lists[i].constructors {
			declaration, queryErr := c.DatatypeConstructor(sort, uint32(j))
			if queryErr != nil {
				return sorts, queryErr
			}
			if queryErr = constructor.setConstructor(declaration); queryErr != nil {
				return sorts, queryErr
			}
		}
	}
	return sorts, nil
}
