package smtcore

// Temporary math/big views of the difference-logic syntax helpers for the
// dense theory, removed when the dense theory moves to its Ext numerals.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTIntegerDiffTheory) numeralBig(expression ast.ASTHandle) (*big.Rat, bool, error) {
	r := smtQZero()
	defer smtQDel(t.q, &r)
	ok, err := t.numeral(expression, &r)
	if err != nil || !ok {
		return nil, false, err
	}
	b, err := smtQBig(t.q, &r)
	return b, err == nil, err
}

func (t *SMTIntegerDiffTheory) offsetBig(expression ast.ASTHandle) (ast.ASTHandle, *big.Rat, bool, error) {
	r := smtQZero()
	defer smtQDel(t.q, &r)
	a, ok, err := t.offset(expression, &r)
	if err != nil || !ok {
		return a, nil, false, err
	}
	b, err := smtQBig(t.q, &r)
	return a, b, err == nil, err
}
