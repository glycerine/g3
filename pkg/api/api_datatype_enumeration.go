package api

// Source: src/api/api_datatype.cpp:88-155.

import "github.com/glycerine/g3/pkg/ast"

type APIEnumerationSort struct {
	Sort                   ast.ASTHandle
	Constants, Recognizers []ast.ASTHandle
}

func (c *SMTContext) EnumerationSort(name string, elements []string) (APIEnumerationSort, error) {
	if len(elements) == 0 {
		return APIEnumerationSort{}, ast.ErrASTDomain
	}
	constructors := make([]APIDatatypeConstructor, len(elements))
	for i, element := range elements {
		constructors[i] = APIDatatypeConstructor{Name: element, Recognizer: "is_" + element}
	}
	sort, err := c.DatatypeSort(APIDatatypeDefinition{Name: name, Constructors: constructors})
	if err != nil {
		return APIEnumerationSort{}, err
	}
	result := APIEnumerationSort{Sort: sort}
	for i := range elements {
		constructor, e := c.DatatypeConstructor(sort, uint32(i))
		if e != nil {
			return APIEnumerationSort{}, e
		}
		recognizer, e := c.DatatypeRecognizer(sort, uint32(i))
		if e != nil {
			return APIEnumerationSort{}, e
		}
		result.Constants = append(result.Constants, constructor)
		result.Recognizers = append(result.Recognizers, recognizer)
	}
	return result, nil
}
