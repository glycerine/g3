package api

// Source: src/api/api_datatype.cpp:230-263,308-312.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// APIDatatypeConstructorObject owns the temporary sort references held by a
// native Z3_constructor. It may be reused only while its context is open.
type APIDatatypeConstructorObject struct {
	owner          *smtContextState
	definition     APIDatatypeConstructor
	sortRefs       []ast.ASTRef
	constructorRef ast.ASTRef
	constructor    ast.ASTHandle
	closed         bool
}

func (c *SMTContext) NewDatatypeConstructorObject(definition APIDatatypeConstructor) (result *APIDatatypeConstructorObject, resultErr error) {
	manager, err := c.datatypeManager()
	if err != nil {
		return nil, err
	}
	if definition.Name == "" || definition.Recognizer == "" {
		return nil, ast.ErrASTDomain
	}
	result = &APIDatatypeConstructorObject{owner: c.smtContextState}
	result.definition.Name = definition.Name
	result.definition.Recognizer = definition.Recognizer
	result.definition.Accessors = append(result.definition.Accessors, definition.Accessors...)
	result.sortRefs = make([]ast.ASTRef, len(definition.Accessors))
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	for i, accessor := range result.definition.Accessors {
		if accessor.Name == "" || accessor.Recursive != accessor.Sort.IsNull() {
			return nil, ast.ErrASTDomain
		}
		if accessor.Recursive {
			continue
		}
		kind, kindErr := manager.GetKind(accessor.Sort)
		if kindErr != nil {
			return nil, kindErr
		}
		if kind != ast.ASTSort {
			return nil, ast.ErrASTDomain
		}
		result.sortRefs[i], err = manager.NewRef(accessor.Sort)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (o *APIDatatypeConstructorObject) open() error {
	if o == nil || o.owner == nil || o.closed || o.owner.closed || o.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (o *APIDatatypeConstructorObject) Close() (result error) {
	if o == nil || o.closed {
		return nil
	}
	if o.owner == nil || o.owner.command == nil {
		return ErrSMTContextOwner
	}
	manager, err := o.owner.command.Manager()
	if err != nil {
		return err
	}
	if o.constructorRef != (ast.ASTRef{}) {
		result = errors.Join(result, manager.DelRef(&o.constructorRef))
	}
	for i := range o.sortRefs {
		if o.sortRefs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&o.sortRefs[i]))
		}
	}
	if result == nil {
		o.owner = nil
		o.definition = APIDatatypeConstructor{}
		o.sortRefs = nil
		o.constructor = ast.ASTHandle{}
		o.closed = true
	}
	return result
}
