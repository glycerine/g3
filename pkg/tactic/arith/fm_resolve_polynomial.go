package arith

// Source: src/tactic/arith/fm_tactic.cpp:1284-1358.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func fmCoefficient(c *FMConstraint, x uint32) (*unumeral.Rational, bool) {
	for i, v := range c.Variables {
		if v == x {
			return &c.Coefficients[i], true
		}
	}
	return nil, false
}

// fmResolvePolynomial is resolve's new_xs/new_as/new_c computation; the
// returned coefficients and constant are rationals of the manager's
// NumeralService().Manager owned by the caller (fmRelease).
func fmResolvePolynomial(variables *FMVariableTable, lower, upper *FMConstraint, x uint32) (xs []uint32, coefficients []unumeral.Rational, constant unumeral.Rational, strict bool, resultErr error) {
	constant = unumeral.NewRational()
	la, lok := fmCoefficient(lower, x)
	ub, uok := fmCoefficient(upper, x)
	if variables == nil || !lok || !uok || la.Sign() >= 0 || ub.Sign() <= 0 {
		return nil, nil, constant, false, ast.ErrASTDomain
	}
	q := lower.numerals
	// rational a = -la, b = ub;
	a, b, term := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		resultErr = errors.Join(resultErr, q.Del(&a), q.Del(&b), q.Del(&term))
		if resultErr != nil {
			resultErr = errors.Join(resultErr, fmRelease(q, coefficients, &constant))
			xs, coefficients = nil, nil
		}
	}()
	if err := errors.Join(q.Copy(&a, la), q.Neg(&a), q.Copy(&b, ub)); err != nil {
		return nil, nil, constant, false, err
	}
	if ast.ASTSourceDebug && variables.IsInteger(x) && !a.IsOne() && !b.IsOne() {
		return nil, nil, constant, false, ast.ErrASTDomain
	}
	// new_c = lower.c * b + upper.c * a
	if err := q.Mul(&lower.Constant, &b, &constant); err != nil {
		return nil, nil, constant, false, err
	}
	if err := q.Mul(&upper.Constant, &a, &term); err != nil {
		return nil, nil, constant, false, err
	}
	if err := q.Add(&constant, &term, &constant); err != nil {
		return nil, nil, constant, false, err
	}
	strict = lower.Strict || upper.Strict
	positions := make(map[uint32]int, len(lower.Variables)+len(upper.Variables))
	for i, v := range lower.Variables {
		if v == x {
			continue
		}
		positions[v] = len(xs)
		xs = append(xs, v)
		coefficients = append(coefficients, unumeral.NewRational())
		if err := q.Mul(&lower.Coefficients[i], &b, &coefficients[len(coefficients)-1]); err != nil {
			return nil, nil, constant, false, err
		}
	}
	for i, v := range upper.Variables {
		if v == x {
			continue
		}
		if err := q.Mul(&upper.Coefficients[i], &a, &term); err != nil {
			return nil, nil, constant, false, err
		}
		if p, found := positions[v]; found {
			if err := q.Add(&coefficients[p], &term, &coefficients[p]); err != nil {
				return nil, nil, constant, false, err
			}
		} else {
			positions[v] = len(xs)
			xs = append(xs, v)
			coefficients = append(coefficients, unumeral.NewRational())
			if err := q.Copy(&coefficients[len(coefficients)-1], &term); err != nil {
				return nil, nil, constant, false, err
			}
		}
	}
	write := 0
	allInteger := true
	for i := range xs {
		if coefficients[i].IsZero() {
			continue
		}
		if !variables.IsInteger(xs[i]) {
			allInteger = false
		}
		xs[write] = xs[i]
		if write != i {
			if err := q.Swap(&coefficients[write], &coefficients[i]); err != nil {
				return nil, nil, constant, false, err
			}
		}
		write++
	}
	if err := fmRelease(q, coefficients[write:], nil); err != nil {
		return nil, nil, constant, false, err
	}
	xs, coefficients = xs[:write], coefficients[:write]
	if allInteger && strict {
		strict = false
		if err := q.Dec(&constant); err != nil {
			return nil, nil, constant, false, err
		}
	}
	return xs, coefficients, constant, strict, nil
}
