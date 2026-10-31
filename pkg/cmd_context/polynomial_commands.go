package cmd_context

// Source: src/cmd_context/extra_cmds/polynomial_cmds.cpp, to_poly.
// Factorization and root-isolation commands are separate port steps.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

// WritePolynomial translates an arithmetic expression into the source
// sum-of-monomials display. The polynomial manager is scoped to this command.
func (c *Context) WritePolynomial(out io.Writer, expression ast.ASTHandle) (resultErr error) {
	if out == nil {
		return ast.ErrASTHandle
	}
	m, err := c.Manager()
	if err != nil {
		return err
	}
	services, err := c.ModelEvaluatorConfig()
	if err != nil {
		return err
	}
	q, err := m.NumeralService().NewPrivateRationalManager()
	if err != nil {
		if q != nil {
			err = errors.Join(err, q.CloseChecked())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, q.CloseChecked()) }()
	manager, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core: polynomial.MultivariatePolynomialConfig{
			Integer: q.Integers(), Rationals: q, Checkpoint: m.Limit.Inc,
			Cooperation: func() error { return services.Cooperation.Cooperate("polynomial") },
		},
	})
	if err != nil {
		if manager != nil {
			err = errors.Join(err, manager.CloseChecked())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.CloseChecked()) }()
	converter, err := ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: m, Polynomial: manager})
	if err != nil {
		if converter != nil {
			err = errors.Join(err, converter.Close())
		}
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, converter.Close()) }()
	value, denominator, matched, err := converter.ToPolynomial(expression)
	if err != nil {
		if errors.Is(err, ast.ErrExprNotPolynomial) {
			return fmt.Errorf("expression is not a polynomial")
		}
		return err
	}
	if !matched {
		return fmt.Errorf("expression is not a polynomial")
	}
	defer func() { resultErr = errors.Join(resultErr, value.Close(), denominator.Close()) }()
	p, err := value.Get()
	if err != nil {
		return err
	}
	translated, err := converter.ToExpr(p, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&translated)) }()
	result, err := m.RefGet(translated)
	if err != nil {
		return err
	}
	divisor, err := denominator.Display()
	if err != nil {
		return err
	}
	if divisor != "1" {
		if _, err = fmt.Fprintf(out, "(* %s ", divisor); err != nil {
			return err
		}
	}
	if err = c.WriteExpression(out, result); err != nil {
		return err
	}
	if divisor != "1" {
		if _, err = fmt.Fprint(out, ")"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out)
	return err
}
