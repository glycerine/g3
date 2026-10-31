package api

// Source: src/api/api_datalog.cpp fixedpoint_from_stream query result.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// LoadSMT2WithQueries loads a fixedpoint program and returns an owning vector
// of its query expressions. The vector must be closed before its context.
func (f *APIFixedpoint) LoadSMT2WithQueries(input io.Reader) (vector *ASTVector, resultErr error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, ErrSMTContextOwner
	}
	before, err := f.command.Assertions()
	if err != nil {
		return nil, err
	}
	refs, err := smt2.ParseAssertionsWithFixedpointQueries(f.command, input)
	if err != nil {
		return nil, err
	}
	manager, err := f.command.Manager()
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range refs {
			resultErr = errors.Join(resultErr, manager.DelRef(&refs[i]))
		}
		if resultErr != nil && vector != nil {
			resultErr = errors.Join(resultErr, vector.Close())
			vector = nil
		}
	}()
	values := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		values[i], err = manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
	}
	after, err := f.command.Assertions()
	if err != nil {
		return nil, err
	}
	first := 0
	for first < len(before) && first < len(after) && before[first] == after[first] {
		first++
	}
	for _, assertion := range after[first:] {
		if err = f.command.AssertFixedpoint(assertion); err != nil {
			return nil, err
		}
	}
	context := &SMTContext{smtContextState: f.owner}
	return context.NewASTVectorValues(values...)
}
