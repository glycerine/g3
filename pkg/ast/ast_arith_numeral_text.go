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

// IsArithmeticNumeral is ArithmeticNumeralText's matched result for callers
// that only test for a numeral. A small rational is not formatted (its text
// never goes through mpn); a large one still is, so the mpn_to_string trace
// and String's errors are unchanged.
func (m *ASTManager) IsArithmeticNumeral(h ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	rational, _, matched, err := m.arithmeticNumeralParameter(h)
	if err != nil || !matched {
		return false, err
	}
	if raw := &rational.Raw; raw.Num.Cell == 0 && raw.Den.Cell == 0 && raw.Den.Val > 0 {
		return true, nil
	}
	if _, err = m.numerals.String(rational); err != nil {
		return false, err
	}
	return true, nil
}

// ArithmeticNumeralIsZero is arith_util::is_zero: h is an arithmetic
// numeral whose value is 0. It reads the numeral's rational parameter in
// place.
func (m *ASTManager) ArithmeticNumeralIsZero(h ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	rational, _, matched, err := m.arithmeticNumeralParameter(h)
	if err != nil || !matched {
		return false, err
	}
	if raw := &rational.Raw; raw.Num.Cell == 0 && raw.Den.Cell == 0 && raw.Den.Val > 0 {
		return raw.Num.Val == 0, nil
	}
	return m.numerals.IsZero(rational)
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
	if direct, done, e := m.numerals.BigRat(rational); e != nil {
		return nil, false, false, e
	} else if done {
		return direct, integer, true, nil
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

// ArithmeticNumeralRational is arith_util::is_numeral(n, r) for a caller
// that holds rationals of this manager's rational manager
// (NumeralService().Manager): it copies the numeral's rational parameter
// into out, as the source's rational copy constructor does.
func (m *ASTManager) ArithmeticNumeralRational(h ASTHandle, out *unumeral.Rational) (integer, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	rational, integer, matched, err := m.arithmeticNumeralParameter(h)
	if err != nil || !matched {
		return integer, false, err
	}
	return integer, true, m.numerals.Manager.Copy(out, &rational.Raw)
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
