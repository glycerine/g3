package api

// Source: src/api/api_datatype.cpp:26-86.

import "github.com/glycerine/g3/pkg/ast"

type APITupleSort struct {
	Sort, Constructor ast.ASTHandle
	Fields            []ast.ASTHandle
}

func (c *SMTContext) TupleSort(name string, fieldNames []string, fieldSorts []ast.ASTHandle) (APITupleSort, error) {
	if len(fieldNames) != len(fieldSorts) {
		return APITupleSort{}, ast.ErrASTDomain
	}
	accessors := make([]APIDatatypeAccessor, len(fieldNames))
	for i := range fieldNames {
		accessors[i] = APIDatatypeAccessor{Name: fieldNames[i], Sort: fieldSorts[i]}
	}
	sort, err := c.DatatypeSort(APIDatatypeDefinition{Name: name, Constructors: []APIDatatypeConstructor{{Name: name, Recognizer: "is_" + name, Accessors: accessors}}})
	if err != nil {
		return APITupleSort{}, err
	}
	constructor, err := c.DatatypeConstructor(sort, 0)
	if err != nil {
		return APITupleSort{}, err
	}
	fields, err := c.datatypeConstructorAccessors(constructor)
	if err != nil {
		return APITupleSort{}, err
	}
	return APITupleSort{Sort: sort, Constructor: constructor, Fields: fields}, nil
}
