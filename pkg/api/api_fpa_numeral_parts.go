package api

// Source: src/api/api_fpa.cpp:908-1158.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) bitVectorNumeralUint64(value ast.ASTHandle) (uint64, error) {
	text, err := c.NumeralString(value)
	if err != nil {
		return 0, err
	}
	n, ok := new(big.Int).SetString(text, 10)
	if !ok || n.Sign() < 0 || !n.IsUint64() {
		return 0, ast.ErrASTDomain
	}
	return n.Uint64(), nil
}

func (c *SMTContext) FloatingPointNumeralSign(value ast.ASTHandle) (int, error) {
	parts, err := c.floatingPointNumeralBits(value)
	if err != nil {
		return 0, err
	}
	bit, err := c.bitVectorNumeralUint64(parts.sign)
	return int(bit), err
}

func (c *SMTContext) FloatingPointNumeralSignificandUint64(value ast.ASTHandle) (uint64, error) {
	parts, err := c.floatingPointNumeralBits(value)
	if err != nil {
		return 0, err
	}
	return c.bitVectorNumeralUint64(parts.significand)
}

func (c *SMTContext) FloatingPointNumeralExponentInt64(value ast.ASTHandle, biased bool) (int64, error) {
	parts, err := c.floatingPointNumeralBits(value)
	if err != nil {
		return 0, err
	}
	encoded, err := c.bitVectorNumeralUint64(parts.exponent)
	if err != nil {
		return 0, err
	}
	zero, err := c.FloatingPointNumeralIsZero(value)
	if err != nil {
		return 0, err
	}
	infinite, err := c.FloatingPointNumeralIsInfinite(value)
	if err != nil {
		return 0, err
	}
	denormal, err := c.FloatingPointNumeralIsSubnormal(value)
	if err != nil {
		return 0, err
	}
	top := new(big.Int).Lsh(big.NewInt(1), uint(parts.ebits-1))
	if zero {
		return 0, nil
	}
	if infinite {
		if !top.IsInt64() {
			return 0, ast.ErrASTDomain
		}
		return top.Int64(), nil
	}
	if denormal {
		if biased {
			return 0, nil
		}
		minimum := new(big.Int).Sub(big.NewInt(2), top)
		if !minimum.IsInt64() {
			return 0, ast.ErrASTDomain
		}
		return minimum.Int64(), nil
	}
	if biased {
		if encoded > math.MaxInt64 {
			return 0, ast.ErrASTDomain
		}
		return int64(encoded), nil
	}
	bias := new(big.Int).Sub(top, big.NewInt(1))
	result := new(big.Int).Sub(new(big.Int).SetUint64(encoded), bias)
	if !result.IsInt64() {
		return 0, ast.ErrASTDomain
	}
	return result.Int64(), nil
}
