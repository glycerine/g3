package ast

// Source: src/util/mpf.cpp mpf_manager::set(o, ebits, sbits, rm, mpq) and
// set(o, ebits, sbits, rm, mpz exponent, mpq significand), read back through
// sgn/sig/exp/is_nan/is_inf as fpa2bv_converter does, and
// fpa_decl_plugin::mk_numeral(mpf) for the to_fp real/int conversion.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// FPAMpfFields are an mpf's fields as fpa2bv_converter reads them. The
// significand is the decimal text of the mpf manager's mpz: that manager
// owns its own integers, apart from the AST manager's rational manager.
type FPAMpfFields struct {
	NaN, Inf, Zero bool
	Sign           bool
	Significand    string
	Exponent       int64
}

// FPARoundingMode maps BV_RM_VAL (0 ties-to-even, 1 ties-to-away, 2 toward
// positive, 3 toward negative, 4 toward zero) to mpf_rounding_mode.
func FPARoundingMode(bvRM int64) (unumeral.ArbitraryFloatRoundingMode, error) {
	switch bvRM {
	case 0:
		return unumeral.ArbitraryFloatNearestEven, nil
	case 1:
		return unumeral.ArbitraryFloatNearestAway, nil
	case 2:
		return unumeral.ArbitraryFloatTowardPositive, nil
	case 3:
		return unumeral.ArbitraryFloatTowardNegative, nil
	case 4:
		return unumeral.ArbitraryFloatTowardZero, nil
	}
	return 0, ErrASTDomain
}

// FPAMpfSetRational is set(v, ebits, sbits, rm, q) followed by its fields;
// q is a rational of NumeralService().Manager.
func (m *ASTManager) FPAMpfSetRational(ebits, sbits uint32, mode unumeral.ArbitraryFloatRoundingMode, q *unumeral.Rational) (FPAMpfFields, error) {
	return m.fpaMpfSet(ebits, sbits, mode, nil, q, false)
}

// FPAMpfSetExponentRational is set(v, ebits, sbits, rm, e, q) followed by
// its fields.
// e (an integer) and q are rationals of NumeralService().Manager.
func (m *ASTManager) FPAMpfSetExponentRational(ebits, sbits uint32, mode unumeral.ArbitraryFloatRoundingMode, e, q *unumeral.Rational) (FPAMpfFields, error) {
	return m.fpaMpfSet(ebits, sbits, mode, e, q, false)
}

// MkFPANumeralExponentRational is m_plugin->mk_numeral(v) for v set from
// (rm, e, q).
// e (an integer) and q are rationals of NumeralService().Manager.
func (m *ASTManager) MkFPANumeralExponentRational(ebits, sbits uint32, mode unumeral.ArbitraryFloatRoundingMode, e, q *unumeral.Rational) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	var out ASTHandle
	err := m.withMpf(ebits, sbits, mode, e, q, func(p *astFloatingPointPluginOwner, v *unumeral.ArbitraryFloat) error {
		node, err := p.mkNumeral(v, nil)
		out = m.external(node)
		return err
	})
	return out, err
}

func (m *ASTManager) fpaMpfSet(ebits, sbits uint32, mode unumeral.ArbitraryFloatRoundingMode, e, q *unumeral.Rational, _ bool) (fields FPAMpfFields, result error) {
	if err := m.lock(); err != nil {
		return fields, err
	}
	defer m.mu.Unlock()
	result = m.withMpf(ebits, sbits, mode, e, q, func(p *astFloatingPointPluginOwner, v *unumeral.ArbitraryFloat) error {
		var err error
		if fields.NaN, err = p.numeric.IsNaN(v); err != nil {
			return err
		}
		if fields.Inf, err = p.numeric.IsInf(v); err != nil {
			return err
		}
		if fields.Zero, err = p.numeric.IsZero(v); err != nil {
			return err
		}
		fields.Sign = v.Sign()
		fields.Exponent = v.Exponent()
		integer, err := p.numeric.BorrowIntegerManager()
		if err != nil {
			return err
		}
		sig, err := p.numeric.BorrowSignificand(v)
		if err != nil {
			return err
		}
		fields.Significand, err = integer.String(sig)
		return err
	})
	return fields, result
}

// withMpf sets a scoped mpf and runs body with it; the manager lock is held.
func (m *ASTManager) withMpf(ebits, sbits uint32, mode unumeral.ArbitraryFloatRoundingMode, e, q *unumeral.Rational, body func(*astFloatingPointPluginOwner, *unumeral.ArbitraryFloat) error) (result error) {
	if q == nil {
		return ErrASTDomain
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ErrASTHandle
	}
	v, err := p.beginNumeralProbe()
	if err != nil {
		return err
	}
	found := true
	defer p.finishNumeralProbe(v, &result, &found)
	qm, err := p.numeric.BorrowRationalManager()
	if err != nil {
		return err
	}
	r := unumeral.NewRational()
	var exponent unumeral.Int
	defer func() {
		result = errors.Join(result, qm.Del(&r))
		result = errors.Join(result, qm.Z.Del(&exponent))
	}()
	// The mpf manager's rationals are its own: q crosses as its text.
	text, err := m.numerals.Manager.String(q)
	if err != nil {
		return err
	}
	if err = qm.SetString(&r, text); err != nil {
		return err
	}
	if e != nil {
		if !e.IsInt() {
			return ErrASTDomain
		}
		if text, err = m.numerals.Manager.String(e); err != nil {
			return err
		}
		if err = qm.Z.SetString(&exponent, text); err != nil {
			return err
		}
		err = p.numeric.SetExponentRational(&v.value, ebits, sbits, mode, &exponent, &r)
	} else {
		err = p.numeric.SetRational(&v.value, ebits, sbits, mode, &r)
	}
	if err != nil {
		return err
	}
	return body(p, &v.value)
}
