package opt

// Source: src/opt/sortmax.cpp mk_max definitional clauses.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) defineOr(output, left, right ast.ASTHandle) error {
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
	if err = e.clause(notLeft, output); err != nil {
		return err
	}
	if err = e.clause(notRight, output); err != nil {
		return err
	}
	return e.clause(left, right, notOutput)
}
