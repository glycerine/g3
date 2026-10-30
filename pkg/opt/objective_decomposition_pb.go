package opt

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// pseudoBooleanSum retains lia2card_tactic::get_pb_sum for the concrete set of
// integer 0/1 terms supplied by the caller after bound discovery.
func (d *ObjectiveDecomposer) pseudoBooleanSum(root ast.ASTHandle) (pbSum, bool, error) {
	var result pbSum
	one := big.NewRat(1, 1)
	ok, err := d.pbVisit(root, one, nil, &result)
	return result, ok, err
}

func (d *ObjectiveDecomposer) pbVisit(term ast.ASTHandle, multiplier *big.Rat, conditions []ast.ASTHandle, out *pbSum) (bool, error) {
	if numeral, yes, err := d.rational(term); err != nil {
		return false, err
	} else if yes {
		coefficient := ratMul(multiplier, &numeral)
		condition, e := d.conjunction(conditions, mustTrue(d.manager))
		if e != nil {
			return false, e
		}
		if d.manager.IsTrue(condition) {
			out.offset.Add(&out.offset, coefficient)
		} else {
			out.insert(condition, coefficient)
		}
		return true, nil
	}

	if _, yes := d.zeroOne[term]; yes {
		indicator, err := d.indicator(term)
		if err != nil {
			return false, err
		}
		condition, err := d.conjunction(conditions, indicator)
		if err != nil {
			return false, err
		}
		out.insert(condition, multiplier)
		return true, nil
	}

	if yes, err := d.appKind(term, d.arith, 6); err != nil {
		return false, err
	} else if yes {
		args, e := d.args(term)
		if e != nil {
			return false, e
		}
		for _, arg := range args {
			ok, e := d.pbVisit(arg, multiplier, conditions, out)
			if e != nil || !ok {
				return ok, e
			}
		}
		return true, nil
	}
	if yes, err := d.appKind(term, d.arith, 7); err != nil {
		return false, err
	} else if yes {
		args, e := d.args(term)
		if e != nil || len(args) != 2 {
			return false, e
		}
		ok, e := d.pbVisit(args[0], multiplier, conditions, out)
		if e != nil || !ok {
			return ok, e
		}
		return d.pbVisit(args[1], ratNeg(multiplier), conditions, out)
	}
	if yes, err := d.appKind(term, d.arith, 8); err != nil {
		return false, err
	} else if yes {
		args, e := d.args(term)
		if e != nil || len(args) != 1 {
			return false, e
		}
		return d.pbVisit(args[0], ratNeg(multiplier), conditions, out)
	}
	if yes, err := d.appKind(term, d.arith, 9); err != nil {
		return false, err
	} else if yes {
		args, e := d.args(term)
		if e != nil || len(args) != 2 {
			return false, e
		}
		if coefficient, found, e := d.rational(args[0]); e != nil {
			return false, e
		} else if found {
			return d.pbVisit(args[1], ratMul(multiplier, &coefficient), conditions, out)
		}
		if coefficient, found, e := d.rational(args[1]); e != nil {
			return false, e
		} else if found {
			return d.pbVisit(args[0], ratMul(multiplier, &coefficient), conditions, out)
		}
		return false, nil
	}
	if yes, err := d.appKind(term, d.arith, 14); err != nil {
		return false, err
	} else if yes {
		args, e := d.args(term)
		if e != nil || len(args) != 1 {
			return false, e
		}
		return d.pbVisit(args[0], multiplier, conditions, out)
	}
	if yes, err := d.appKind(term, d.manager.BasicFamilyID(), ast.ASTOpIte); err != nil {
		return false, err
	} else if yes {
		args, e := d.args(term)
		if e != nil || len(args) != 3 {
			return false, e
		}
		positive := appendCopy(conditions, args[0])
		ok, e := d.pbVisit(args[1], multiplier, positive, out)
		if e != nil || !ok {
			return ok, e
		}
		negative, e := d.negated(args[0])
		if e != nil {
			return false, e
		}
		return d.pbVisit(args[2], multiplier, appendCopy(conditions, negative), out)
	}
	return false, nil
}

func appendCopy(source []ast.ASTHandle, value ast.ASTHandle) []ast.ASTHandle {
	result := make([]ast.ASTHandle, len(source), len(source)+1)
	copy(result, source)
	return append(result, value)
}

func mustTrue(m *ast.ASTManager) ast.ASTHandle { h, _ := m.MkTrue(); return h }
