package arith

// Source: the retained bounds_arith_subsumption instantiation in
// src/tactic/arith/arith_bounds_tactic.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type arithmeticBoundInfo struct {
	value  *unumeral.Rational // a rational of the manager's NumeralService().Manager
	index  int
	strict bool
}

func arithmeticBoundsApp(manager *ast.ASTManager, expression ast.ASTHandle) (operation int32, left, right ast.ASTHandle, matched bool, err error) {
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, left, right, false, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil || family != arithmetic {
		return 0, left, right, false, err
	}
	operation, err = manager.GetDeclKind(expression)
	if err != nil || operation < 2 || operation > 5 {
		return 0, left, right, false, err
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return 0, left, right, false, err
	}
	left, err = manager.GetArg(expression, 0)
	if err != nil {
		return 0, left, right, false, err
	}
	right, err = manager.GetArg(expression, 1)
	return operation, left, right, err == nil, err
}

func arithmeticBoundsRelation(manager *ast.ASTManager, expression ast.ASTHandle) (left, right ast.ASTHandle, strict, matched bool, err error) {
	negated, argument, err := manager.IsNot(expression)
	if err != nil {
		return left, right, false, false, err
	}
	if negated {
		expression = argument
	}
	operation, first, second, matched, err := arithmeticBoundsApp(manager, expression)
	if err != nil || !matched {
		return left, right, false, false, err
	}
	switch operation {
	case 2: // <=
		left, right, strict = first, second, negated
	case 3: // >=
		left, right, strict = second, first, negated
	case 4: // <
		left, right, strict = first, second, !negated
	case 5: // >
		left, right, strict = second, first, !negated
	}
	if negated {
		left, right = right, left
	}
	return left, right, strict, true, nil
}

// arithmeticBoundsNumeral is a.is_numeral(e, r) into value.
func arithmeticBoundsNumeral(manager *ast.ASTManager, expression ast.ASTHandle, value *unumeral.Rational) (matched bool, err error) {
	_, matched, err = manager.ArithmeticNumeralRational(expression, value)
	return matched && err == nil, err
}

// arithmeticBoundsImproves is the subsumption test against the previous
// bound: a strictly better value, or an equal one that becomes strict.
func arithmeticBoundsImproves(q *unumeral.RationalManager, previous arithmeticBoundInfo, found bool, value *unumeral.Rational, strict, upper bool) (bool, error) {
	if !found {
		return true, nil
	}
	cmp, err := q.Compare(previous.value, value)
	if err != nil {
		return false, err
	}
	if upper && cmp > 0 || !upper && cmp < 0 {
		return true, nil
	}
	return cmp == 0 && strict && !previous.strict, nil
}

// ArithmeticBoundsKeep returns the source arithmetic-bound subsumption mask.
// It is the concrete, proof-disabled instantiation used to simplify Farkas
// lemmas: for each term it retains only the strongest lower and upper bound.
func ArithmeticBoundsKeep(manager *ast.ASTManager, formulas []ast.ASTHandle) (_ []bool, resultErr error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	q := manager.NumeralService().Manager
	// The bound maps' rationals and the scratch value.
	var owned []*unumeral.Rational
	defer func() {
		for _, v := range owned {
			resultErr = errors.Join(resultErr, q.Del(v))
		}
	}()
	newValue := func() *unumeral.Rational {
		v := new(unumeral.Rational)
		*v = unumeral.NewRational()
		owned = append(owned, v)
		return v
	}
	value := newValue()
	keep := make([]bool, len(formulas))
	for i := range keep {
		keep[i] = true
	}
	lower := make(map[ast.ASTHandle]arithmeticBoundInfo)
	upper := make(map[ast.ASTHandle]arithmeticBoundInfo)
	for index, formula := range formulas {
		canceled, err := manager.Canceled()
		if err != nil {
			return nil, err
		}
		if canceled {
			return nil, z3exception.NewDefaultException(messages.Canceled)
		}
		left, right, strict, matched, err := arithmeticBoundsRelation(manager, formula)
		if err != nil || !matched {
			if err != nil {
				return nil, err
			}
			continue
		}
		if numeral, err := arithmeticBoundsNumeral(manager, right, value); err != nil {
			return nil, err
		} else if numeral {
			previous, found := upper[left]
			better, err := arithmeticBoundsImproves(q, previous, found, value, strict, true)
			if err != nil {
				return nil, err
			}
			if better {
				if found {
					keep[previous.index] = false
				}
				upper[left] = arithmeticBoundInfo{value: value, index: index, strict: strict}
				value = newValue()
			} else {
				keep[index] = false
			}
			continue
		}
		numeral, err := arithmeticBoundsNumeral(manager, left, value)
		if err != nil {
			return nil, err
		}
		if !numeral {
			continue
		}
		previous, found := lower[right]
		better, err := arithmeticBoundsImproves(q, previous, found, value, strict, false)
		if err != nil {
			return nil, err
		}
		if better {
			if found {
				keep[previous.index] = false
			}
			lower[right] = arithmeticBoundInfo{value: value, index: index, strict: strict}
			value = newValue()
		} else {
			keep[index] = false
		}
	}
	return keep, nil
}
