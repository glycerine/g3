package api

// Source: src/api/api_fpa.cpp:908-1001,1160-1198.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type floatingPointNumeralBits struct {
	sign, exponent, significand ast.ASTHandle
	ebits, sbits                uint32
}

func (c *SMTContext) floatingPointNumeralBits(value ast.ASTHandle) (floatingPointNumeralBits, error) {
	manager, _, err := c.floatingPointManager()
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	nan, err := c.FloatingPointNumeralIsNaN(value)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	if nan {
		return floatingPointNumeralBits{}, ast.ErrASTDomain
	}
	sort, err := manager.GetSort(value)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	ebits, sbits, matched, err := manager.FloatingPointSortShape(sort)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	if !matched || sbits < 2 {
		return floatingPointNumeralBits{}, ast.ErrASTDomain
	}
	packed, found, _, err := manager.RewriteFPAToIEEEBVNumeral(value, false)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	if !found {
		return floatingPointNumeralBits{}, ast.ErrASTDomain
	}
	text, err := c.NumeralString(packed)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	bits, ok := new(big.Int).SetString(text, 10)
	if !ok || bits.Sign() < 0 {
		return floatingPointNumeralBits{}, ast.ErrASTDomain
	}
	trailingWidth := sbits - 1
	signValue := new(big.Int).Rsh(new(big.Int).Set(bits), uint(ebits+trailingWidth))
	exponentMask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(ebits)), big.NewInt(1))
	exponentValue := new(big.Int).And(new(big.Int).Rsh(new(big.Int).Set(bits), uint(trailingWidth)), exponentMask)
	significandMask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(trailingWidth)), big.NewInt(1))
	significandValue := new(big.Int).And(new(big.Int).Set(bits), significandMask)
	signSort, err := c.BitVectorSort(1)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	exponentSort, err := c.BitVectorSort(ebits)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	significandSort, err := c.BitVectorSort(trailingWidth)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	sign, err := c.Numeral(signValue.String(), signSort)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	exponent, err := c.Numeral(exponentValue.String(), exponentSort)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	significand, err := c.Numeral(significandValue.String(), significandSort)
	if err != nil {
		return floatingPointNumeralBits{}, err
	}
	return floatingPointNumeralBits{sign: sign, exponent: exponent, significand: significand, ebits: ebits, sbits: sbits}, nil
}

func (c *SMTContext) FloatingPointNumeralSignBit(value ast.ASTHandle) (ast.ASTHandle, error) {
	parts, err := c.floatingPointNumeralBits(value)
	return parts.sign, err
}

func (c *SMTContext) FloatingPointNumeralSignificandBitVector(value ast.ASTHandle) (ast.ASTHandle, error) {
	parts, err := c.floatingPointNumeralBits(value)
	return parts.significand, err
}

func (c *SMTContext) FloatingPointNumeralExponentBitVector(value ast.ASTHandle, biased bool) (ast.ASTHandle, error) {
	parts, err := c.floatingPointNumeralBits(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := c.floatingPointNumeralExponentInt64Parts(value, parts, biased)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.BitVectorSort(parts.ebits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.BitVectorValue(exponent, sort)
}
