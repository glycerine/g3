package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

// compileKarrGuardCondition builds native filter rows for one conjunct.
func (a *karrDatalogActivation) compileKarrGuardCondition(guard ast.ASTHandle, positions map[ast.ASTHandle]uint32, arity uint32) (karrGuardRule, error) {
	var out karrGuardRule
	family, err := a.manager.GetFamilyID(guard)
	if err != nil {
		return out, err
	}
	op, err := a.manager.GetDeclKind(guard)
	if err != nil {
		return out, err
	}
	guardArity, err := a.manager.GetNumArgs(guard)
	if err != nil {
		return out, err
	}
	negated := family == a.manager.BasicFamilyID() && op == ast.ASTOpNot
	if negated {
		if guardArity != 1 {
			return out, ErrKarrDatalogFragment
		}
		guard, err = a.manager.GetArg(guard, 0)
		if err != nil {
			return out, err
		}
		family, err = a.manager.GetFamilyID(guard)
		if err != nil {
			return out, err
		}
		op, err = a.manager.GetDeclKind(guard)
		if err != nil {
			return out, err
		}
		guardArity, err = a.manager.GetNumArgs(guard)
		if err != nil {
			return out, err
		}
	}
	if guardArity != 2 {
		return out, ErrKarrDatalogFragment
	}
	if !negated && family == a.manager.BasicFamilyID() && op == ast.ASTOpOr {
		if err = a.compileKarrDisjunction(guard, positions, arity, &out); err != nil {
			return out, err
		}
		return out, nil
	}
	leftMul, rightMul := int64(1), int64(-1)
	if !negated && family == a.manager.BasicFamilyID() && op == ast.ASTOpEq {
		out.equal = true
	} else {
		arith, err := a.manager.RegisterArithmeticPlugin()
		if err != nil {
			return out, err
		}
		if family != arith || op < 2 || op > 5 {
			return out, ErrKarrDatalogFragment
		}
		if negated {
			// not (<=, >=, <, >) becomes (>, <, >=, <=).
			switch op {
			case 2:
				op = 5
			case 3:
				op = 4
			case 4:
				op = 3
			case 5:
				op = 2
			}
		}
		if op == 2 || op == 4 {
			leftMul, rightMul = -1, 1
		}
		if op == 4 || op == 5 {
			out.constant.SetInt64(-1)
		}
	}
	out.row = make([]big.Rat, arity)
	arithmetic, err := a.manager.RegisterArithmeticPlugin()
	if err != nil {
		return out, err
	}
	integer, err := a.manager.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		return out, err
	}
	add := func(expr ast.ASTHandle, factor int64) error {
		return a.addKarrLinear(expr, big.NewRat(factor, 1), positions, out.row, &out.constant, integer, arithmetic)
	}
	left, err := a.manager.GetArg(guard, 0)
	if err != nil {
		return out, err
	}
	right, err := a.manager.GetArg(guard, 1)
	if err != nil {
		return out, err
	}
	if err = add(left, leftMul); err != nil {
		return out, err
	}
	if err = add(right, rightMul); err != nil {
		return out, err
	}
	return out, nil
}
