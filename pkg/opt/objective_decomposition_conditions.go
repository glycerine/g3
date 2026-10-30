package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *ObjectiveDecomposer) negated(term ast.ASTHandle) (ast.ASTHandle, error) {
	if yes, inner, err := d.manager.IsNot(term); err != nil {
		return ast.ASTHandle{}, err
	} else if yes {
		return inner, nil
	}
	h, err := d.manager.MkNot(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return d.retain(h)
}

func (d *ObjectiveDecomposer) conjunction(conditions []ast.ASTHandle, leaf ast.ASTHandle) (ast.ASTHandle, error) {
	args := make([]ast.ASTHandle, 0, len(conditions)+1)
	for _, c := range conditions {
		if d.manager.IsFalse(c) {
			return d.manager.MkFalse()
		}
		if !d.manager.IsTrue(c) {
			args = append(args, c)
		}
	}
	if !d.manager.IsTrue(leaf) {
		args = append(args, leaf)
	}
	if len(args) == 0 {
		return d.manager.MkTrue()
	}
	if len(args) == 1 {
		return args[0], nil
	}
	h, err := d.manager.MkAndArgs(args)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return d.retain(h)
}

func (d *ObjectiveDecomposer) indicator(term ast.ASTHandle) (ast.ASTHandle, error) {
	sort, err := d.manager.GetSort(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	family, err := d.manager.GetFamilyID(sort)
	if err != nil || family != d.arith {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	kind, err := d.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one := big.NewRat(1, 1)
	numeral, err := d.manager.ArithmeticNumeralRat(one, kind == 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	eq, err := d.manager.MkEq(term, numeral)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return d.retain(eq)
}
