package api

// Source: src/api/api_datatype.cpp:157-218.

import "github.com/glycerine/g3/pkg/ast"

type APIListSort struct {
	Sort, Nil, IsNil, Cons, IsCons, Head, Tail ast.ASTHandle
}

func (c *SMTContext) ListSort(name string, elementSort ast.ASTHandle) (APIListSort, error) {
	sort, err := c.DatatypeSort(APIDatatypeDefinition{
		Name: name,
		Constructors: []APIDatatypeConstructor{
			{Name: "nil", Recognizer: "is_nil"},
			{Name: "cons", Recognizer: "is_cons", Accessors: []APIDatatypeAccessor{
				{Name: "head", Sort: elementSort},
				{Name: "tail", Recursive: true},
			}},
		},
	})
	if err != nil {
		return APIListSort{}, err
	}
	nilDecl, err := c.DatatypeConstructor(sort, 0)
	if err != nil {
		return APIListSort{}, err
	}
	isNil, err := c.DatatypeRecognizer(sort, 0)
	if err != nil {
		return APIListSort{}, err
	}
	cons, err := c.DatatypeConstructor(sort, 1)
	if err != nil {
		return APIListSort{}, err
	}
	isCons, err := c.DatatypeRecognizer(sort, 1)
	if err != nil {
		return APIListSort{}, err
	}
	fields, err := c.datatypeConstructorAccessors(cons)
	if err != nil {
		return APIListSort{}, err
	}
	if len(fields) != 2 {
		return APIListSort{}, ast.ErrASTDomain
	}
	return APIListSort{Sort: sort, Nil: nilDecl, IsNil: isNil, Cons: cons, IsCons: isCons, Head: fields[0], Tail: fields[1]}, nil
}
