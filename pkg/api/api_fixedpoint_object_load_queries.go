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
// Parsing completes in a separate context before the fixedpoint is changed.
func (f *APIFixedpoint) LoadSMT2WithQueries(input io.Reader) (vector *ASTVector, resultErr error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, ErrSMTContextOwner
	}
	context := &SMTContext{smtContextState: f.owner}
	parser, err := context.newSMT2ParserContext(SMT2Bindings{})
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, parser.Close())
		if resultErr != nil && vector != nil {
			resultErr = errors.Join(resultErr, vector.Close())
			vector = nil
		}
	}()
	manager, err := parser.Manager()
	if err != nil {
		return nil, err
	}
	queries, err := smt2.ParseAssertionsWithFixedpointQueries(parser, input)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range queries {
			resultErr = errors.Join(resultErr, manager.DelRef(&queries[i]))
		}
	}()
	predicates, err := parser.FixedpointPredicates()
	if err != nil {
		return nil, err
	}
	rules, names, err := parser.FixedpointNamedRules()
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range rules {
			resultErr = errors.Join(resultErr, manager.DelRef(&rules[i]))
		}
	}()
	assertions, err := parser.Assertions()
	if err != nil {
		return nil, err
	}
	values := make([]ast.ASTHandle, len(queries))
	for i := range queries {
		values[i], err = manager.RefGet(queries[i])
		if err != nil {
			return nil, err
		}
	}
	vector, err = context.NewASTVectorValues(values...)
	if err != nil {
		return nil, err
	}
	for _, predicate := range predicates {
		if err = f.command.RegisterFixedpointRelation(predicate); err != nil {
			return vector, err
		}
	}
	for i := range rules {
		h, e := manager.RefGet(rules[i])
		if e != nil {
			return vector, e
		}
		if e = f.command.AddFixedpointRule(h, names[i]); e != nil {
			return vector, e
		}
	}
	for _, assertion := range assertions {
		if err = f.command.AssertFixedpoint(assertion); err != nil {
			return vector, err
		}
	}
	return vector, nil
}
