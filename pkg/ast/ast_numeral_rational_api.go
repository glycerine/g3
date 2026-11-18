package ast

// Source: src/ast/bv_decl_plugin.cpp:865-868 (bv_util::mk_numeral(rational,
// unsigned)); src/ast/bv_decl_plugin.cpp:778-786 (bv_recognizers::is_numeral);
// src/ast/ast.h:124 (parameter(rational const&)).
//
// Rational-valued entry points for callers that hold rationals of this
// manager's rational manager (NumeralService().Manager), as z3's callers
// hold `rational` values.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// BitVectorNumeralFromRational is bv_util::mk_numeral(val, bv_size) for a
// rational of NumeralService().Manager.
func (m *ASTManager) BitVectorNumeralFromRational(r *unumeral.Rational, bits uint32) (result ASTHandle, resultErr error) {
	if r == nil {
		return ASTHandle{}, ErrASTDomain
	}
	service := m.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	if err = service.Manager.Copy(&value.Raw, r); err != nil {
		return ASTHandle{}, err
	}
	return m.BitVectorNumeral(&value, bits)
}

// BitVectorNumeralRational is bv_util::is_numeral(e, val, bv_size): when h
// is a bit-vector numeral its value is copied into out (a rational of
// NumeralService().Manager) and its width is returned.
func (m *ASTManager) BitVectorNumeralRational(h ASTHandle, out *unumeral.Rational) (width uint32, matched bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return 0, false, e
	}
	matched, err = m.isBitVectorNumeral(r)
	if err != nil || !matched {
		return 0, false, err
	}
	info, e := m.funcInfo(m.declAt(m.appAt(r.slot).decl.slot).info)
	if e != nil {
		return 0, false, e
	}
	token, e := m.infoParameter(info, 0)
	if e != nil {
		return 0, false, e
	}
	parameter, e := m.parameter(token)
	if e != nil {
		return 0, false, e
	}
	if parameter.Kind != ASTParameterRational {
		return 0, false, ErrASTDomain
	}
	if e = m.numerals.Manager.Copy(out, &parameter.Rational.Raw); e != nil {
		return 0, false, e
	}
	token, e = m.infoParameter(info, 1)
	if e != nil {
		return 0, false, e
	}
	parameter, e = m.parameter(token)
	if e != nil {
		return 0, false, e
	}
	if parameter.Kind != ASTParameterInt || parameter.integer < 0 {
		return 0, false, ErrASTDomain
	}
	return uint32(parameter.integer), true, nil
}

// NewParameterFromRational is parameter(rational const&) for a rational of
// NumeralService().Manager.
func (m *ASTManager) NewParameterFromRational(r *unumeral.Rational) (result ASTParameter, resultErr error) {
	if r == nil {
		return ASTParameter{}, ErrASTDomain
	}
	service := m.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return ASTParameter{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	if err = service.Manager.Copy(&value.Raw, r); err != nil {
		return ASTParameter{}, err
	}
	return m.NewParameterRational(&value)
}
