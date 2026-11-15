package opt

// Source: src/opt/sortmax.cpp unsigned-weight input expansion.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const maxSortMaxInputs = 4096

type maxSMTWeightedTerm struct {
	expression ast.ASTHandle
	weight     big.Int
}

// maxSMTScaledTerms gives sortmax's m_soft/m_weights as integers: every
// rational weight times scale, the least common multiple of the weight
// denominators. find_mutexes/process_mutex only add, subtract and compare
// weights, so running them on the scaled integers and dividing by scale
// afterwards yields exactly the rational new_soft weights and m_lower.
func maxSMTScaledTerms(manager *ast.ASTManager, objective *arithmeticObjective) ([]maxSMTWeightedTerm, *big.Int, error) {
	scale := big.NewInt(1)
	for i := range objective.softTerms {
		denominator := objective.softWeights[i].Denom()
		var gcd big.Int
		gcd.GCD(nil, nil, scale, denominator)
		scale.Mul(scale, new(big.Int).Quo(denominator, &gcd))
	}
	terms := make([]maxSMTWeightedTerm, len(objective.softTerms))
	for i := range objective.softTerms {
		expression, err := manager.RefGet(objective.softTerms[i])
		if err != nil {
			return nil, nil, err
		}
		terms[i].expression = expression
		weight := objective.softWeights[i]
		terms[i].weight.Mul(weight.Num(), new(big.Int).Quo(scale, weight.Denom()))
	}
	return terms, scale, nil
}

// maxSMTUnscaleUnsigned is rational::is_unsigned/get_unsigned of a scaled
// weight: the weight must be a non-negative integer below 2^32.
func maxSMTUnscaleUnsigned(weight, scale *big.Int) (uint32, bool) {
	var quotient, remainder big.Int
	quotient.QuoRem(weight, scale, &remainder)
	if remainder.Sign() != 0 || quotient.Sign() < 0 || !quotient.IsUint64() || quotient.Uint64() > uint64(^uint32(0)) {
		return 0, false
	}
	return uint32(quotient.Uint64()), true
}

func maxSMTExpandTerms(terms []maxSMTWeightedTerm, capacity int) []ast.ASTHandle {
	inputs := make([]ast.ASTHandle, 0, capacity)
	for i := range terms {
		for count := terms[i].weight.Int64(); count > 0; count-- {
			inputs = append(inputs, terms[i].expression)
		}
	}
	return inputs
}
