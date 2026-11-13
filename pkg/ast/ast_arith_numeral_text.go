package ast

import (
	"math/big"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Source: src/ast/arith_decl_plugin.cpp:632-639.

// ArithmeticNumeralText reads the exact numeral value and integer flag for
// the retained arithmetic-tactic consumer. The returned Go string is detached
// from the AST parameter's numeral owner.
func (m *ASTManager) ArithmeticNumeralText(h ASTHandle) (value string, integer, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	rational, integer, matched, err := m.arithmeticNumeralParameter(h)
	if err != nil || !matched {
		return "", integer, false, err
	}
	value, err = m.numerals.String(rational)
	if err != nil {
		return "", false, false, err
	}
	return value, integer, true, nil
}

// ArithmeticNumeralBigRat is ArithmeticNumeralText followed by
// big.Rat.SetString of the text, without the text when the numeral is small.
// A text that does not parse reports ErrASTDomain with matched set.
func (m *ASTManager) ArithmeticNumeralBigRat(h ASTHandle) (value *big.Rat, integer, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	rational, integer, matched, err := m.arithmeticNumeralParameter(h)
	if err != nil || !matched {
		return nil, integer, false, err
	}
	if raw := &rational.Raw; raw.Num.Cell == 0 && raw.Den.Cell == 0 && raw.Den.Val > 0 {
		if raw.Den.Val == 1 {
			// An integer needs no normalization; big.NewRat would run a GCD.
			return new(big.Rat).SetInt64(int64(raw.Num.Val)), integer, true, nil
		}
		return big.NewRat(int64(raw.Num.Val), int64(raw.Den.Val)), integer, true, nil
	}
	text, err := m.numerals.String(rational)
	if err != nil {
		return nil, false, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, integer, true, ErrASTDomain
	}
	return value, integer, true, nil
}

func (m *ASTManager) arithmeticNumeralParameter(h ASTHandle) (rational *unumeral.OwnedRational, integer, matched bool, err error) {
	r, err := m.requireExpr(h)
	if err != nil {
		return
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.arithmeticPlugin == nil {
		return
	}
	family, kind, err := m.appIdentity(m.appAt(r.slot))
	if err != nil || family != m.arithmeticPlugin.family || kind != 0 {
		return
	}
	info, err := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if err != nil {
		return
	}
	p, err := m.infoParameter(info, 0)
	if err != nil {
		return
	}
	first, err := m.parameter(p)
	if err != nil {
		return
	}
	if first.Kind != ASTParameterRational {
		err = ErrASTDomain
		return
	}
	p, err = m.infoParameter(info, 1)
	if err != nil {
		return
	}
	second, err := m.parameter(p)
	if err != nil {
		return
	}
	if second.Kind != ASTParameterInt {
		err = ErrASTDomain
		return
	}
	return &first.Rational, second.integer != 0, true, nil
}
