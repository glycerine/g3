package opt

// Source: src/opt/sortmax.cpp mk_min definitional clauses.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) defineAnd(output, left, right ast.ASTHandle) error {
	notOutput, err := e.negate(output)
	if err != nil {
		return err
	}
	notLeft, err := e.negate(left)
	if err != nil {
		return err
	}
	notRight, err := e.negate(right)
	if err != nil {
		return err
	}
	if err = e.clause(left, notOutput); err != nil {
		return err
	}
	if err = e.clause(right, notOutput); err != nil {
		return err
	}
	return e.clause(notLeft, notRight, output)
}
