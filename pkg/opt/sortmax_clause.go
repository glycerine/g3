package opt

// Source: src/opt/sortmax.cpp mk_clause.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// clause is s().assert_expr(mk_or(m, n, lits)): the disjunction is not
// trailed, so the solver's assertion is its only owner and simplification
// may release it.
func (e *sortMaxEncoder) clause(literals ...ast.ASTHandle) (result error) {
	manager := e.engine.solver.manager
	formula, err := manager.MkOrArray(literals)
	if err != nil {
		return err
	}
	ref, err := manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, manager.DelRef(&ref)) }()
	if err = e.engine.solver.solver().Assert(formula); err != nil {
		return err
	}
	e.engine.stats.Clauses++
	return nil
}
