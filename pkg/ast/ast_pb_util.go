package ast

// Source: src/ast/pb_decl_plugin.cpp:103-181,200-217.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// pseudoBooleanIntegerParameter is parameter(rational) for an integral
// rational of m.numerals.Manager; int32 values use an int parameter.
func (m *ASTManager) pseudoBooleanIntegerParameter(value *unumeral.Rational) (ASTParameter, error) {
	q := m.numerals.Manager
	if small, e := q.IsInt32(value); e != nil {
		return ASTParameter{}, e
	} else if small {
		n, e := q.Int32(value)
		if e != nil {
			return ASTParameter{}, e
		}
		return m.newParameterInt(n)
	}
	r, e := m.numerals.NewValue()
	if e != nil {
		return ASTParameter{}, e
	}
	if e = q.Copy(&r.Raw, value); e != nil {
		_ = m.numerals.Del(&r)
		return ASTParameter{}, e
	}
	p, e := m.newParameterRational(&r)
	return p, errors.Join(e, m.numerals.Del(&r))
}

func (m *ASTManager) pseudoBooleanBuild(kind int32, coefficients []unumeral.Rational, bound *unumeral.Rational, args []ASTHandle) (result ASTHandle, err error) {
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil || len(coefficients) != len(args) {
		return result, ErrASTDomain
	}
	parameters := make([]ASTParameter, 0, len(args)+1)
	defer func() {
		for _, value := range parameters {
			err = errors.Join(err, m.dropParameter(value))
		}
	}()
	parameter, e := m.pseudoBooleanIntegerParameter(bound)
	if e != nil {
		return result, e
	}
	parameters = append(parameters, parameter)
	for i := range coefficients {
		parameter, e = m.pseudoBooleanIntegerParameter(&coefficients[i])
		if e != nil {
			return result, e
		}
		parameters = append(parameters, parameter)
	}
	r, e := m.mkFamilyApp(p.family, kind, parameters, args)
	return m.external(r), e
}

// pbRelease destroys normalize's m_coeffs and m_k.
func (m *ASTManager) pbRelease(coefficients []unumeral.Rational, k *unumeral.Rational) (err error) {
	q := m.numerals.Manager
	for i := range coefficients {
		err = errors.Join(err, q.Del(&coefficients[i]))
	}
	return errors.Join(err, q.Del(k))
}

// pseudoBooleanNormalize is pb_util::normalize: m_coeffs and m_k scaled by
// the lcm of the coefficients' denominators. The caller owns the results
// (pbRelease).
func (m *ASTManager) pseudoBooleanNormalize(coefficients []unumeral.OwnedRational, bound *unumeral.OwnedRational) (_ []unumeral.Rational, k unumeral.Rational, err error) {
	if bound == nil {
		return nil, k, ErrASTHandle
	}
	q := m.numerals.Manager
	values := make([]unumeral.Rational, len(coefficients))
	for i := range values {
		values[i] = unumeral.NewRational()
	}
	k = unumeral.NewRational()
	d, den := unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		err = errors.Join(err, q.Del(&d), q.Del(&den))
		if err != nil {
			err = errors.Join(err, m.pbRelease(values, &k))
		}
	}()
	allOnes := true
	for i := 0; i < len(coefficients) && allOnes; i++ {
		allOnes = coefficients[i].Raw.IsInt()
	}
	for i := range coefficients {
		if err = q.Copy(&values[i], &coefficients[i].Raw); err != nil {
			return nil, k, err
		}
	}
	if err = q.Copy(&k, &bound.Raw); err != nil {
		return nil, k, err
	}
	if allOnes {
		return values, k, nil
	}
	if err = q.SetInt32(&d, 1); err != nil {
		return nil, k, err
	}
	for i := range values {
		if err = errors.Join(q.DenominatorRational(&values[i], &den), q.LCM(&d, &den, &d)); err != nil {
			return nil, k, err
		}
	}
	for i := range values {
		if err = q.Mul(&d, &values[i], &values[i]); err != nil {
			return nil, k, err
		}
	}
	if err = q.Mul(&d, &k, &k); err != nil {
		return nil, k, err
	}
	return values, k, nil
}

func (m *ASTManager) pseudoBooleanRelation(kind int32, coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (_ ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	if len(coefficients) != len(args) {
		return ASTHandle{}, ErrASTDomain
	}
	values, k, e := m.pseudoBooleanNormalize(coefficients, bound)
	if e != nil {
		return ASTHandle{}, e
	}
	defer func() { err = errors.Join(err, m.pbRelease(values, &k)) }()
	q := m.numerals.Manager
	switch kind {
	case ASTPBOpLE:
		e = q.Floor(&k, &k)
	case ASTPBOpGE:
		e = q.Ceil(&k, &k)
	case ASTPBOpEQ:
		if !k.IsInt() {
			r, e := m.mkApp(m.basic.declarations[ASTOpFalse], nil)
			return m.external(r), e
		}
	default:
		return ASTHandle{}, ErrASTDomain
	}
	if e != nil {
		return ASTHandle{}, e
	}
	return m.pseudoBooleanBuild(kind, values, &k, args)
}

func (m *ASTManager) MkPseudoBooleanLE(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	return m.pseudoBooleanRelation(ASTPBOpLE, coefficients, args, bound)
}
func (m *ASTManager) MkPseudoBooleanGE(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	return m.pseudoBooleanRelation(ASTPBOpGE, coefficients, args, bound)
}
func (m *ASTManager) MkPseudoBooleanEQ(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (ASTHandle, error) {
	return m.pseudoBooleanRelation(ASTPBOpEQ, coefficients, args, bound)
}

func (m *ASTManager) pseudoBooleanCardinality(kind int32, args []ASTHandle, k uint32) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	p := m.auxiliaryPlugin(astPseudoBooleanPlugin)
	if p == nil {
		return result, ErrASTHandle
	}
	parameter, e := m.newParameterInt(int32(k))
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(parameter)) }()
	r, e := m.mkFamilyApp(p.family, kind, []ASTParameter{parameter}, args)
	return m.external(r), e
}

func (m *ASTManager) MkPseudoBooleanAtMost(args []ASTHandle, k uint32) (ASTHandle, error) {
	return m.pseudoBooleanCardinality(ASTPBOpAtMost, args, k)
}
func (m *ASTManager) MkPseudoBooleanAtLeast(args []ASTHandle, k uint32) (ASTHandle, error) {
	return m.pseudoBooleanCardinality(ASTPBOpAtLeast, args, k)
}

// MkPseudoBooleanLT preserves the source complement transformation:
// a*x < k becomes a*(not x) >= -floor(k) + sum(a) + 1 after normalization.
func (m *ASTManager) MkPseudoBooleanLT(coefficients []unumeral.OwnedRational, args []ASTHandle, bound *unumeral.OwnedRational) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	if len(coefficients) != len(args) {
		return result, ErrASTDomain
	}
	values, k, e := m.pseudoBooleanNormalize(coefficients, bound)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.pbRelease(values, &k)) }()
	// m_k = floor(m_k); m_k.neg(); m_k += 1; m_k += m_coeffs[i]
	q := m.numerals.Manager
	if e = errors.Join(q.Floor(&k, &k), q.Neg(&k), q.Inc(&k)); e != nil {
		return result, e
	}
	negated := make([]ASTHandle, len(args))
	for i, arg := range args {
		if e = q.Add(&k, &values[i], &k); e != nil {
			return result, e
		}
		n, e := m.mkFamilyApp(0, ASTOpNot, nil, []ASTHandle{arg})
		if e != nil {
			return result, e
		}
		negated[i] = m.external(n)
	}
	return m.pseudoBooleanBuild(ASTPBOpGE, values, &k, negated)
}
