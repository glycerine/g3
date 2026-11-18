package arith

// Source: src/tactic/arith/fm_tactic.cpp:474-510.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (p *FMInputParser) literal(term ast.ASTHandle) (ast.ASTHandle, bool, bool, error) {
	if yes, err := p.uninterpretedConstant(term); err != nil || yes {
		return term, false, yes, err
	}
	not, child, err := p.manager.IsNot(term)
	if err != nil || !not {
		return ast.ASTHandle{}, false, false, err
	}
	yes, err := p.uninterpretedConstant(child)
	return child, true, yes, err
}

// inequality is is_linear_ineq: the coefficients and constant are rationals
// of the manager's NumeralService().Manager owned by the caller (fmRelease;
// nothing is held when matched is false).
func (p *FMInputParser) inequality(term ast.ASTHandle, intern bool) (xs []uint32, coefficients []unumeral.Rational, constant unumeral.Rational, strict, matched bool, err error) {
	constant = unumeral.NewRational()
	not, child, err := p.manager.IsNot(term)
	if err != nil {
		return nil, nil, constant, false, false, err
	}
	if not {
		term = child
	}
	if !intern {
		if err = p.store.diagnostics.recognition(term); err != nil {
			return nil, nil, constant, false, false, err
		}
	}
	kind, err := p.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return nil, nil, constant, false, false, err
	}
	family, err := p.manager.GetFamilyID(term)
	if err != nil || family != p.Family {
		return nil, nil, constant, false, false, err
	}
	op, err := p.manager.GetDeclKind(term)
	if err != nil || (op != 2 && op != 3) {
		return nil, nil, constant, false, false, err
	}
	lhs, err := p.manager.GetArg(term, 0)
	if err != nil {
		return nil, nil, constant, false, false, err
	}
	rhs, err := p.manager.GetArg(term, 1)
	if err != nil {
		return nil, nil, constant, false, false, err
	}
	q := p.manager.NumeralService().Manager
	_, numeral, err := p.manager.ArithmeticNumeralRational(rhs, &constant)
	if err != nil || !numeral {
		return nil, nil, constant, false, false, errors.Join(err, q.Del(&constant))
	}
	xs, coefficients, useful, err := p.polynomial(lhs, intern)
	if err != nil || !useful {
		return nil, nil, constant, false, false, errors.Join(err, fmRelease(q, coefficients, &constant))
	}
	negative := not
	strict = not
	if op == 3 {
		negative = !negative
	}
	if negative {
		err = q.Neg(&constant)
		for i := range coefficients {
			err = errors.Join(err, q.Neg(&coefficients[i]))
		}
		if err != nil {
			return nil, nil, constant, false, false, errors.Join(err, fmRelease(q, coefficients, &constant))
		}
	}
	return xs, coefficients, constant, strict, true, nil
}
