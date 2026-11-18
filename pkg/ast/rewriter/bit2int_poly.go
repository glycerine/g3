package rewriter

// Source: src/ast/rewriter/bit2int.cpp:185-235.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (b *Bit2Int) isArithmeticOperation(term ast.ASTHandle, kind int32) (bool, error) {
	actualKind, err := b.manager.GetKind(term)
	if err != nil || actualKind != ast.ASTApp {
		return false, err
	}
	family, err := b.manager.GetFamilyID(term)
	if err != nil || family != b.arithFamily {
		return false, err
	}
	op, err := b.manager.GetDeclKind(term)
	return op == kind, err
}

// arithmeticInteger is is_numeral(n, k, is_int) && is_int into k, a
// rational of the AST manager's NumeralService().Manager.
func (b *Bit2Int) arithmeticInteger(term ast.ASTHandle, k *unumeral.Rational) (bool, error) {
	integer, matched, err := b.manager.ArithmeticNumeralRational(term, k)
	return err == nil && matched && integer, err
}

// positiveArithmeticNumeral is m_arith_util.mk_numeral(-k, true).
func (b *Bit2Int) positiveArithmeticNumeral(c *bit2IntCall, k *unumeral.Rational) (ast.ASTHandle, error) {
	service := b.manager.NumeralService()
	rational, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer service.Del(&rational)
	if err = errors.Join(service.Manager.Copy(&rational.Raw, k), service.Manager.Neg(&rational.Raw)); err != nil {
		return ast.ASTHandle{}, err
	}
	term, err := b.manager.ArithmeticNumeral(&rational, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(term)
}

func (b *Bit2Int) isBVPoly(c *bit2IntCall, root ast.ASTHandle) (pos, neg ast.ASTHandle, matched bool, err error) {
	// numeral k;
	q := b.manager.NumeralService().Manager
	value := unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&value)) }()
	zero, err := b.manager.RefGet(b.bit0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	pos, err = b.mkBV2Int(c, zero)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	neg = pos
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		term := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		bv2int, err := b.manager.IsBitVectorOperation(term, 58)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if bv2int {
			pos, matched, err = b.mkAdd(c, term, pos)
			if err != nil || !matched {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			continue
		}
		numeral, err := b.arithmeticInteger(term, &value)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if numeral {
			if !value.IsNeg() {
				pos, matched, err = b.mkAdd(c, term, pos)
			} else {
				positive, x := b.positiveArithmeticNumeral(c, &value)
				if x != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, x
				}
				neg, matched, err = b.mkAdd(c, positive, neg)
			}
			if err != nil || !matched {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			continue
		}
		add, err := b.isArithmeticOperation(term, 6)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if add {
			count, err := b.manager.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			for i := uint32(0); i < count; i++ {
				arg, err := b.manager.GetArg(term, i)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				stack = append(stack, arg)
			}
			continue
		}
		mul, err := b.isArithmeticOperation(term, 9)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if mul {
			count, err := b.manager.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			if count == 2 {
				negativeProduct := false
				left, err := b.manager.GetArg(term, 0)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				right, err := b.manager.GetArg(term, 1)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
					numeral, err := b.arithmeticInteger(pair[0], &value)
					if err != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, false, err
					}
					other, err := b.manager.IsBitVectorOperation(pair[1], 58)
					if err != nil {
						return ast.ASTHandle{}, ast.ASTHandle{}, false, err
					}
					if numeral && value.IsMinusOne() && other {
						neg, matched, err = b.mkAdd(c, pair[1], neg)
						if err != nil || !matched {
							return ast.ASTHandle{}, ast.ASTHandle{}, false, err
						}
						negativeProduct = true
						break
					}
				}
				if negativeProduct {
					continue
				}
			}
		}
		unaryMinus, err := b.isArithmeticOperation(term, 8)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
		if unaryMinus {
			count, err := b.manager.GetNumArgs(term)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, false, err
			}
			if count == 1 {
				arg, err := b.manager.GetArg(term, 0)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				bv2int, err := b.manager.IsBitVectorOperation(arg, 58)
				if err != nil {
					return ast.ASTHandle{}, ast.ASTHandle{}, false, err
				}
				if bv2int {
					neg, matched, err = b.mkAdd(c, arg, neg)
					if err != nil || !matched {
						return ast.ASTHandle{}, ast.ASTHandle{}, false, err
					}
					continue
				}
			}
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	return pos, neg, true, nil
}
