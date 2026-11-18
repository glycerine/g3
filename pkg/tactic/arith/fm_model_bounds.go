package arith

// Source: src/tactic/arith/fm_tactic.cpp:57-156.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type fmBoundKind uint8

const (
	fmNoBound fmBoundKind = iota
	fmLowerBound
	fmUpperBound
)

// fmRat is m_util.is_numeral(h, out).
func fmRat(m *ast.ASTManager, h ast.ASTHandle, out *unumeral.Rational) (bool, error) {
	_, yes, e := m.ArithmeticNumeralRational(h, out)
	return yes && e == nil, e
}
func (c *FMModelConverter) evaluate(ev *model.ModelEvaluator, h ast.ASTHandle, ref ast.ASTRef) (ast.ASTHandle, error) {
	if e := ev.Evaluate(h, ref); e != nil {
		return ast.ASTHandle{}, e
	}
	return c.manager.RefGet(ref)
}

// processBound is fm_mc's process(x, clause, ev): result (a rational of the
// manager's NumeralService().Manager) receives the bound.
func (c *FMModelConverter) processBound(variable, clause ast.ASTHandle, ev *model.ModelEvaluator, ref ast.ASTRef, result *unumeral.Rational) (_ fmBoundKind, resultErr error) {
	m := c.manager
	q := m.NumeralService().Manager
	// rational r, bound, a, val;
	bound, coefficient, coef, value := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() {
		resultErr = errors.Join(resultErr, q.Del(&bound), q.Del(&coefficient), q.Del(&coef), q.Del(&value))
	}()
	f, op, n, e := c.appInfo(clause)
	if e != nil {
		return fmNoBound, e
	}
	lits := []ast.ASTHandle{clause}
	if f == m.BasicFamilyID() && op == ast.ASTOpOr {
		lits = nil
		for i := uint32(0); i < n; i++ {
			h, e := m.GetArg(clause, i)
			if e != nil {
				return fmNoBound, e
			}
			lits = append(lits, h)
		}
	}
	var kind fmBoundKind
	for _, lit := range lits {
		neg, arg, e := c.notArg(lit)
		if e != nil {
			return fmNoBound, e
		}
		atom := lit
		if neg {
			atom = arg
		}
		if yes, _ := c.uninterpConst(atom); yes {
			value, e := c.evaluate(ev, lit, ref)
			if e != nil {
				return fmNoBound, e
			}
			truth, e := m.MkTrue()
			if e != nil {
				return fmNoBound, e
			}
			if value == truth {
				return fmNoBound, nil
			}
			continue
		}
		af, aop, an, e := c.appInfo(atom)
		if e != nil {
			return fmNoBound, e
		}
		arith, e := m.ArithmeticFamilyID()
		if e != nil {
			return fmNoBound, e
		}
		if af != arith || (aop != 2 && aop != 3) || an != 2 {
			return fmNoBound, ast.ErrASTDomain
		}
		lhs, e := m.GetArg(atom, 0)
		if e != nil {
			return fmNoBound, e
		}
		rhs, e := m.GetArg(atom, 1)
		if e != nil {
			return fmNoBound, e
		}
		ok, e := fmRat(m, rhs, &bound)
		if e != nil || !ok {
			return fmNoBound, e
		}
		strict := neg
		directionNeg := neg
		if aop == 3 {
			directionNeg = !directionNeg
		}
		if directionNeg {
			if e = q.Neg(&bound); e != nil {
				return fmNoBound, e
			}
		}
		terms := []ast.ASTHandle{lhs}
		lf, lop, ln, _ := c.appInfo(lhs)
		if lf == arith && lop == 6 {
			terms = nil
			for i := uint32(0); i < ln; i++ {
				h, e := m.GetArg(lhs, i)
				if e != nil {
					return fmNoBound, e
				}
				terms = append(terms, h)
			}
		}
		if e = q.SetInt32(&coefficient, 0); e != nil {
			return fmNoBound, e
		}
		found := false
		for _, term := range terms {
			v := term
			if e = q.SetInt32(&coef, 1); e != nil {
				return fmNoBound, e
			}
			tf, top, tn, _ := c.appInfo(term)
			if tf == arith && top == 9 && tn == 2 {
				first, e := m.GetArg(term, 0)
				if e != nil {
					return fmNoBound, e
				}
				second, e := m.GetArg(term, 1)
				if e != nil {
					return fmNoBound, e
				}
				if yes, e := fmRat(m, first, &coef); e != nil {
					return fmNoBound, e
				} else if yes {
					v = second
				}
			}
			// Source strips to_real for declaration identity, while evaluating the
			// original monomial for variables already interpreted in the model.
			vf, vop, vn, e := c.appInfo(v)
			if e != nil {
				return fmNoBound, e
			}
			if vf == arith && vop == 14 && vn == 1 {
				v, e = m.GetArg(v, 0)
				if e != nil {
					return fmNoBound, e
				}
			}
			if ast.ASTSourceDebug {
				yes, e := c.uninterpConst(v)
				if e != nil {
					return fmNoBound, e
				}
				if !yes {
					return fmNoBound, ast.ErrASTDomain
				}
			}
			if yes, _ := c.isVariable(v, variable); yes {
				if e = q.Copy(&coefficient, &coef); e != nil {
					return fmNoBound, e
				}
				if directionNeg {
					if e = q.Neg(&coefficient); e != nil {
						return fmNoBound, e
					}
				}
				found = true
				continue
			}
			evaluated, e := c.evaluate(ev, term, ref)
			if e != nil {
				return fmNoBound, e
			}
			yes, e := fmRat(m, evaluated, &value)
			if ast.ASTSourceDebug && e == nil && !yes {
				return fmNoBound, ast.ErrASTDomain
			}
			if e != nil || !yes {
				return fmNoBound, e
			}
			if directionNeg {
				if e = q.Neg(&value); e != nil {
					return fmNoBound, e
				}
			}
			if e = q.Sub(&bound, &value, &bound); e != nil {
				return fmNoBound, e
			}
		}
		if !found || coefficient.IsZero() {
			return fmNoBound, ast.ErrASTDomain
		}
		sort, e := m.GetRange(variable)
		if e != nil {
			return fmNoBound, e
		}
		integer, e := m.GetDeclKind(sort)
		if e != nil {
			return fmNoBound, e
		}
		if integer == 1 && strict {
			if ast.ASTSourceDebug && !bound.IsInt() {
				return fmNoBound, ast.ErrASTDomain
			}
			if e = q.Dec(&bound); e != nil {
				return fmNoBound, e
			}
		}
		lower := coefficient.IsNeg()
		if e = q.Div(&bound, &coefficient, &bound); e != nil {
			return fmNoBound, e
		}
		if integer == 1 {
			if lower {
				e = q.Ceil(&bound, &bound)
			} else {
				e = q.Floor(&bound, &bound)
			}
			if e != nil {
				return fmNoBound, e
			}
		}
		if lower {
			kind = fmLowerBound
		} else {
			kind = fmUpperBound
		}
		if e = q.Copy(result, &bound); e != nil {
			return fmNoBound, e
		}
	}
	if ast.ASTSourceDebug && kind == fmNoBound {
		return fmNoBound, ast.ErrASTDomain
	}
	return kind, nil
}
