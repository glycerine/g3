package arith

// Source: src/tactic/arith/fm_tactic.cpp:438-472,1029-1061.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (p *FMInputParser) arithmetic(term ast.ASTHandle, operation int32) (bool, error) {
	kind, err := p.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(term)
	if err != nil || family != p.Family {
		return false, err
	}
	op, err := p.manager.GetDeclKind(term)
	return op == operation, err
}

// monomial writes the coefficient into coefficient (a rational of the
// manager's NumeralService().Manager).
func (p *FMInputParser) monomial(term ast.ASTHandle, coefficient *unumeral.Rational) (ast.ASTHandle, bool, error) {
	q := p.manager.NumeralService().Manager
	base, yes, err := p.variableTerm(term)
	if err != nil || yes {
		if err == nil {
			err = q.SetInt32(coefficient, 1)
		}
		return base, yes, err
	}
	mul, err := p.arithmetic(term, 9)
	if err != nil || !mul {
		return ast.ASTHandle{}, false, err
	}
	n, err := p.manager.GetNumArgs(term)
	if err != nil || n != 2 {
		return ast.ASTHandle{}, false, err
	}
	coefficientTerm, err := p.manager.GetArg(term, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	numeral, err := p.manager.IsArithmeticNumeral(coefficientTerm)
	if err != nil || !numeral {
		return ast.ASTHandle{}, false, err
	}
	variable, err := p.manager.GetArg(term, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	base, yes, err = p.variableTerm(variable)
	if err != nil || !yes {
		return ast.ASTHandle{}, false, err
	}
	if _, _, err = p.manager.ArithmeticNumeralRational(coefficientTerm, coefficient); err != nil {
		return ast.ASTHandle{}, false, err
	}
	return base, true, nil
}

// polynomial is is_linear_mon over the sum: the coefficients (rationals of
// the manager's NumeralService().Manager) are owned by the caller, also
// when the result is not useful.
func (p *FMInputParser) polynomial(term ast.ASTHandle, intern bool) (_ []uint32, _ []unumeral.Rational, _ bool, resultErr error) {
	q := p.manager.NumeralService().Manager
	coefficient := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&coefficient)) }()
	terms := []ast.ASTHandle{term}
	if add, err := p.arithmetic(term, 6); err != nil {
		return nil, nil, false, err
	} else if add {
		n, err := p.manager.GetNumArgs(term)
		if err != nil {
			return nil, nil, false, err
		}
		terms = terms[:0]
		for i := uint32(0); i < n; i++ {
			h, err := p.manager.GetArg(term, i)
			if err != nil {
				return nil, nil, false, err
			}
			terms = append(terms, h)
		}
	}
	seen := map[ast.ASTHandle]bool{}
	xs, coefficients := make([]uint32, 0, len(terms)), make([]unumeral.Rational, 0, len(terms))
	allForbidden := true
	for _, monomial := range terms {
		base, yes, err := p.monomial(monomial, &coefficient)
		if err != nil || !yes || seen[base] {
			return nil, nil, false, errors.Join(err, fmRelease(q, coefficients, nil))
		}
		seen[base] = true
		integer, err := p.integerVariable(base)
		if err != nil {
			return nil, nil, false, errors.Join(err, fmRelease(q, coefficients, nil))
		}
		declaration, err := p.manager.GetDecl(base)
		if err != nil {
			return nil, nil, false, errors.Join(err, fmRelease(q, coefficients, nil))
		}
		if !p.forbidden[declaration] && (!p.RealOnly || !integer) {
			allForbidden = false
		}
		if intern {
			x, err := p.internVariable(base)
			if err != nil {
				return nil, nil, false, errors.Join(err, fmRelease(q, coefficients, nil))
			}
			stored := unumeral.NewRational()
			coefficients = append(coefficients, stored)
			xs = append(xs, x)
			if err = q.Copy(&coefficients[len(coefficients)-1], &coefficient); err != nil {
				return nil, nil, false, errors.Join(err, fmRelease(q, coefficients, nil))
			}
		}
	}
	return xs, coefficients, !allForbidden, nil
}
