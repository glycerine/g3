package smtcore

// Source: src/smt/theory_pb.cpp:961-970,1029-1045,1146,1838-1915.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtPBRationalSum is a debug-validation numeral sum with sticky failures.
type smtPBRationalSum struct {
	q   *numeral.RationalManager
	err error
}

func (v *smtPBRationalSum) add(a, b, out *numeral.Rational) {
	if v.err == nil {
		v.err = v.q.Add(a, b, out)
	}
}
func (v *smtPBRationalSum) cmp(a, b *numeral.Rational) int {
	if v.err != nil {
		return 0
	}
	c, err := v.q.Compare(a, b)
	v.err = err
	return c
}
func (v *smtPBRationalSum) zadd(a, b, out *numeral.Int) {
	if v.err == nil {
		v.err = v.q.Integers().Add(a, b, out)
	}
}
func (v *smtPBRationalSum) zcmp(a, b *numeral.Int) int {
	if v.err != nil {
		return 0
	}
	c, err := v.q.Integers().Compare(a, b)
	v.err = err
	return c
}

func (a *smtPBArgs) validateWellFormed(q *numeral.RationalManager) (err error) {
	if !smtcoreSourceDebug {
		return nil
	}
	if a.bound.Sign() <= 0 {
		return ast.ErrASTDomain
	}
	seen := make(map[int32]bool)
	v := smtPBRationalSum{q: q}
	sum, one := smtQZero(), smtQInt(1)
	defer func() { err = smtErrJoin(err, smtQDel(q, &sum)) }()
	for i := range a.terms {
		term := &a.terms[i]
		if v.cmp(&term.coeff, &a.bound) > 0 || v.cmp(&term.coeff, &one) < 0 || term.literal == SMTTrueLiteral || term.literal == SMTFalseLiteral || term.literal == SMTNullLiteral || seen[term.literal.Var()] {
			return smtErrJoin(v.err, ast.ErrASTDomain)
		}
		seen[term.literal.Var()] = true
		v.add(&sum, &term.coeff, &sum)
	}
	if v.cmp(&sum, &a.bound) < 0 {
		return smtErrJoin(v.err, ast.ErrASTDomain)
	}
	return v.err
}

func (t *SMTPseudoBooleanTheory) validateWatch(c *smtPBConstraint) error {
	if !smtcoreSourceDebug {
		return nil
	}
	a := c.current()
	if c.watchSize < 0 || c.watchSize > len(a.terms) {
		return ast.ErrASTDomain
	}
	// scoped_mpz sum, max
	v := smtPBRationalSum{q: t.q()}
	z := v.q.Integers()
	var sum, maximum numeral.Int
	defer func() { t.keep(smtErrJoin(z.Del(&sum), z.Del(&maximum))) }()
	for i := range a.terms[:c.watchSize] {
		n := &a.terms[i].coeff.Num
		v.zadd(&sum, n, &sum)
		if v.zcmp(n, &maximum) > 0 && v.err == nil {
			v.err = z.Copy(&maximum, n)
		}
	}
	if v.zcmp(&c.watchSum, &sum) != 0 || v.zcmp(&sum, &a.bound.Num) < 0 || v.zcmp(&c.maxWatch, &maximum) != 0 {
		return smtErrJoin(v.err, ast.ErrASTDomain)
	}
	return v.err
}

func (t *SMTPseudoBooleanTheory) validateAssign(c *smtPBConstraint, premises []SMTLiteral, consequent SMTLiteral) error {
	if !smtcoreSourceDebug {
		return nil
	}
	blocked := make(map[SMTLiteral]bool)
	for _, l := range premises {
		value, err := t.state.Assignment(l)
		if err != nil {
			return err
		}
		if value != lbool.True {
			return ast.ErrASTDomain
		}
		blocked[l.Negated()] = true
	}
	value, err := t.state.Assignment(consequent)
	if err != nil {
		return err
	}
	if value != lbool.Undef {
		return ast.ErrASTDomain
	}
	value, err = t.state.Assignment(c.lit())
	if err != nil {
		return err
	}
	if value != lbool.True {
		return ast.ErrASTDomain
	}
	blocked[consequent] = true
	// numeral sum
	v := smtPBRationalSum{q: t.q()}
	sum := smtQZero()
	defer func() { t.keep(smtQDel(v.q, &sum)) }()
	for i := range c.current().terms {
		term := &c.current().terms[i]
		if !blocked[term.literal] {
			v.add(&sum, &term.coeff, &sum)
		}
	}
	cmp := v.cmp(&sum, &c.current().bound)
	if v.err != nil {
		return v.err
	}
	if cmp >= 0 {
		if err = t.emitPB("validate_assign", 1879, func(out io.Writer) error {
			_, _ = io.WriteString(out, "invalid assign")
			if err := t.displayInequality(out, c, true); err != nil {
				return err
			}
			if err := smtPBWriteLiterals(out, premises, true); err != nil {
				return err
			}
			_, _ = io.WriteString(out, " => ")
			if err := smtPBWriteLiteral(out, consequent); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) validateVariableSums(c *smtPBConstraint) error {
	if !smtcoreSourceDebug {
		return nil
	}
	// scoped_mpz sum, maxs
	v := smtPBRationalSum{q: t.q()}
	z := v.q.Integers()
	var sum, maximum, zero numeral.Int
	defer func() { t.keep(smtErrJoin(z.Del(&sum), z.Del(&maximum))) }()
	for i := range c.current().terms {
		term := &c.current().terms[i]
		value, err := t.state.Assignment(term.literal)
		if err != nil {
			return err
		}
		if value == lbool.True {
			v.zadd(&sum, &term.coeff.Num, &sum)
		}
		if value != lbool.False {
			v.zadd(&maximum, &term.coeff.Num, &maximum)
		}
	}
	if v.zcmp(&maximum, &c.maxSum) > 0 {
		if err := t.traceConstraint("assign_watch", 1036, c, true); err != nil {
			return err
		}
	}
	if v.zcmp(&c.minSum, &sum) > 0 || v.zcmp(&sum, &maximum) > 0 || v.zcmp(&maximum, &c.maxSum) > 0 || v.zcmp(&c.minSum, &c.maxSum) > 0 || v.zcmp(&c.minSum, &zero) < 0 {
		return smtErrJoin(v.err, ast.ErrASTDomain)
	}
	return v.err
}

func (t *SMTPseudoBooleanTheory) validateFinalConstraint(c *smtPBConstraint) error {
	if !smtcoreSourceDebug {
		return nil
	}
	value, err := t.state.Assignment(c.lit())
	if err != nil {
		return err
	}
	if value == lbool.Undef {
		return nil
	}
	expression, err := t.state.BoolExpression(c.lit().Var())
	if err != nil {
		return err
	}
	if !t.state.isRelevant(expression) {
		return nil
	}
	vs := smtPBRationalSum{q: t.q()}
	sum, maximum := smtQZero(), smtQZero()
	defer func() { t.keep(smtErrJoin(smtQDel(vs.q, &sum), smtQDel(vs.q, &maximum))) }()
	for i := range c.current().terms {
		term := &c.current().terms[i]
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return err
		}
		if v == lbool.True {
			vs.add(&sum, &term.coeff, &sum)
		}
		if v != lbool.False {
			vs.add(&maximum, &term.coeff, &maximum)
		}
	}
	if vs.err != nil {
		return vs.err
	}
	k := &c.current().bound
	if err = t.emitPB("validate_final_check", 1918, func(out io.Writer) error {
		_, _ = io.WriteString(out, "validate: ")
		if err := t.displayInequality(out, c, true); err != nil {
			return err
		}
		sumText, err := vs.q.String(&sum)
		if err != nil {
			return err
		}
		maxText, err := vs.q.String(&maximum)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "sum: %s %s %s\n", sumText, maxText, value.String())
		return err
	}); err != nil {
		return err
	}
	if vs.cmp(&sum, &maximum) > 0 {
		return smtErrJoin(vs.err, ast.ErrASTDomain)
	}
	if !c.equality {
		if (vs.cmp(&sum, k) >= 0) != (value == lbool.True) || (vs.cmp(&maximum, k) < 0) != (value == lbool.False) {
			return smtErrJoin(vs.err, ast.ErrASTDomain)
		}
	} else if (vs.cmp(&sum, k) == 0) != (value == lbool.True) {
		return smtErrJoin(vs.err, ast.ErrASTDomain)
	}
	return vs.err
}
