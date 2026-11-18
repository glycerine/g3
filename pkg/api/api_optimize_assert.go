package api

// Source: src/api/api_opt.cpp:70-136.

import (
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/ast"
)

func (o *APIOptimize) Assert(formula ast.ASTHandle) error {
	if err := o.open(); err != nil {
		return err
	}
	return o.command.Assert(formula, "")
}

// AssertSoft is Z3_optimize_assert_soft with its rational weight: a rational
// of the AST manager's NumeralService().Manager, borrowed (the objective
// keeps a copy).
func (o *APIOptimize) AssertSoft(formula ast.ASTHandle, weight *numeral.Rational, id string) (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	return o.command.AddSoftConstraintIndexed(formula, weight, id)
}

// AssertSoftEmptySymbol is Z3_optimize_assert_soft with the non-null empty
// symbol as id (a $ || replay symbol; Z3_mk_string_symbol("") is null): its
// group is distinct from the null id's group "".
func (o *APIOptimize) AssertSoftEmptySymbol(formula ast.ASTHandle, weight *numeral.Rational) (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	return o.command.AddSoftConstraintEmptySymbolIndexed(formula, weight)
}

func (o *APIOptimize) Maximize(expression ast.ASTHandle) (uint32, error) {
	return o.addObjective(expression, true)
}

func (o *APIOptimize) Minimize(expression ast.ASTHandle) (uint32, error) {
	return o.addObjective(expression, false)
}

func (o *APIOptimize) addObjective(expression ast.ASTHandle, maximize bool) (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	index, err := o.command.ObjectiveCount()
	if err != nil {
		return 0, err
	}
	if err = o.command.AddObjective(expression, maximize); err != nil {
		return 0, err
	}
	return index, nil
}

func (o *APIOptimize) Push() error {
	if err := o.open(); err != nil {
		return err
	}
	return o.command.Push(1)
}

func (o *APIOptimize) Pop() error {
	if err := o.open(); err != nil {
		return err
	}
	return o.command.Pop(1)
}
