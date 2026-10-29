package api

// Source: src/api/api_datatype.cpp:266-305.

import "github.com/glycerine/g3/pkg/ast"

type APIDatatypeConstructorQuery struct {
	Constructor, Recognizer ast.ASTHandle
	Accessors               []ast.ASTHandle
}

func (o *APIDatatypeConstructorObject) Query() (APIDatatypeConstructorQuery, error) {
	if err := o.open(); err != nil {
		return APIDatatypeConstructorQuery{}, err
	}
	if o.constructor.IsNull() {
		return APIDatatypeConstructorQuery{}, ast.ErrASTDomain
	}
	manager, err := o.owner.command.Manager()
	if err != nil {
		return APIDatatypeConstructorQuery{}, err
	}
	recognizer, err := manager.DatatypeConstructorRecognizer(o.constructor)
	if err != nil {
		return APIDatatypeConstructorQuery{}, err
	}
	accessors, err := manager.DatatypeConstructorAccessors(o.constructor)
	if err != nil {
		return APIDatatypeConstructorQuery{}, err
	}
	if len(accessors) != len(o.definition.Accessors) {
		return APIDatatypeConstructorQuery{}, ast.ErrASTDomain
	}
	return APIDatatypeConstructorQuery{Constructor: o.constructor, Recognizer: recognizer, Accessors: append([]ast.ASTHandle(nil), accessors...)}, nil
}
