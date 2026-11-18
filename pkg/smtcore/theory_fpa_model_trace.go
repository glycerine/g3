package smtcore

// Source: src/smt/theory_fpa.cpp:129-132,194-198,206-209,228.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *smtFPAValueProcedure) modelTraceTags(generator *SMTModelGenerator) *diagnostic.TraceTags {
	if generator != nil && generator.trace != nil {
		return generator.trace
	}
	return p.manager.BorrowSourceTraceTags()
}

func (p *smtFPAValueProcedure) emitModelTrace(generator *SMTModelGenerator, tag string, line int, body func(io.Writer) error) error {
	tags := p.modelTraceTags(generator)
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var operandErr error
	err := tags.EmitConditional(tag, "mk_value", "src/smt/theory_fpa.cpp", line, func() bool { return true }, func(out io.Writer) error {
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (p *smtFPAValueProcedure) displayModelTraceExpression(out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

func (p *smtFPAValueProcedure) traceInputs(generator *SMTModelGenerator, values []ast.ASTHandle) error {
	line := 129
	if p.rounding {
		line = 206
	}
	return p.emitModelTrace(generator, "t_fpa_detail", line, func(out io.Writer) error {
		for i, value := range values {
			if _, err := fmt.Fprintf(out, "value[%d] = ", i); err != nil {
				return err
			}
			if err := p.displayModelTraceExpression(out, value); err != nil {
				return err
			}
			if err := diagnostic.WriteTraceEndl(out); err != nil {
				return err
			}
		}
		return nil
	})
}

func (p *smtFPAValueProcedure) readModelBitVector(value ast.ASTHandle) (text string, width uint32, result error) {
	service := p.manager.NumeralService()
	number, err := service.NewValue()
	if err != nil {
		return "", 0, err
	}
	defer func() { result = errors.Join(result, service.Del(&number)) }()
	found, err := p.manager.BitVectorNumeralValue(value, &number, &width)
	if err != nil {
		return "", 0, err
	}
	if !found {
		return "", 0, ast.ErrASTDomain
	}
	text, result = service.String(&number)
	return text, width, result
}

func (p *smtFPAValueProcedure) modelComponents(values []ast.ASTHandle) (_ [3]string, result error) {
	var components [3]string
	if len(values) == 3 {
		for i, value := range values {
			text, _, err := p.readModelBitVector(value)
			if err != nil {
				return components, err
			}
			components[i] = text
		}
		return components, nil
	}
	if len(values) != 1 || p.ebits < 2 || p.sbits < 2 {
		return components, ast.ErrASTDomain
	}
	// rational all_r; scoped_mpz all_z, sgn_z, exp_z (the source's mpz
	// manager split: machine_div2k and mod by m_powers2).
	q := p.manager.NumeralService().Manager
	z := q.Integers()
	allR := smtQZero()
	var allZ, sgnZ, expZ, power numeral.Int
	defer func() {
		result = smtErrJoin(result, smtQDel(q, &allR), z.Del(&allZ), z.Del(&sgnZ), z.Del(&expZ), z.Del(&power))
	}()
	width, found, err := p.manager.BitVectorNumeralRational(values[0], &allR)
	if err != nil {
		return components, err
	}
	if !found || width != p.ebits+p.sbits || !allR.IsInt() || allR.IsNeg() {
		return components, ast.ErrASTDomain
	}
	two := numeral.NewInt(2)
	steps := []func() error{
		func() error { return q.Numerator(&allR, &allZ) },
		func() error { return z.MachineDiv2k(&allZ, width-1, &sgnZ) },
		func() error { return z.Power(&two, width-1, &power) },
		func() error { return z.Mod(&allZ, &power, &allZ) },
		func() error { return z.MachineDiv2k(&allZ, p.sbits-1, &expZ) },
		func() error { return z.Power(&two, p.sbits-1, &power) },
		func() error { return z.Mod(&allZ, &power, &allZ) },
	}
	for _, step := range steps {
		if err = step(); err != nil {
			return components, err
		}
	}
	for i, n := range []*numeral.Int{&sgnZ, &expZ, &allZ} {
		if components[i], err = z.String(n); err != nil {
			return components, err
		}
	}
	return components, nil
}

func (p *smtFPAValueProcedure) traceResult(generator *SMTModelGenerator, values []ast.ASTHandle, value ast.ASTHandle) error {
	line := 194
	if p.rounding {
		line = 228
	}
	return p.emitModelTrace(generator, "t_fpa", line, func(out io.Writer) error {
		if p.rounding {
			if _, err := io.WriteString(out, "result: "); err != nil {
				return err
			}
		} else {
			components, err := p.modelComponents(values)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "result: [%s,%s,%s] --> ", components[0], components[1], components[2]); err != nil {
				return err
			}
		}
		if err := p.displayModelTraceExpression(out, value); err != nil {
			return err
		}
		return diagnostic.WriteTraceEndl(out)
	})
}
