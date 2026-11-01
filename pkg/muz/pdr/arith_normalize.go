package pdr

// Source: src/muz/pdr/pdr_util.cpp arith_normalizer and normalize_arithmetic.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// NormalizeArithmetic divides both sides of a real comparison by the smallest
// nonzero absolute monomial coefficient, matching the source PDR normalizer.
// The returned AST is borrowed from the manager.
func NormalizeArithmetic(manager *ast.ASTManager, formula ast.ASTHandle) (ast.ASTHandle, error) {
	if manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return normalizeArithmeticTree(manager, formula, make(map[ast.ASTHandle]ast.ASTHandle))
}

// The source rewriter visits application children and quantifier bodies while
// leaving patterns unchanged. Rebuild with the original declaration so that
// parameters and uninterpreted symbols keep their identities.
func normalizeArithmeticTree(manager *ast.ASTManager, formula ast.ASTHandle, cache map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	if result, found := cache[formula]; found {
		return result, nil
	}
	kind, err := manager.GetKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result := formula
	if kind == ast.ASTQuantifier {
		body, err := manager.GetQuantifierBody(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		rewritten, err := normalizeArithmeticTree(manager, body, cache)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if rewritten != body {
			result, err = manager.UpdateQuantifierBody(formula, rewritten)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	} else if kind == ast.ASTApp {
		count, err := manager.GetNumArgs(formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, count)
		changed := false
		for i := range args {
			arg, err := manager.GetArg(formula, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args[i], err = normalizeArithmeticTree(manager, arg, cache)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			changed = changed || args[i] != arg
		}
		if changed {
			decl, err := manager.GetDecl(formula)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			result, err = manager.MkApp(decl, args)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		result, err = normalizeArithmeticComparison(manager, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	cache[formula] = result
	return result, nil
}

func normalizeArithmeticComparison(manager *ast.ASTManager, formula ast.ASTHandle) (ast.ASTHandle, error) {
	kind, err := manager.GetKind(formula)
	if err != nil || kind != ast.ASTApp {
		return formula, err
	}
	family, err := manager.GetFamilyID(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := manager.GetDeclKind(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negateResult := false
	if family == arithmetic && op == 4 {
		op, negateResult = 3, true
	} else if family == arithmetic && op == 5 {
		op, negateResult = 2, true
	}
	if (family == arithmetic && op != 2 && op != 3) || (family == 0 && op != ast.ASTOpEq) || (family != arithmetic && family != 0) {
		return formula, nil
	}
	left, err := manager.GetArg(formula, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := manager.GetArg(formula, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftSort, err := manager.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSort, err := manager.GetSort(right)
	if err != nil || leftSort != rightSort {
		return formula, err
	}
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftSort != realSort {
		if !negateResult {
			return formula, nil
		}
		return arithmeticNormalizerNegatedRelation(manager, family, op, left, right)
	}
	leftTerms, err := arithmeticNormalizerTerms(manager, left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightTerms, err := arithmeticNormalizerTerms(manager, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var divisor *big.Rat
	for _, terms := range [][]ast.ASTHandle{leftTerms, rightTerms} {
		for _, term := range terms {
			coefficient, _, err := arithmeticNormalizerCoefficient(manager, term)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if coefficient.Sign() == 0 {
				continue
			}
			coefficient.Abs(coefficient)
			if divisor == nil || coefficient.Cmp(divisor) < 0 {
				divisor = coefficient
			}
		}
	}
	if divisor == nil || divisor.Cmp(big.NewRat(1, 1)) == 0 {
		if !negateResult {
			return formula, nil
		}
		return arithmeticNormalizerNegatedRelation(manager, family, op, left, right)
	}
	left, err = arithmeticNormalizerDivide(manager, leftSort, leftTerms, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = arithmeticNormalizerDivide(manager, rightSort, rightTerms, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := []ast.ASTHandle{left, right}
	var result ast.ASTHandle
	if family == 0 {
		result, err = manager.MkFamilyApp(0, ast.ASTOpEq, nil, args)
	} else {
		result, err = manager.MkFamilyApp(family, op, nil, args)
	}
	if err != nil || !negateResult {
		return result, err
	}
	not, err := manager.MkFamilyApp(0, ast.ASTOpNot, nil, []ast.ASTHandle{result})
	return not, err
}

func arithmeticNormalizerNegatedRelation(manager *ast.ASTManager, family ast.FamilyID, op int32, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	relation, err := manager.MkFamilyApp(family, op, nil, []ast.ASTHandle{left, right})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFamilyApp(0, ast.ASTOpNot, nil, []ast.ASTHandle{relation})
}

func arithmeticNormalizerTerms(manager *ast.ASTManager, expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return nil, err
	}
	op, err := manager.GetDeclKind(expression)
	if err != nil || family != arithmetic || op != 6 {
		return []ast.ASTHandle{expression}, err
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	terms := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		term, err := manager.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, nil
}

func arithmeticNormalizerCoefficient(manager *ast.ASTManager, term ast.ASTHandle) (*big.Rat, ast.ASTHandle, error) {
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	if text, _, numeral, err := manager.ArithmeticNumeralText(term); err != nil {
		return nil, ast.ASTHandle{}, err
	} else if numeral {
		coefficient, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, ast.ASTHandle{}, ast.ErrASTDomain
		}
		return coefficient, ast.ASTHandle{}, nil
	}
	kind, err := manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return big.NewRat(1, 1), term, err
	}
	family, err := manager.GetFamilyID(term)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	op, err := manager.GetDeclKind(term)
	if err != nil || family != arithmetic || op != 9 {
		return big.NewRat(1, 1), term, err
	}
	count, err := manager.GetNumArgs(term)
	if err != nil || count != 2 {
		return big.NewRat(1, 1), term, err
	}
	coefficientTerm, err := manager.GetArg(term, 0)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	text, _, numeral, err := manager.ArithmeticNumeralText(coefficientTerm)
	if err != nil || !numeral {
		return big.NewRat(1, 1), term, err
	}
	coefficient, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ASTHandle{}, ast.ErrASTDomain
	}
	powerProduct, err := manager.GetArg(term, 1)
	return coefficient, powerProduct, err
}

func arithmeticNormalizerDivide(manager *ast.ASTManager, sort ast.ASTHandle, terms []ast.ASTHandle, divisor *big.Rat) (ast.ASTHandle, error) {
	args := make([]ast.ASTHandle, 0, len(terms))
	for _, term := range terms {
		coefficient, powerProduct, err := arithmeticNormalizerCoefficient(manager, term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		coefficient.Quo(coefficient, divisor)
		if powerProduct == (ast.ASTHandle{}) {
			if coefficient.Sign() == 0 {
				continue
			}
			text := coefficient.RatString()
			constant, err := manager.ArithmeticRealNumeralText(text)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args = append(args, constant)
			continue
		}
		if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
			args = append(args, powerProduct)
			continue
		}
		coefficientHandle, err := manager.ArithmeticRealNumeralText(coefficient.RatString())
		if err != nil {
			return ast.ASTHandle{}, err
		}
		product, err := manager.MkArithmeticMul(coefficientHandle, powerProduct)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, product)
	}
	if len(args) == 0 {
		return manager.ArithmeticRealNumeralText("0")
	}
	if len(args) == 1 {
		return args[0], nil
	}
	family, err := manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkFamilyApp(family, 6, nil, args)
}
