package cmd_context

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// makeOPBTerm is opb::parse_term: the coefficient numeral, then the
// literals (parse_ids nests binary ands), then a fresh 0 and the ite.
func (c *Context) makeOPBTerm(term *opt.OPBTerm, variables map[uint32]bool) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := opbNumeral(manager, term.Coefficient)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var condition ast.ASTHandle
	for i, literal := range term.Literals {
		// parse_id: m.mk_const(symbol(id), m.mk_bool_sort()); symbol(int)
		// converts the int to its unsigned index, a numerical symbol
		// (printed k!id), not a cmd_context declaration.
		boolean, e := manager.MkBoolSort()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		index := uint32(literal.Variable)
		p, e := manager.MkConst(usymbol.Numerical(index), boolean)
		if e == nil && literal.Negative {
			p, e = manager.MkNot(p)
		}
		if e == nil && i > 0 {
			p, e = manager.MkAnd(condition, p)
		}
		if e != nil {
			return ast.ASTHandle{}, e
		}
		variables[index] = true
		condition = p
	}
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkIte(condition, value, zero)
}

func (c *Context) makeOPBSum(terms []opt.OPBTerm, add bool, variables map[uint32]bool) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.makeOPBTerm(&terms[0], variables)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := 1; i < len(terms); i++ {
		next, e := c.makeOPBTerm(&terms[i], variables)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if add {
			result, e = manager.MkArithmeticAdd(result, next)
		} else {
			result, e = manager.MkArithmeticSub(result, next)
		}
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return result, nil
}

// opbNumeral is arith.mk_numeral(rational(num.c_ptr()), true) for OPB's
// coefficient digit text.
func opbNumeral(manager *ast.ASTManager, text string) (_ ast.ASTHandle, err error) {
	q := manager.NumeralService().Manager
	r := numeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&r)) }()
	if err = q.SetString(&r, text); err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.ArithmeticNumeralFromRational(&r, true)
}
