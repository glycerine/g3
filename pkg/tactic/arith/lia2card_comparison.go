package arith

// Source: src/tactic/arith/lia2card_tactic.cpp is_pb, mk_le, mk_ge and mk_eq.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// lia2cardComparison returns an owning Boolean result for left <= right, or
// left = right when equality is true. Unsupported sums return matched=false.
func lia2cardComparison(m *ast.ASTManager, left, right ast.ASTHandle, zeroOne map[ast.ASTHandle]bool, equality, compileEquality bool, tags *diagnostic.TraceTags) (result ast.ASTRef, matched bool, err error) {
	return lia2cardComparisonRetaining(m, left, right, zeroOne, equality, compileEquality, tags, nil)
}

// lia2cardComparisonRetaining keeps the matched sum in *args, which is
// lia_rewriter_cfg::args: is_pb resets it at its next call, so the
// collected terms outlive the rewrite step. A nil args releases on return.
func lia2cardComparisonRetaining(m *ast.ASTManager, left, right ast.ASTHandle, zeroOne map[ast.ASTHandle]bool, equality, compileEquality bool, tags *diagnostic.TraceTags, args **PBSum) (result ast.ASTRef, matched bool, err error) {
	if m == nil {
		return result, false, ast.ErrASTHandle
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result, matched = ast.ASTRef{}, false
		}
	}()
	// is_pb: get_pb_sum(x, 1, ...) && get_pb_sum(y, -1, ...), with no
	// difference term built.
	if args != nil && *args != nil {
		previous := *args
		*args = nil
		if err = previous.Close(); err != nil {
			return result, false, err
		}
	}
	sum, matched, err := extractPBSumTerms(m, []ast.ASTHandle{left, right}, []*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 1)}, zeroOne, tags)
	if err != nil || !matched {
		return result, matched, err
	}
	if args != nil {
		*args = sum
	} else {
		defer func() { err = errors.Join(err, sum.Close()) }()
	}
	if _, err = m.RegisterPseudoBooleanPlugin(); err != nil {
		return result, false, err
	}
	service := m.NumeralService()
	weights := make([]unumeral.OwnedRational, len(sum.Coefficients))
	var initialized int
	defer func() {
		for j := initialized - 1; j >= 0; j-- {
			err = errors.Join(err, service.Del(&weights[j]))
		}
	}()
	for j := range weights {
		weights[j], err = service.NewValue()
		if err != nil {
			return result, false, err
		}
		initialized++
		if err = service.SetString(&weights[j], sum.Coefficients[j].RatString()); err != nil {
			return result, false, err
		}
	}
	boundRat := new(big.Rat).Neg(&sum.Constant)
	bound, err := service.NewValue()
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, service.Del(&bound)) }()
	if err = service.SetString(&bound, boundRat.RatString()); err != nil {
		return result, false, err
	}
	makeLE := func() (ast.ASTHandle, error) {
		if len(weights) == 0 {
			if boundRat.Sign() < 0 {
				return m.MkFalse()
			}
			return m.MkTrue()
		}
		if boundRat.Sign() < 0 {
			return m.MkFalse()
		}
		if len(weights) == 1 && sum.Coefficients[0].Cmp(big.NewRat(1, 1)) == 0 {
			if boundRat.Cmp(big.NewRat(1, 1)) >= 0 {
				return m.MkTrue()
			}
			if boundRat.Sign() == 0 {
				return m.MkNot(sum.Args[0])
			}
		}
		return m.MkPseudoBooleanLE(weights, sum.Args, &bound)
	}
	makeGE := func() (ast.ASTHandle, error) {
		if len(weights) == 0 {
			if boundRat.Sign() > 0 {
				return m.MkFalse()
			}
			return m.MkTrue()
		}
		if len(weights) == 1 && sum.Coefficients[0].Cmp(big.NewRat(1, 1)) == 0 {
			if boundRat.Cmp(big.NewRat(1, 1)) == 0 {
				return sum.Args[0], nil
			}
			if boundRat.Sign() == 0 {
				return m.MkNot(sum.Args[0])
			}
		}
		if boundRat.Sign() < 0 {
			return m.MkTrue()
		}
		return m.MkPseudoBooleanGE(weights, sum.Args, &bound)
	}
	var output ast.ASTHandle
	if !equality {
		output, err = makeLE()
	} else if boundRat.Sign() < 0 {
		output, err = m.MkFalse()
	} else if len(weights) == 0 {
		if boundRat.Sign() == 0 {
			output, err = m.MkTrue()
		} else {
			output, err = m.MkFalse()
		}
	} else if compileEquality {
		output, err = m.MkPseudoBooleanEQ(weights, sum.Args, &bound)
	} else {
		var ge, le ast.ASTHandle
		ge, err = makeGE()
		if err == nil {
			var geRef ast.ASTRef
			geRef, err = m.NewRef(ge)
			if err == nil {
				le, err = makeLE()
				if err == nil {
					output, err = m.MkAnd(ge, le)
				}
				err = errors.Join(err, m.DelRef(&geRef))
			}
		}
	}
	if err != nil {
		return result, false, err
	}
	result, err = m.NewRef(output)
	if err != nil {
		return result, false, err
	}
	return result, true, nil
}
