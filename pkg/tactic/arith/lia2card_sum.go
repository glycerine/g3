package arith

// Source: src/tactic/arith/lia2card_tactic.cpp, get_pb_sum/get_sum and helpers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// PBSum represents constant + sum(coefficients[i] * indicator(args[i])).
// All coefficients are positive. It owns the constructed Boolean expressions
// and its rationals (of the manager's NumeralService().Manager); callers must
// Close it before closing the manager.
type PBSum struct {
	Args         []ast.ASTHandle
	Coefficients []unumeral.Rational
	Constant     unumeral.Rational
	manager      *ast.ASTManager
	refs         []ast.ASTRef
}

func (s *PBSum) Close() error {
	if s == nil {
		return nil
	}
	// args.reset() and ~expr_ref_vector release front to back.
	for len(s.refs) > 0 {
		if err := s.manager.DelRef(&s.refs[0]); err != nil {
			return err
		}
		s.refs = s.refs[1:]
	}
	q := s.manager.NumeralService().Manager
	for len(s.Coefficients) > 0 {
		if err := q.Del(&s.Coefficients[0]); err != nil {
			return err
		}
		s.Coefficients = s.Coefficients[1:]
	}
	if err := q.Del(&s.Constant); err != nil {
		return err
	}
	s.Args = nil
	s.Coefficients = nil
	return nil
}

func (s *PBSum) pin(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := s.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.refs = append(s.refs, ref)
	return h, nil
}

// lia2cardNumeral recognizes the source's recursive minus and to_real numeral
// wrappers, without accepting arbitrary arithmetic expressions as numerals.
// The value is written into r (a rational of the manager's
// NumeralService().Manager).
func lia2cardNumeral(m *ast.ASTManager, h ast.ASTHandle, family ast.FamilyID, r *unumeral.Rational) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	fid, err := m.GetFamilyID(h)
	if err != nil || fid != family {
		return false, err
	}
	op, err := m.GetDeclKind(h)
	if err != nil {
		return false, err
	}
	if op == 8 || op == 14 {
		arg, err := m.GetArg(h, 0)
		if err != nil {
			return false, err
		}
		found, err := lia2cardNumeral(m, arg, family, r)
		if err == nil && found && op == 8 {
			err = m.NumeralService().Manager.Neg(r)
		}
		return found, err
	}
	_, found, err := m.ArithmeticNumeralRational(h, r)
	return found && err == nil, err
}

// ExtractPBSum borrows the arithmetic term and known 0/1 variables. It
// preserves source traversal order and duplicate weighted conditions. Failed
// extraction releases all temporary AST owners before returning.
func ExtractPBSum(m *ast.ASTManager, term ast.ASTHandle, zeroOne map[ast.ASTHandle]bool) (*PBSum, bool, error) {
	return extractPBSumWithTrace(m, term, zeroOne, nil)
}

func extractPBSumWithTrace(m *ast.ASTManager, term ast.ASTHandle, zeroOne map[ast.ASTHandle]bool, tags *diagnostic.TraceTags) (*PBSum, bool, error) {
	return extractPBSumTerms(m, []ast.ASTHandle{term}, []int32{1}, zeroOne, tags)
}

// extractPBSumTerms accumulates get_pb_sum(terms[i], muls[i], ...) into one
// sum, in order; is_pb calls it on x with 1 and on y with -1.
func extractPBSumTerms(m *ast.ASTManager, terms []ast.ASTHandle, muls []int32, zeroOne map[ast.ASTHandle]bool, tags *diagnostic.TraceTags) (_ *PBSum, _ bool, resultErr error) {
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, false, err
	}
	q := m.NumeralService().Manager
	// s holds only lia_rewriter_cfg::args. conds is get_pb_sum's local
	// expr_ref_vector: get_sum pushes and pops it, so an ite's (not c)
	// dies at its pop_back unless insert_arg copied it into args.
	s := &PBSum{manager: m, Constant: unumeral.NewRational()}
	var conds []ast.ASTRef
	pushCond := func(h ast.ASTHandle) error {
		ref, err := m.NewRef(h)
		if err == nil {
			conds = append(conds, ref)
		}
		return err
	}
	popCond := func() error {
		last := len(conds) - 1
		err := m.DelRef(&conds[last])
		conds = conds[:last]
		return err
	}
	defer func() {
		for len(conds) != 0 {
			_ = popCond()
		}
	}()
	// insert_arg: expr_ref cond = add_conds(conds, x), released on return.
	insert := func(weight *unumeral.Rational, x ast.ASTHandle) (result error) {
		var cond ast.ASTRef
		{
			// add_conds: es.push_back(e); result = mk_and(es); es.pop_back().
			pushed := !m.IsTrue(x)
			if pushed {
				if err := pushCond(x); err != nil {
					return err
				}
			}
			hs := make([]ast.ASTHandle, len(conds))
			var err error
			for i := range conds {
				if hs[i], err = m.RefGet(conds[i]); err != nil {
					break
				}
			}
			var and ast.ASTHandle
			if err == nil {
				and, err = m.MkAndArray(hs)
			}
			if err == nil {
				cond, err = m.NewRef(and)
			}
			if pushed {
				err = errors.Join(err, popCond())
			}
			if err != nil {
				return err
			}
		}
		defer func() { result = errors.Join(result, m.DelRef(&cond)) }()
		condition, err := m.RefGet(cond)
		if err != nil {
			return err
		}
		if m.IsTrue(condition) {
			return q.Add(&s.Constant, weight, &s.Constant)
		}
		sign, err := q.Sign(weight)
		if err != nil || sign == 0 {
			return err
		}
		coefficient := unumeral.NewRational()
		if err = q.Copy(&coefficient, weight); err != nil {
			return errors.Join(err, q.Del(&coefficient))
		}
		if sign < 0 {
			condition, err = s.pin(m.MkNot(condition))
			if err == nil {
				err = q.Neg(&coefficient)
			}
			if err == nil {
				err = q.Add(&s.Constant, weight, &s.Constant)
			}
		} else {
			condition, err = s.pin(condition, nil)
		}
		if err != nil {
			return errors.Join(err, q.Del(&coefficient))
		}
		s.Args = append(s.Args, condition)
		s.Coefficients = append(s.Coefficients, coefficient)
		return nil
	}
	// scaled is a rational temporary for -mul and r*mul.
	scaled := func(mul *unumeral.Rational, by *unumeral.Rational, out *unumeral.Rational) error {
		if by == nil {
			return errors.Join(q.Copy(out, mul), q.Neg(out))
		}
		return q.Mul(by, mul, out)
	}
	var getSum func(ast.ASTHandle, *unumeral.Rational) (bool, error)
	getSum = func(h ast.ASTHandle, mul *unumeral.Rational) (_ bool, resultErr error) {
		// rational r; and the scaled multiplier of a recursive call.
		r, next := unumeral.NewRational(), unumeral.NewRational()
		defer func() { resultErr = errors.Join(resultErr, q.Del(&r), q.Del(&next)) }()
		kind, err := m.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		fid, err := m.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return false, err
		}
		if fid == family {
			n, err := m.GetNumArgs(h)
			if err != nil {
				return false, err
			}
			if op == 6 {
				for i := uint32(0); i < n; i++ {
					a, err := m.GetArg(h, i)
					if err != nil {
						return false, err
					}
					ok, err := getSum(a, mul)
					if err != nil || !ok {
						return ok, err
					}
				}
				return true, nil
			}
			if (op == 7 || op == 9) && n == 2 {
				left, err := m.GetArg(h, 0)
				if err != nil {
					return false, err
				}
				right, err := m.GetArg(h, 1)
				if err != nil {
					return false, err
				}
				if op == 7 {
					ok, err := getSum(left, mul)
					if err != nil || !ok {
						return ok, err
					}
					if err = scaled(mul, nil, &next); err != nil {
						return false, err
					}
					return getSum(right, &next)
				}
				for side := 0; side < 2; side++ {
					ok, err := lia2cardNumeral(m, left, family, &r)
					if err != nil {
						return false, err
					}
					if ok {
						if err = scaled(mul, &r, &next); err != nil {
							return false, err
						}
						return getSum(right, &next)
					}
					left, right = right, left
				}
			} else if (op == 8 || op == 14) && n == 1 {
				arg, err := m.GetArg(h, 0)
				if err != nil {
					return false, err
				}
				if op == 8 {
					if err = scaled(mul, nil, &next); err != nil {
						return false, err
					}
					return getSum(arg, &next)
				}
				return getSum(arg, mul)
			}
		}
		ite, err := arithmeticBasicApp(m, h, ast.ASTOpIte)
		if err != nil {
			return false, err
		}
		if ite {
			condition, err := m.GetArg(h, 0)
			if err != nil {
				return false, err
			}
			left, err := m.GetArg(h, 1)
			if err != nil {
				return false, err
			}
			right, err := m.GetArg(h, 2)
			if err != nil {
				return false, err
			}
			if err = pushCond(condition); err != nil {
				return false, err
			}
			ok, err := getSum(left, mul)
			err = errors.Join(err, popCond())
			if err != nil {
				return false, err
			}
			neg, err := m.MkNot(condition)
			if err == nil {
				err = pushCond(neg)
			}
			if err != nil {
				return false, err
			}
			other, err := getSum(right, mul)
			err = errors.Join(err, popCond())
			return ok && other, err
		}
		if zeroOne[h] {
			sort, err := m.GetSort(h)
			if err != nil {
				return false, err
			}
			k, err := m.GetDeclKind(sort)
			if err != nil {
				return false, err
			}
			// mk_01: a raw m.mk_eq(x, a.mk_numeral(1, sort)).
			var one ast.ASTHandle
			if k == 1 {
				one, err = m.ArithmeticIntegerNumeralText("1")
			} else {
				one, err = m.ArithmeticRealNumeralText("1")
			}
			if err != nil {
				return false, err
			}
			indicator, err := m.MkEq(h, one)
			if err != nil {
				return false, err
			}
			return true, insert(mul, indicator)
		}
		ok, err := lia2cardNumeral(m, h, family, &r)
		if err != nil || !ok {
			if err == nil {
				err = lia2cardTraceUnsupported(m, tags, h)
			}
			return ok, err
		}
		truth, err := m.MkTrue()
		if err != nil {
			return false, err
		}
		if err = q.Mul(mul, &r, &next); err != nil {
			return false, err
		}
		return true, insert(&next, truth)
	}
	mul := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&mul)) }()
	for i, term := range terms {
		if err := q.SetInt32(&mul, muls[i]); err != nil {
			return nil, false, errors.Join(err, s.Close())
		}
		ok, err := getSum(term, &mul)
		if err != nil || !ok {
			return nil, ok, errors.Join(err, s.Close())
		}
	}
	return s, true, nil
}
