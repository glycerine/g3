package interp

// Source checkpoint: src/interp/iz3mgr.cpp and src/interp/iz3mgr.h proof readers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// LemmaTheory identifies the theory recorded on a theory-lemma proof.
type LemmaTheory uint8

const (
	ArithTheory LemmaTheory = iota
	ArrayTheory
	UnknownTheory
)

// LemmaKind identifies the retained arithmetic theory-lemma variants.
type LemmaKind uint8

const (
	FarkasKind LemmaKind = iota
	Leq2EqKind
	Eq2LeqKind
	GCDTestKind
	AssignBoundsKind
	EqPropagateKind
	GomoryCutKind
	ArithMysteryKind
	UnknownKind
)

func (m *Manager) ProofRule(proof ast.ASTHandle) (int32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	family, err := m.ast.GetFamilyID(proof)
	if err != nil {
		return 0, err
	}
	if family != m.ast.BasicFamilyID() {
		return 0, ast.ErrASTDomain
	}
	return m.ast.GetDeclKind(proof)
}

func (m *Manager) NumPremises(proof ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	n, err := m.ast.GetNumArgs(proof)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, ast.ErrASTDomain
	}
	return n - 1, nil
}

func (m *Manager) Premise(proof ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	n, err := m.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= n {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.ast.GetArg(proof, index)
}

func (m *Manager) Conclusion(proof ast.ASTHandle) (ast.ASTHandle, error) {
	n, err := m.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.GetArg(proof, n)
}

func (m *Manager) proofParameters(proof ast.ASTHandle) (parameters []ast.ASTParameter, result error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	decl, err := m.ast.GetDecl(proof)
	if err != nil {
		return nil, err
	}
	parameters, err = m.ast.GetDeclParametersCopy(decl)
	if err != nil {
		return nil, err
	}
	return parameters, nil
}

func (m *Manager) releaseParameters(parameters []ast.ASTParameter) (result error) {
	for i := range parameters {
		result = errors.Join(result, m.ast.DelParameter(&parameters[i]))
	}
	return result
}

func (m *Manager) parameterSymbolText(parameter ast.ASTParameter) (string, error) {
	symbol, err := m.ast.ParameterSymbol(parameter)
	if err != nil {
		return "", err
	}
	text, present, err := m.ast.Symbols.BareString(symbol)
	if err != nil {
		return "", err
	}
	if present {
		return text, nil
	}
	return m.ast.Symbols.String(symbol)
}

func (m *Manager) TheoryLemmaTheory(proof ast.ASTHandle) (result LemmaTheory, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return UnknownTheory, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	if len(parameters) == 0 {
		return UnknownTheory, nil
	}
	name, err := m.parameterSymbolText(parameters[0])
	if err != nil {
		return UnknownTheory, nil
	}
	switch name {
	case "arith":
		return ArithTheory, nil
	case "array":
		return ArrayTheory, nil
	default:
		return UnknownTheory, nil
	}
}

func (m *Manager) TheoryLemmaKind(proof ast.ASTHandle) (result LemmaKind, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return UnknownKind, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	if len(parameters) < 2 {
		return ArithMysteryKind, nil
	}
	name, err := m.parameterSymbolText(parameters[1])
	if err != nil {
		return UnknownKind, nil
	}
	switch name {
	case "farkas":
		return FarkasKind, nil
	case "triangle-eq":
		conclusion, e := m.Conclusion(proof)
		if e != nil {
			return UnknownKind, e
		}
		first, e := m.Arg(conclusion, 0)
		if e != nil {
			return UnknownKind, e
		}
		if op, e := m.Operator(first); e != nil {
			return UnknownKind, e
		} else if op == OperatorNot {
			return Eq2LeqKind, nil
		}
		return Leq2EqKind, nil
	case "gcd-test":
		return GCDTestKind, nil
	case "assign-bounds":
		return AssignBoundsKind, nil
	case "eq-propagate":
		return EqPropagateKind, nil
	case "gomory-cut":
		return GomoryCutKind, nil
	default:
		return UnknownKind, nil
	}
}

// errBadFarkasCoefficient is iz3mgr's iz3_exception("Bad Farkas
// coefficient"); get_broken_gcd_test_coeffs and get_gomory_cut_coeffs
// throw the char* instead (badFarkasCoefficientFailure).
func errBadFarkasCoefficient() error { return iz3Exception("Bad Farkas coefficient") }

func badFarkasCoefficientFailure() error { return NewError("Bad Farkas coefficient") }

// rationalParameter copies a rational parameter into out (a rational of
// Rationals()).
func (m *Manager) rationalParameter(parameter ast.ASTParameter, out *unumeral.Rational, failure func() error) (err error) {
	service := m.ast.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, service.Del(&value)) }()
	matched, err := m.ast.ParameterRationalCopy(parameter, &value)
	if err != nil {
		return err
	}
	if !matched {
		return failure()
	}
	return service.Manager.Copy(out, &value.Raw)
}

// normalizeCoefficientDenominators multiplies values by the lcm of their
// denominators.
func (m *Manager) normalizeCoefficientDenominators(values []unumeral.Rational) (err error) {
	q := m.Rationals()
	lcm, d := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&lcm), q.Del(&d)) }()
	if err = q.SetInt32(&lcm, 1); err != nil {
		return err
	}
	for i := range values {
		if err = errors.Join(q.DenominatorRational(&values[i], &d), q.LCM(&lcm, &d, &lcm)); err != nil {
			return err
		}
	}
	for i := range values {
		if err = q.Mul(&values[i], &lcm, &values[i]); err != nil {
			return err
		}
	}
	return nil
}

// theoryLemmaCoefficients returns rationals the caller releases
// (releaseRationals).
func (m *Manager) theoryLemmaCoefficients(proof ast.ASTHandle, absolute bool, failure func() error) (result []unumeral.Rational, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, m.releaseParameters(parameters))
		if err != nil && result != nil {
			err = errors.Join(err, m.releaseRationals(result))
			result = nil
		}
	}()
	if len(parameters) < 2 {
		return nil, failure()
	}
	result = newRationals(len(parameters) - 2)
	for i := range result {
		if err = m.rationalParameter(parameters[i+2], &result[i], failure); err != nil {
			return result, err
		}
		if absolute && result[i].IsNeg() {
			if err = m.Rationals().Neg(&result[i]); err != nil {
				return result, err
			}
		}
	}
	return result, m.normalizeCoefficientDenominators(result)
}

func (m *Manager) FarkasCoefficients(proof ast.ASTHandle) ([]unumeral.Rational, error) {
	return m.theoryLemmaCoefficients(proof, true, errBadFarkasCoefficient)
}

func (m *Manager) BrokenGCDTestCoefficients(proof ast.ASTHandle) ([]unumeral.Rational, error) {
	return m.theoryLemmaCoefficients(proof, false, badFarkasCoefficientFailure)
}

func (m *Manager) GomoryCutCoefficients(proof ast.ASTHandle) ([]unumeral.Rational, error) {
	return m.theoryLemmaCoefficients(proof, true, badFarkasCoefficientFailure)
}

func (m *Manager) inequalityHasNegativeOrientation(inequality ast.ASTHandle) (bool, error) {
	isNot, body, err := m.ast.IsNot(inequality)
	if err != nil {
		return false, err
	}
	target := inequality
	if isNot {
		target = body
	}
	op, err := m.Operator(target)
	if err != nil {
		return false, err
	}
	if isNot {
		return op == OperatorLE || op == OperatorLT, nil
	}
	return op == OperatorGE || op == OperatorGT, nil
}

func (m *Manager) assignBoundsCoefficients(proof ast.ASTHandle, rule bool) (result []unumeral.Rational, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, m.releaseParameters(parameters))
		if err != nil && result != nil {
			err = errors.Join(err, m.releaseRationals(result))
			result = nil
		}
	}()
	if len(parameters) < 2 {
		return nil, errBadFarkasCoefficient()
	}
	conclusion, err := m.Conclusion(proof)
	if err != nil {
		return nil, err
	}
	consequence, err := m.Arg(conclusion, 0)
	if err != nil {
		return nil, err
	}
	consequenceNegative, err := m.inequalityHasNegativeOrientation(consequence)
	if err != nil {
		return nil, err
	}
	q := m.Rationals()
	result = newRationals(len(parameters) - 1)
	if err = q.SetInt32(&result[0], 1); err != nil {
		return result, err
	}
	for i := 2; i < len(parameters); i++ {
		value := &result[i-1]
		if e := m.rationalParameter(parameters[i], value, errBadFarkasCoefficient); e != nil {
			return result, e
		}
		var inequality ast.ASTHandle
		if rule {
			premise, e := m.Premise(proof, uint32(i-2))
			if e != nil {
				return result, e
			}
			inequality, e = m.Conclusion(premise)
			if e != nil {
				return result, e
			}
		} else {
			var e error
			inequality, e = m.Arg(conclusion, uint32(i-1))
			if e != nil {
				return result, e
			}
		}
		negative, e := m.inequalityHasNegativeOrientation(inequality)
		if e != nil {
			return result, e
		}
		if negative != consequenceNegative {
			if e = q.Neg(value); e != nil {
				return result, e
			}
		}
		if value.IsNeg() {
			if e = q.Neg(value); e != nil {
				return result, e
			}
		}
	}
	return result, m.normalizeCoefficientDenominators(result)
}

func (m *Manager) AssignBoundsCoefficients(proof ast.ASTHandle) ([]unumeral.Rational, error) {
	return m.assignBoundsCoefficients(proof, false)
}

func (m *Manager) AssignBoundsRuleCoefficients(proof ast.ASTHandle) ([]unumeral.Rational, error) {
	return m.assignBoundsCoefficients(proof, true)
}

// coefficientTerms makes the values' int numerals and releases the values.
func (m *Manager) coefficientTerms(values []unumeral.Rational) (result []ast.ASTHandle, err error) {
	defer func() { err = errors.Join(err, m.releaseRationals(values)) }()
	result = make([]ast.ASTHandle, len(values))
	for i := range values {
		term, err := m.MakeInt(&values[i])
		if err != nil {
			return nil, err
		}
		result[i] = term
	}
	return result, nil
}

func (m *Manager) FarkasCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.FarkasCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) AssignBoundsCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.AssignBoundsCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) AssignBoundsRuleCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.AssignBoundsRuleCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) GomoryCutCoefficientTerms(proof ast.ASTHandle) ([]ast.ASTHandle, error) {
	values, err := m.GomoryCutCoefficients(proof)
	if err != nil {
		return nil, err
	}
	return m.coefficientTerms(values)
}

func (m *Manager) IsFarkasCoefficientNegative(proof ast.ASTHandle, index uint32) (result bool, err error) {
	parameters, err := m.proofParameters(proof)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.releaseParameters(parameters)) }()
	parameterIndex := int(index) + 2
	if parameterIndex >= len(parameters) {
		return false, ast.ErrASTDomain
	}
	err = m.withRational(func(value *unumeral.Rational) error {
		if e := m.rationalParameter(parameters[parameterIndex], value, errBadFarkasCoefficient); e != nil {
			return e
		}
		result = value.IsNeg()
		return nil
	})
	return result, err
}
