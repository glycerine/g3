package api

// Source: src/api/api_datalog.cpp:280-311, 500-525.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *APIFixedpoint) Push() error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.PushFixedpoint()
}

func (f *APIFixedpoint) Pop() error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.PopFixedpoint()
}

func (f *APIFixedpoint) Query(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpoint(declaration)
}

func (f *APIFixedpoint) QueryRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	return f.command.QueryFixedpointRelations(declarations)
}
