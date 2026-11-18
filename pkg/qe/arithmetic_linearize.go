package qe

// Source: src/qe/qe_arith.cpp imp::linearize/extract_coefficients.

import (
	"errors"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// linearizeTerm is imp::linearize(mbo, model, mul, t, c, fmls, ts, tids):
// multiplier and constant are rationals of the manager's
// NumeralService().Manager.
func (c *arithmeticProjectionCall) linearizeTerm(multiplier *unumeral.Rational, h ast.ASTHandle, constant *unumeral.Rational, forms *[]ast.ASTRef, terms *projectionTermMap) (err error) {
	p := c.owner
	m := p.manager
	q := p.q()
	// rational mul1;
	mul1, product := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&mul1), q.Del(&product)) }()
	// expr_ref val(m): assigned by the ite and mod cases, it dies when this
	// call returns.
	var val ast.ASTRef
	defer func() {
		if val != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&val))
		}
	}()
	args, mul, err := p.operation(h, p.family, 9)
	if err != nil {
		return err
	}
	if mul && len(args) == 2 {
		for i, arg := range args {
			ok, err := p.numeralExpression(arg, &mul1)
			if err != nil {
				return err
			}
			if ok {
				if err = q.Mul(multiplier, &mul1, &product); err != nil {
					return err
				}
				return c.linearizeTerm(&product, args[1-i], constant, forms, terms)
			}
		}
	}
	args, add, err := p.operation(h, p.family, 6)
	if err != nil {
		return err
	}
	if add {
		for _, arg := range args {
			if err = c.linearizeTerm(multiplier, arg, constant, forms, terms); err != nil {
				return err
			}
		}
		return nil
	}
	args, sub, err := p.operation(h, p.family, 7)
	if err != nil {
		return err
	}
	if sub && len(args) == 2 {
		if err = c.linearizeTerm(multiplier, args[0], constant, forms, terms); err != nil {
			return err
		}
		if err = errors.Join(q.Copy(&product, multiplier), q.Neg(&product)); err != nil {
			return err
		}
		return c.linearizeTerm(&product, args[1], constant, forms, terms)
	}
	args, neg, err := p.operation(h, p.family, 8)
	if err != nil {
		return err
	}
	if neg && len(args) == 1 {
		if err = errors.Join(q.Copy(&product, multiplier), q.Neg(&product)); err != nil {
			return err
		}
		return c.linearizeTerm(&product, args[0], constant, forms, terms)
	}
	ok, err := p.numeral(h, &mul1)
	if err != nil {
		return err
	}
	if ok {
		// c += mul*mul1;
		if err = q.Mul(multiplier, &mul1, &product); err != nil {
			return err
		}
		return q.Add(constant, &product, constant)
	}
	args, ite, err := p.operation(h, m.BasicFamilyID(), ast.ASTOpIte)
	if err != nil {
		return err
	}
	if ite && len(args) == 3 {
		var value ast.ASTHandle
		val, value, err = c.evalLocal(args[0])
		if err != nil {
			return err
		}
		truth := m.IsTrue(value)
		if !truth && !m.IsFalse(value) {
			return ast.ErrASTDomain
		}
		if err = p.emit("linearize", 188, func(w io.Writer) error {
			if e := writeExpression(m, p.config, w, args[0]); e != nil {
				return e
			}
			_, _ = io.WriteString(w, " := ")
			if e := writeExpression(m, p.config, w, value); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		}); err != nil {
			return err
		}
		if truth {
			if err = c.linearizeTerm(multiplier, args[1], constant, forms, terms); err != nil {
				return err
			}
			return p.appendRef(forms, args[0])
		}
		not, err := c.hold(m.MkNotSimplified(args[0]))
		if err != nil {
			return err
		}
		if err = p.appendRef(forms, not); err != nil {
			return err
		}
		return c.linearizeTerm(multiplier, args[2], constant, forms, terms)
	}
	args, mod, err := p.operation(h, p.family, 13)
	if err != nil {
		return err
	}
	if mod && len(args) == 2 {
		ok, err := p.numeralExpression(args[1], &mul1)
		if err != nil {
			return err
		}
		nonzero := false
		if ok {
			zero, err := q.IsZero(&mul1)
			if err != nil {
				return err
			}
			nonzero = !zero
		}
		if nonzero {
			return c.linearizeMod(multiplier, h, args[0], &mul1, constant, forms, &val)
		}
	}
	return c.insertMul(h, multiplier, terms)
}

// linearizeMod is linearize's (mod t1 mul1) case: val = eval(t) is the
// caller's expr_ref.
func (c *arithmeticProjectionCall) linearizeMod(multiplier *unumeral.Rational, h, t1 ast.ASTHandle, mul1, constant *unumeral.Rational, forms *[]ast.ASTRef, val *ast.ASTRef) (err error) {
	p := c.owner
	m := p.manager
	q := p.q()
	// rational r; ... rational c0(-r), mul0(1); obj_map<expr, rational> ts0;
	r, product, c0, mul0 := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	nested := projectionTerms(m)
	defer func() {
		err = errors.Join(err, nested.release(), q.Del(&mul0), q.Del(&c0), q.Del(&product), q.Del(&r))
	}()
	var value ast.ASTHandle
	*val, value, err = c.evalLocal(h)
	if err != nil {
		return err
	}
	ok, err := p.numeral(value, &r)
	if err != nil {
		return err
	}
	if !ok {
		return ast.ErrASTDomain
	}
	// c += mul*r;
	if err = q.Mul(multiplier, &r, &product); err != nil {
		return err
	}
	if err = q.Add(constant, &product, constant); err != nil {
		return err
	}
	if err = errors.Join(q.Copy(&c0, &r), q.Neg(&c0), q.SetInt32(&mul0, 1)); err != nil {
		return err
	}
	if err = c.linearizeTerm(&mul0, t1, &c0, forms, nested); err != nil {
		return err
	}
	coeffs, err := c.coefficients(nested)
	if err != nil {
		return err
	}
	return mboAddDivides(c.mbo, coeffs, q, &c0, mul1)
}

// insertMul is imp::insert_mul(x, v, ts).
func (c *arithmeticProjectionCall) insertMul(h ast.ASTHandle, multiplier *unumeral.Rational, terms *projectionTermMap) (err error) {
	p := c.owner
	m := p.manager
	q := p.q()
	if err = p.emit("insert_mul", 42, func(w io.Writer) error {
		text, e := q.String(multiplier)
		if e != nil {
			return e
		}
		_, _ = io.WriteString(w, "Adding variable ")
		if e := writeExpression(m, p.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " "+text+"\n")
		return nil
	}); err != nil {
		return err
	}
	// rational w;
	w := unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&w)) }()
	old, ok, err := terms.get(h)
	if err != nil {
		return err
	}
	if ok {
		if err = q.Add(old, multiplier, &w); err != nil {
			return err
		}
		return terms.set(h, &w)
	}
	return terms.set(h, multiplier)
}

func (c *arithmeticProjectionCall) linearizeLiteral(lit ast.ASTHandle, forms *[]ast.ASTRef) (_ bool, err error) {
	p := c.owner
	m := p.manager
	// The block-scoped `expr_ref val1 = eval(e1), val2 = eval(e2)` of the
	// negated equality die, val2 first, after its two linearize calls.
	var locals []ast.ASTRef
	release := func() error {
		var e error
		for i := len(locals) - 1; i >= 0; i-- {
			e = errors.Join(e, m.DelRef(&locals[i]))
		}
		locals = nil
		return e
	}
	defer func() { err = errors.Join(err, release()) }()
	if ast.ASTSourceDebug {
		// DEBUG_CODE(eval(lit, val); SASSERT(m.is_true(val))), isolated from
		// the shared evaluator so release builds see the same cache and IDs.
		valid := true
		if err := c.debugEval([]ast.ASTHandle{lit}, func(h, value ast.ASTHandle) (bool, error) {
			if m.IsTrue(value) {
				return true, nil
			}
			valid = false
			return false, p.emit("linearize", 65, func(w io.Writer) error {
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " := ")
				if e := writeExpression(m, p.config, w, value); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			})
		}); err != nil {
			return false, err
		}
		if !valid {
			return false, ast.ErrASTDomain
		}
	}
	negative, inner, err := m.IsNot(lit)
	if err != nil {
		return false, err
	}
	if negative {
		lit = inner
	}
	if ast.ASTSourceDebug {
		double, _, e := m.IsNot(lit)
		if e != nil {
			return false, e
		}
		if double {
			return false, ast.ErrASTDomain
		}
	}
	// rational mul(1), c(0); obj_map<expr, rational> ts;
	q := p.q()
	mulValue, constantValue, negMul := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	terms := projectionTerms(m)
	defer func() {
		err = errors.Join(err, terms.release(), q.Del(&negMul), q.Del(&constantValue), q.Del(&mulValue))
	}()
	mul, constant := &mulValue, &constantValue
	if err = q.SetInt32(mul, 1); err != nil {
		return false, err
	}
	if negative {
		if err = q.Neg(mul); err != nil {
			return false, err
		}
	}
	relation := simplex.LessEqual
	var left, right ast.ASTHandle
	matched := false
	for _, op := range []int32{2, 3, 4, 5} {
		args, ok, err := p.operation(lit, p.family, op)
		if err != nil {
			return false, err
		}
		if !ok || len(args) != 2 {
			continue
		}
		left, right = args[0], args[1]
		if op == 3 || op == 5 {
			left, right = right, left
		}
		if (op == 2 || op == 3) == negative {
			relation = simplex.Less
		}
		matched = true
		break
	}
	if !matched {
		args, eq, err := p.operation(lit, m.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return false, err
		}
		if eq && len(args) == 2 {
			_, arith, err := p.arithmetic(args[0])
			if err != nil {
				return false, err
			}
			if arith {
				left, right = args[0], args[1]
				relation = simplex.Equal
				matched = true
				if negative {
					lr, lv, err := c.evalLocal(left)
					if err != nil {
						return false, err
					}
					locals = append(locals, lr)
					rr, rv, err := c.evalLocal(right)
					if err != nil {
						return false, err
					}
					locals = append(locals, rr)
					// rational r1, r2;
					l, r := unumeral.NewRational(), unumeral.NewRational()
					ok, err := p.numeral(lv, &l)
					if err == nil && ok {
						ok, err = p.numeral(rv, &r)
					}
					cmp := 0
					if err == nil && ok {
						cmp, err = q.Compare(&l, &r)
					}
					if err = errors.Join(err, q.Del(&l), q.Del(&r)); err != nil || !ok {
						return false, err
					}
					if ast.ASTSourceDebug && cmp == 0 {
						return false, ast.ErrASTDomain
					}
					if cmp < 0 {
						left, right = right, left
					}
					relation = simplex.Less
				}
			}
		}
	}
	if !matched {
		args, distinct, err := p.operation(lit, m.BasicFamilyID(), ast.ASTOpDistinct)
		if err != nil {
			return false, err
		}
		if distinct && len(args) > 0 {
			_, arith, err := p.arithmetic(args[0])
			if err != nil {
				return false, err
			}
			if arith {
				if !negative {
					type entry struct {
						h     ast.ASTHandle
						value unumeral.Rational
					}
					values := make([]*entry, 0, len(args))
					defer func() {
						for _, v := range values {
							err = errors.Join(err, q.Del(&v.value))
						}
					}()
					for _, arg := range args {
						// val = eval(arg): the new value replaces the old.
						r, v, e := c.evalLocal(arg)
						if e != nil {
							return false, e
						}
						if e = release(); e != nil {
							return false, errors.Join(e, m.DelRef(&r))
						}
						locals = append(locals, r)
						value := &entry{h: arg, value: unumeral.NewRational()}
						values = append(values, value)
						ok, e := p.numeral(v, &value.value)
						if e != nil || !ok {
							return false, e
						}
					}
					var cmpErr error
					sort.Slice(values, func(i, j int) bool {
						c, e := q.Compare(&values[i].value, &values[j].value)
						cmpErr = errors.Join(cmpErr, e)
						return c < 0
					})
					if cmpErr != nil {
						return false, cmpErr
					}
					for i := 0; i+1 < len(values); i++ {
						if ast.ASTSourceDebug {
							c, e := q.Compare(&values[i].value, &values[i+1].value)
							if e != nil {
								return false, e
							}
							if c >= 0 {
								return false, ast.ErrASTDomain
							}
						}
						// expr_ref fml(a.mk_lt(...)) dies after each iteration.
						h, e := m.MkFamilyApp(p.family, 4, nil, []ast.ASTHandle{values[i].h, values[i+1].h})
						if e != nil {
							return false, e
						}
						fml, e := m.NewRef(h)
						if e != nil {
							return false, e
						}
						ok, e := c.linearizeLiteral(h, forms)
						if e = errors.Join(e, m.DelRef(&fml)); e != nil || !ok {
							return false, e
						}
					}
					return true, nil
				}
				values := make(map[string]ast.ASTHandle)
				for _, arg := range args {
					// expr_ref val = eval(arg1) dies at the end of each
					// iteration; the matching one after its linearize calls.
					if e := release(); e != nil {
						return false, e
					}
					r, v, e := c.evalLocal(arg)
					if e != nil {
						return false, e
					}
					locals = append(locals, r)
					num := unumeral.NewRational()
					ok, e := p.numeral(v, &num)
					var key string
					if e == nil && ok {
						key, e = q.String(&num)
					}
					if e = errors.Join(e, q.Del(&num)); e != nil || !ok {
						return false, e
					}
					if other, ok := values[key]; ok {
						left, right = arg, other
						matched = true
						relation = simplex.Equal
						break
					}
					values[key] = arg
				}
				if !matched {
					if ast.ASTSourceDebug {
						return false, ast.ErrASTDomain
					}
					coeffs, e := c.coefficients(terms)
					if e != nil {
						return false, e
					}
					return true, mboAddConstraint(c.mbo, coeffs, q, constant, simplex.Equal)
				}
			}
		}
	}
	if !matched {
		err := p.emit("linearize", 146, func(w io.Writer) error {
			_, _ = io.WriteString(w, "Skipping ")
			if e := writeExpression(m, p.config, w, lit); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
		return false, err
	}
	if err = c.linearizeTerm(mul, left, constant, forms, terms); err != nil {
		return false, err
	}
	if err = errors.Join(q.Copy(&negMul, mul), q.Neg(&negMul)); err != nil {
		return false, err
	}
	if err = c.linearizeTerm(&negMul, right, constant, forms, terms); err != nil {
		return false, err
	}
	if err = release(); err != nil {
		return false, err
	}
	coeffs, err := c.coefficients(terms)
	if err != nil {
		return false, err
	}
	return true, mboAddConstraint(c.mbo, coeffs, q, constant, relation)
}
