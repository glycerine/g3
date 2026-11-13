package smtcore

// Source: src/smt/theory_utvpi.cpp utvpi_tester::linearize overloads.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type smtUTVPITerm struct {
	expression  ast.ASTHandle
	coefficient big.Rat
}

type smtUTVPILinearizer struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	terms   []smtUTVPITerm
	weight  big.Rat
	// coeffMap is m_coeff_map: an obj_map whose table order (kept across
	// resets) decides the order of the linearization's terms.
	coeffMap     *ast.ObjMap
	coefficients []*big.Rat
}

func newSMTUTVPILinearizer(m *ast.ASTManager) (*smtUTVPILinearizer, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &smtUTVPILinearizer{manager: m, family: family, coeffMap: ast.NewObjMap(m)}, nil
}

func (l *smtUTVPILinearizer) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	value, _, matched, err := l.manager.ArithmeticNumeralBigRat(expression)
	if err != nil || !matched {
		if matched {
			return nil, false, ast.ErrASTDomain
		}
		return nil, false, err
	}
	return value, true, nil
}

// A null right operand selects the source single-term overload. Successful
// output is left-right = sum(terms)+weight, with at most two unit coefficients.
func (l *smtUTVPILinearizer) linearize(left, right ast.ASTHandle) (bool, error) {
	l.terms = nil
	l.weight.SetInt64(0)
	work := []smtUTVPITerm{{left, *big.NewRat(1, 1)}}
	if !right.IsNull() {
		work = append(work, smtUTVPITerm{right, *big.NewRat(-1, 1)})
	}
	l.coeffMap.Reset()
	l.coefficients = l.coefficients[:0]
	push := func(expression ast.ASTHandle, coefficient *big.Rat) {
		work = append(work, smtUTVPITerm{expression, *new(big.Rat).Set(coefficient)})
	}
	for len(work) > 0 {
		i := len(work) - 1
		term := work[i]
		work = work[:i]
		kind, err := l.manager.GetKind(term.expression)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := l.manager.GetFamilyID(term.expression)
		if err != nil {
			return false, err
		}
		count, err := l.manager.GetNumArgs(term.expression)
		if err != nil {
			return false, err
		}
		if family == l.family {
			operation, err := l.manager.GetDeclKind(term.expression)
			if err != nil {
				return false, err
			}
			switch operation {
			case smtArithOpAdd:
				for j := uint32(0); j < count; j++ {
					child, err := l.manager.GetArg(term.expression, j)
					if err != nil {
						return false, err
					}
					push(child, &term.coefficient)
				}
				continue
			case smtArithOpMul:
				if count == 2 {
					x, err := l.manager.GetArg(term.expression, 0)
					if err != nil {
						return false, err
					}
					y, err := l.manager.GetArg(term.expression, 1)
					if err != nil {
						return false, err
					}
					n, numeral, err := l.numeral(x)
					if err != nil {
						return false, err
					}
					if !numeral {
						x, y = y, x
						n, numeral, err = l.numeral(x)
						if err != nil {
							return false, err
						}
					}
					if numeral {
						push(y, new(big.Rat).Mul(&term.coefficient, n))
						continue
					}
				}
			case smtArithOpSub:
				if count == 2 {
					x, err := l.manager.GetArg(term.expression, 0)
					if err != nil {
						return false, err
					}
					y, err := l.manager.GetArg(term.expression, 1)
					if err != nil {
						return false, err
					}
					push(x, &term.coefficient)
					push(y, new(big.Rat).Neg(&term.coefficient))
					continue
				}
			case smtArithOpUMinus, smtArithOpToReal:
				if count == 1 {
					child, err := l.manager.GetArg(term.expression, 0)
					if err != nil {
						return false, err
					}
					coefficient := new(big.Rat).Set(&term.coefficient)
					if operation == smtArithOpUMinus {
						coefficient.Neg(coefficient)
					}
					push(child, coefficient)
					continue
				}
			}
			n, numeral, err := l.numeral(term.expression)
			if err != nil {
				return false, err
			}
			if numeral {
				l.weight.Add(&l.weight, new(big.Rat).Mul(n, &term.coefficient))
				continue
			}
		}
		if family != ast.NullFamilyID || count != 0 {
			return false, nil
		}
		// m_coeff_map.insert_if_not_there2(e, 0)->m_value += mul
		entry, err := l.coeffMap.InsertIfNotThere(term.expression)
		if err != nil {
			return false, err
		}
		if entry.Num == 0 {
			l.coefficients = append(l.coefficients, new(big.Rat))
			entry.Num = uint64(len(l.coefficients))
		}
		c := l.coefficients[entry.Num-1]
		c.Add(c, &term.coefficient)
	}
	for _, entry := range l.coeffMap.Entries() {
		coefficient := l.coefficients[entry.Num-1]
		if coefficient.Sign() == 0 {
			continue
		}
		l.terms = append(l.terms, smtUTVPITerm{entry.Key, *new(big.Rat).Set(coefficient)})
		if len(l.terms) > 2 {
			return false, nil
		}
		if coefficient.Cmp(big.NewRat(1, 1)) != 0 && coefficient.Cmp(big.NewRat(-1, 1)) != 0 {
			return false, nil
		}
	}
	return true, nil
}
