package rewriter

// Source: src/ast/rewriter/arith_rewriter.cpp:367-454,561-583,685-733.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (r *ArithRewriter) algebraicNumeralSyntax(h ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := r.manager.GetFamilyID(h)
	if err != nil || family != r.family {
		return false, err
	}
	declKind, err := r.manager.GetDeclKind(h)
	return err == nil && declKind == 1, err
}

// algebraicOperand is to_irrational_algebraic_numeral(h), or am.set(v,
// q.to_mpq()) for a rational numeral h. It applies no max_degree test:
// only mk_div_irrat_irrat, mk_power_core and is_anum_simp_target do.
func (r *ArithRewriter) algebraicOperand(h ast.ASTHandle, manager *polynomial.AlgebraicManager) (value polynomial.Algebraic, irrational, matched bool, err error) {
	matched, err = r.manager.ArithmeticAlgebraicNumeralValue(h, &value)
	if err != nil || matched {
		irrational = matched
		return
	}
	rational, _, numeral, e := r.numeral(h)
	if e != nil || !numeral {
		return value, false, false, e
	}
	service := r.manager.NumeralService()
	owned, e := service.NewValue()
	if e != nil {
		return value, false, false, e
	}
	defer func() { err = errors.Join(err, service.Del(&owned)) }()
	if e = service.SetString(&owned, rational.RatString()); e != nil {
		return value, false, false, e
	}
	if e = manager.SetRational(&value, &owned.Raw); e != nil {
		return value, false, false, e
	}
	return value, false, true, nil
}

func (r *ArithRewriter) makeAlgebraicNumeral(manager *polynomial.AlgebraicManager, value *polynomial.Algebraic) (result ast.ASTHandle, err error) {
	rational, err := manager.IsRational(value)
	if err != nil {
		return result, err
	}
	if !rational {
		return r.manager.ArithmeticAlgebraicNumeral(value)
	}
	service := r.manager.NumeralService()
	owned, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, service.Del(&owned)) }()
	if err = manager.ToOwnedRational(value, service, &owned); err != nil {
		return result, err
	}
	return r.manager.ArithmeticNumeral(&owned, false)
}

func (r *ArithRewriter) rewriteAlgebraicComparison(left, right ast.ASTHandle, relation int32, out ast.ASTRef) (decided bool, err error) {
	if !r.algebraicSimplify {
		return false, nil
	}
	leftSyntax, err := r.algebraicNumeralSyntax(left)
	if err != nil {
		return false, err
	}
	rightSyntax, err := r.algebraicNumeralSyntax(right)
	if err != nil || !leftSyntax && !rightSyntax {
		return false, err
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return false, err
	}
	leftValue, leftIrrational, leftMatched, err := r.algebraicOperand(left, manager)
	if err != nil || !leftMatched {
		return false, err
	}
	defer func() { err = errors.Join(err, manager.Del(&leftValue)) }()
	rightValue, rightIrrational, rightMatched, err := r.algebraicOperand(right, manager)
	if err != nil || !rightMatched {
		return false, err
	}
	defer func() { err = errors.Join(err, manager.Del(&rightValue)) }()
	if !leftIrrational && !rightIrrational {
		return false, nil
	}
	comparison, err := manager.Compare(&leftValue, &rightValue)
	if err != nil {
		return false, err
	}
	truth := comparison == 0
	if relation == arithOpLE {
		truth = comparison <= 0
	} else if relation == arithOpGE {
		truth = comparison >= 0
	}
	return true, r.setBool(out, truth)
}

// isAnumSimpTarget is arith_rewriter::is_anum_simp_target. A nil manager
// is borrowed at the first irrational argument.
func (r *ArithRewriter) isAnumSimpTarget(manager *polynomial.AlgebraicManager, args ast.AppArguments) (bool, error) {
	if !r.algebraicSimplify {
		return false, nil
	}
	numIrrat, numRat := 0, 0
	for i := uint32(0); i < args.Count(); i++ {
		h, err := args.At(i)
		if err != nil {
			return false, err
		}
		_, _, numeral, err := r.numeral(h)
		if err != nil {
			return false, err
		}
		if numeral {
			numRat++
			if numIrrat > 0 {
				return true, nil
			}
		}
		low, err := r.lowDegreeIrrational(manager, h)
		if err != nil {
			return false, err
		}
		if low {
			numIrrat++
			if numIrrat > 1 || numRat > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

// lowDegreeIrrational is is_irrational_algebraic_numeral(e) &&
// am.degree(to_irrational_algebraic_numeral(e)) <= m_max_degree.
func (r *ArithRewriter) lowDegreeIrrational(manager *polynomial.AlgebraicManager, h ast.ASTHandle) (low bool, err error) {
	if syntax, e := r.algebraicNumeralSyntax(h); e != nil || !syntax {
		return false, e
	}
	if manager == nil {
		if manager, err = r.manager.ArithmeticAlgebraicManager(); err != nil {
			return false, err
		}
	}
	var value polynomial.Algebraic
	matched, err := r.manager.ArithmeticAlgebraicNumeralValue(h, &value)
	if err != nil || !matched {
		return false, err
	}
	defer func() { err = errors.Join(err, manager.Del(&value)) }()
	degree, err := manager.Degree(&value)
	return err == nil && degree <= r.maxDegree, err
}

// rewriteAlgebraicFold is the is_anum_simp_target branch of
// arith_rewriter::mk_add_core and mk_mul_core.
func (r *ArithRewriter) rewriteAlgebraicFold(kind int32, args ast.AppArguments, out ast.ASTRef) (status RewriteStatus, result error) {
	if !r.algebraicSimplify {
		return RewriteFailed, nil
	}
	target, err := r.isAnumSimpTarget(nil, args)
	if err != nil || !target {
		return RewriteFailed, err
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, err
	}
	// expr_ref_buffer new_args holds the numerals made for the accumulator.
	var newArgs []ast.ASTHandle
	var held []ast.ASTRef
	defer func() {
		for i := range held {
			result = errors.Join(result, r.manager.DelRef(&held[i]))
		}
	}()
	unit := big.NewRat(0, 1)
	if kind == arithOpMul {
		unit = big.NewRat(1, 1)
	}
	var acc polynomial.Algebraic
	if err = r.algebraicFromRational(manager, unit, &acc); err != nil {
		return RewriteFailed, err
	}
	defer func() { result = errors.Join(result, manager.Del(&acc)) }()
	pushNumeral := func() error {
		h, e := r.makeAlgebraicNumeral(manager, &acc)
		if e != nil {
			return e
		}
		ref, e := r.manager.NewRef(h)
		if e != nil {
			return e
		}
		held = append(held, ref)
		newArgs = append(newArgs, h)
		return nil
	}
	combine := func(arg *polynomial.Algebraic) error {
		var next polynomial.Algebraic
		var e error
		if kind == arithOpAdd {
			e = manager.Add(&acc, arg, &next)
		} else {
			e = manager.Mul(&acc, arg, &next)
		}
		if e != nil {
			return e
		}
		if e = manager.Del(&acc); e != nil {
			return errors.Join(e, manager.Del(&next))
		}
		acc = next
		return nil
	}
	for i := uint32(0); i < args.Count(); i++ {
		d, e := manager.Degree(&acc)
		if e != nil {
			return RewriteFailed, e
		}
		if d > 1 && d > r.maxDegree {
			if e = pushNumeral(); e != nil {
				return RewriteFailed, e
			}
			if e = manager.Del(&acc); e != nil {
				return RewriteFailed, e
			}
			if e = r.algebraicFromRational(manager, unit, &acc); e != nil {
				return RewriteFailed, e
			}
		}
		h, e := args.At(i)
		if e != nil {
			return RewriteFailed, e
		}
		rational, _, numeral, e := r.numeral(h)
		if e != nil {
			return RewriteFailed, e
		}
		if numeral {
			var arg polynomial.Algebraic
			if e = r.algebraicFromRational(manager, rational, &arg); e != nil {
				return RewriteFailed, e
			}
			e = combine(&arg)
			if e = errors.Join(e, manager.Del(&arg)); e != nil {
				return RewriteFailed, e
			}
			continue
		}
		var irarg polynomial.Algebraic
		irrational, e := r.manager.ArithmeticAlgebraicNumeralValue(h, &irarg)
		if e != nil {
			return RewriteFailed, e
		}
		if irrational {
			degree, e := manager.Degree(&irarg)
			if e == nil && degree <= r.maxDegree {
				e = combine(&irarg)
				if e = errors.Join(e, manager.Del(&irarg)); e != nil {
					return RewriteFailed, e
				}
				continue
			}
			if e = errors.Join(e, manager.Del(&irarg)); e != nil {
				return RewriteFailed, e
			}
		}
		newArgs = append(newArgs, h)
	}
	if len(newArgs) == 0 {
		h, e := r.makeAlgebraicNumeral(manager, &acc)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, h)
	}
	if err = pushNumeral(); err != nil {
		return RewriteFailed, err
	}
	if kind == arithOpAdd {
		status, err = r.rewriteAdd(ast.SliceArguments(newArgs), out)
	} else {
		status, err = r.rewriteMul(ast.SliceArguments(newArgs), out)
	}
	if err != nil || status != RewriteFailed {
		return status, err
	}
	h, err := r.app(kind, newArgs...)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}

// rewriteAlgebraicDivision is the m_anum_simp dispatch of
// arith_rewriter::mk_div_core: mk_div_irrat_rat, mk_div_irrat_irrat or
// mk_div_rat_irrat. dispatched reports that mk_div_core returns the
// returned status.
func (r *ArithRewriter) rewriteAlgebraicDivision(left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, dispatched bool, err error) {
	if !r.algebraicSimplify {
		return RewriteFailed, false, nil
	}
	leftIrrational, err := r.algebraicNumeralSyntax(left)
	if err != nil {
		return RewriteFailed, false, err
	}
	rightIrrational, err := r.algebraicNumeralSyntax(right)
	if err != nil || !leftIrrational && !rightIrrational {
		return RewriteFailed, false, err
	}
	_, _, leftRational, err := r.numeral(left)
	if err != nil {
		return RewriteFailed, false, err
	}
	_, _, rightRational, err := r.numeral(right)
	if err != nil {
		return RewriteFailed, false, err
	}
	if !(leftIrrational && (rightRational || rightIrrational) || rightIrrational && leftRational) {
		return RewriteFailed, false, nil
	}
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, true, err
	}
	a, _, am, err := r.algebraicOperand(left, manager)
	if err != nil || !am {
		return RewriteFailed, true, err
	}
	defer func() { _ = manager.Del(&a) }()
	b, _, bm, err := r.algebraicOperand(right, manager)
	if err != nil || !bm {
		return RewriteFailed, true, err
	}
	defer func() { _ = manager.Del(&b) }()
	if leftIrrational && rightRational {
		// mk_div_irrat_rat
		zero, e := manager.IsZero(&b)
		if e != nil || zero {
			return RewriteFailed, true, e
		}
	} else if leftIrrational && rightIrrational {
		// mk_div_irrat_irrat
		for _, v := range []*polynomial.Algebraic{&a, &b} {
			degree, e := manager.Degree(v)
			if e != nil || degree > r.maxDegree {
				return RewriteFailed, true, e
			}
		}
	}
	var quotient polynomial.Algebraic
	if err = manager.Div(&a, &b, &quotient); err != nil {
		return RewriteFailed, true, err
	}
	defer func() { _ = manager.Del(&quotient) }()
	h, err := r.makeAlgebraicNumeral(manager, &quotient)
	if err != nil {
		return RewriteFailed, true, err
	}
	return RewriteDone, true, r.manager.SetRef(out, h)
}

// algebraicFromRational is am.set(r, q.to_mpq()).
func (r *ArithRewriter) algebraicFromRational(manager *polynomial.AlgebraicManager, rational *big.Rat, value *polynomial.Algebraic) (err error) {
	service := r.manager.NumeralService()
	owned, err := service.NewValue()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, service.Del(&owned)) }()
	if err = service.SetString(&owned, rational.RatString()); err != nil {
		return err
	}
	return manager.SetRational(value, &owned.Raw)
}

// rewriteAlgebraicRationalRoot is mk_power_core's m_anum_simp branch for a
// rational base: am.set(r, xk); am.root(r, u_den_y, r); if (is_neg_y)
// am.inv(r); result = mk_numeral(r, false).
func (r *ArithRewriter) rewriteAlgebraicRationalRoot(xk *big.Rat, den uint32, negative bool, out ast.ASTRef) (RewriteStatus, error) {
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, err
	}
	var value polynomial.Algebraic
	if err = r.algebraicFromRational(manager, xk, &value); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&value) }()
	return r.algebraicRootResult(manager, &value, den, negative, out)
}

// rewriteAlgebraicPower is mk_power_core's irrational branch: the degree
// test, am.power(val, u_num_y, r), am.root(r, u_den_y, r), am.inv(r) for a
// negative exponent and mk_numeral(r, false).
func (r *ArithRewriter) rewriteAlgebraicPower(base ast.ASTHandle, num, den uint32, negative bool, out ast.ASTRef) (RewriteStatus, error) {
	manager, err := r.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return RewriteFailed, err
	}
	var value polynomial.Algebraic
	matched, err := r.manager.ArithmeticAlgebraicNumeralValue(base, &value)
	if err != nil || !matched {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&value) }()
	degree, err := manager.Degree(&value)
	if err != nil || degree > r.maxDegree {
		return RewriteFailed, err
	}
	if den%2 == 0 {
		isNegative, e := manager.IsNeg(&value)
		if e != nil {
			return RewriteFailed, e
		}
		if isNegative && num%2 == 1 {
			return RewriteFailed, nil
		}
	}
	var powered polynomial.Algebraic
	if err = manager.Power(&value, num, &powered); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&powered) }()
	return r.algebraicRootResult(manager, &powered, den, negative, out)
}

func (r *ArithRewriter) algebraicRootResult(manager *polynomial.AlgebraicManager, value *polynomial.Algebraic, den uint32, negative bool, out ast.ASTRef) (RewriteStatus, error) {
	var rooted polynomial.Algebraic
	if err := manager.Root(value, den, &rooted); err != nil {
		return RewriteFailed, err
	}
	defer func() { _ = manager.Del(&rooted) }()
	if negative {
		if err := manager.Inv(&rooted); err != nil {
			return RewriteFailed, err
		}
	}
	h, err := r.makeAlgebraicNumeral(manager, &rooted)
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, h)
}
