package api

// Source: src/api/api_datalog.cpp:314-349, 405-440.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (f *APIFixedpoint) ReasonUnknown() (string, error) {
	if err := f.open(); err != nil {
		return "", err
	}
	return f.command.FixedpointReasonUnknown()
}

func (f *APIFixedpoint) Answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.FixedpointAnswer(declaration)
}

func (f *APIFixedpoint) Model(declaration ast.ASTHandle) (*model.Model, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return f.command.FixedpointModel(declaration)
}

func (f *APIFixedpoint) EvaluateModel(declaration, expression ast.ASTHandle, completion bool) (ast.ASTRef, error) {
	if err := f.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return f.command.EvaluateFixedpointModel(declaration, expression, completion)
}
