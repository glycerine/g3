package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:126-136 (mk_mul_core) and
// src/ast/rewriter/poly_rewriter_def.h:78-138 (mk_mul_app),
// 140-189 (mk_flat_mul_core), 192-328 (mk_nflat_mul_core), specialized for
// arith_rewriter_core.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/productiter"
)

// rewriteMul is poly_rewriter::mk_mul_core.
func (r *ArithRewriter) rewriteMul(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if args.Count() == 0 {
		return RewriteFailed, ast.ErrASTDomain
	}
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		h, err := args.At(uint32(i))
		if err != nil {
			return RewriteFailed, err
		}
		values[i] = h
	}
	if len(values) == 1 {
		return RewriteDone, r.manager.SetRef(out, values[0])
	}
	// set_curr_sort(m().get_sort(args[0])): numerals take the sort of args[0].
	var st RewriteStatus
	var result ast.ASTHandle
	var err error
	if r.flat {
		st, result, err = r.mkFlatMulCore(values, values[0])
	} else {
		st, result, err = r.mkNflatMulCore(values, values[0])
	}
	if err != nil || st == RewriteFailed {
		return st, err
	}
	return st, r.manager.SetRef(out, result)
}

func (r *ArithRewriter) isMul(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	_, children, matched, err := r.operation(h, arithOpMul)
	return children, matched, err
}

func (r *ArithRewriter) isAdd(h ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	kind, err := r.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	_, children, matched, err := r.operation(h, arithOpAdd)
	return children, matched, err
}

func (r *ArithRewriter) isNumeralArg(h ast.ASTHandle) (*big.Rat, bool, error) {
	v, _, numeral, err := r.numeral(h)
	return v, numeral, err
}

// mkMulApp is the n-ary poly_rewriter::mk_mul_app.
func (r *ArithRewriter) mkMulApp(args []ast.ASTHandle, sortTerm ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(args) {
	case 0:
		return r.numeralForTerm(big.NewRat(1, 1), sortTerm)
	case 1:
		return args[0], nil
	}
	if !(r.mulToPower && !r.expandPower) { // use_power()
		return r.app(arithOpMul, args...)
	}
	prev, kPrev, err := r.powerBody(args[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var newArgs []ast.ASTHandle
	pushPower := func() error {
		if kPrev.Cmp(big.NewRat(1, 1)) == 0 {
			newArgs = append(newArgs, prev)
			return nil
		}
		k, err := r.numeralForTerm(kPrev, sortTerm)
		if err != nil {
			return err
		}
		power, err := r.app(arithOpPower, prev, k)
		if err != nil {
			return err
		}
		newArgs = append(newArgs, power)
		return nil
	}
	for i := 1; i < len(args); i++ {
		arg, k, err := r.powerBody(args[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if arg == prev {
			kPrev = new(big.Rat).Add(kPrev, k)
		} else {
			if err = pushPower(); err != nil {
				return ast.ASTHandle{}, err
			}
			prev = arg
			kPrev = k
		}
	}
	if err = pushPower(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(newArgs) == 1 {
		return newArgs[0], nil
	}
	return r.app(arithOpMul, newArgs...)
}

// mkMulAppCoeff is poly_rewriter::mk_mul_app(numeral const & c, expr * arg).
func (r *ArithRewriter) mkMulAppCoeff(c *big.Rat, arg, sortTerm ast.ASTHandle) (ast.ASTHandle, error) {
	if c.Cmp(big.NewRat(1, 1)) == 0 {
		return arg, nil
	}
	if v, numeral, err := r.isNumeralArg(arg); err != nil {
		return ast.ASTHandle{}, err
	} else if numeral && v.Sign() == 0 { // is_zero(arg)
		return arg, nil
	}
	n, err := r.numeralForTerm(c, sortTerm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.mkMulApp([]ast.ASTHandle{n, arg}, sortTerm)
}

// mkFlatMulCore is poly_rewriter::mk_flat_mul_core.
func (r *ArithRewriter) mkFlatMulCore(args []ast.ASTHandle, sortTerm ast.ASTHandle) (RewriteStatus, ast.ASTHandle, error) {
	numArgs := len(args)
	// only try to apply flattening if it is not already in one of the flat
	// monomial forms
	// - (* c x)
	// - (* c (* x_1 ... x_n))
	_, firstNumeral, err := r.isNumeralArg(args[0])
	if err != nil {
		return RewriteFailed, ast.ASTHandle{}, err
	}
	nestedCoeff := false
	if numArgs == 2 && firstNumeral {
		children, mul, err := r.isMul(args[1])
		if err != nil {
			return RewriteFailed, ast.ASTHandle{}, err
		}
		if mul {
			_, nestedCoeff, err = r.isNumeralArg(children[0])
			if err != nil {
				return RewriteFailed, ast.ASTHandle{}, err
			}
		}
	}
	if numArgs != 2 || !firstNumeral || nestedCoeff {
		i := 0
		for ; i < numArgs; i++ {
			_, mul, err := r.isMul(args[i])
			if err != nil {
				return RewriteFailed, ast.ASTHandle{}, err
			}
			if mul {
				break
			}
		}
		if i < numArgs {
			// input has nested monomials.
			flatArgs := append([]ast.ASTHandle(nil), args[:i]...)
			// we need the todo buffer to handle:
			// (* (* c (* x_1 ... x_n)) (* d (* y_1 ... y_n)))
			var todo []ast.ASTHandle
			for j := i; j < numArgs; j++ {
				_, mul, err := r.isMul(args[j])
				if err != nil {
					return RewriteFailed, ast.ASTHandle{}, err
				}
				if !mul {
					flatArgs = append(flatArgs, args[j])
					continue
				}
				todo = append(todo, args[j])
				for len(todo) != 0 {
					curr := todo[len(todo)-1]
					todo = todo[:len(todo)-1]
					children, mul, err := r.isMul(curr)
					if err != nil {
						return RewriteFailed, ast.ASTHandle{}, err
					}
					if mul {
						for k := len(children); k > 0; {
							k--
							todo = append(todo, children[k])
						}
					} else {
						flatArgs = append(flatArgs, curr)
					}
				}
			}
			st, result, err := r.mkNflatMulCore(flatArgs, sortTerm)
			if err != nil {
				return RewriteFailed, ast.ASTHandle{}, err
			}
			if st == RewriteFailed {
				result, err = r.mkMulApp(flatArgs, sortTerm)
				return RewriteDone, result, err
			}
			return st, result, nil
		}
	}
	return r.mkNflatMulCore(args, sortTerm)
}

// mkNflatMulCore is poly_rewriter::mk_nflat_mul_core.
func (r *ArithRewriter) mkNflatMulCore(args []ast.ASTHandle, sortTerm ast.ASTHandle) (RewriteStatus, ast.ASTHandle, error) {
	fail := func(err error) (RewriteStatus, ast.ASTHandle, error) { return RewriteFailed, ast.ASTHandle{}, err }
	numArgs := len(args)
	// cheap case
	if numArgs == 2 {
		a, numeral, err := r.isNumeralArg(args[0])
		if err != nil {
			return fail(err)
		}
		if numeral && a.Cmp(big.NewRat(1, 1)) != 0 && a.Sign() != 0 {
			kind, err := r.manager.GetKind(args[1])
			if err != nil {
				return fail(err)
			}
			if kind == ast.ASTVar {
				return fail(nil)
			}
			family, err := r.manager.GetFamilyID(args[1])
			if err != nil {
				return fail(err)
			}
			if family != r.family {
				return fail(nil)
			}
		}
	}
	c := big.NewRat(1, 1)
	numCoeffs := 0
	numAdd := 0
	var variable ast.ASTHandle
	for _, arg := range args {
		a, numeral, err := r.isNumeralArg(arg)
		if err != nil {
			return fail(err)
		}
		if numeral {
			numCoeffs++
			c.Mul(c, a)
			continue
		}
		variable = arg
		if _, add, err := r.isAdd(arg); err != nil {
			return fail(err)
		} else if add {
			numAdd++
		}
	}
	// normalize(c) is the identity for arithmetic.
	// (* c_1 ... c_n) --> c_1*...*c_n
	// (* s ... 0 ... r) --> 0
	if numCoeffs == numArgs || c.Sign() == 0 {
		result, err := r.numeralForTerm(c, sortTerm)
		return RewriteDone, result, err
	}
	if numCoeffs == numArgs-1 {
		// (* c_1 ... c_n x) --> x  if c_1*...*c_n == 1
		if c.Cmp(big.NewRat(1, 1)) == 0 {
			return RewriteDone, variable, nil
		}
		children, mul, err := r.isMul(variable)
		if err != nil {
			return fail(err)
		}
		if mul {
			// apply basic simplification even when flattening is not enabled.
			// (* c1 (* c2 x)) --> (* c1*c2 x)
			if len(children) == 2 {
				cPrime, numeral, err := r.isNumeralArg(children[0])
				if err != nil {
					return fail(err)
				}
				if numeral {
					c.Mul(c, cPrime)
					result, err := r.mkMulAppCoeff(c, children[1], sortTerm)
					return Rewrite1, result, err
				}
			}
			// var is a power-product
			return fail(nil)
		}
		if numAdd == 0 || r.hoistCoeff {
			if numArgs == 2 && args[1] == variable {
				// it is already simplified
				return fail(nil)
			}
			// (* c_1 ... c_n x) --> (* c_1*...*c_n x)
			result, err := r.mkMulAppCoeff(c, variable, sortTerm)
			return RewriteDone, result, err
		}
		// (* c_1 ... c_n (+ t_1 ... t_m)) --> (+ (* c_1*...*c_n t_1) ... (* c_1*...*c_n t_m))
		terms, _, err := r.isAdd(variable)
		if err != nil {
			return fail(err)
		}
		newAddArgs := make([]ast.ASTHandle, len(terms))
		for i, t := range terms {
			if newAddArgs[i], err = r.mkMulAppCoeff(c, t, sortTerm); err != nil {
				return fail(err)
			}
		}
		result, err := r.makeAdd(newAddArgs, sortTerm)
		return Rewrite2, result, err
	}

	if !r.sumOfMonomials || numAdd == 0 {
		var newArgs []ast.ASTHandle
		var prev ast.ASTHandle
		havePrev := false
		ordered := true
		for _, curr := range args {
			if _, numeral, err := r.isNumeralArg(curr); err != nil {
				return fail(err)
			} else if numeral {
				continue
			}
			if havePrev {
				less, err := r.monomialLess(curr, prev)
				if err != nil {
					return fail(err)
				}
				if less {
					ordered = false
				}
			}
			newArgs = append(newArgs, curr)
			prev, havePrev = curr, true
		}
		if ordered && numCoeffs == 0 && !(r.mulToPower && !r.expandPower) {
			return fail(nil)
		}
		if !ordered {
			if err := r.sortMonomials(newArgs); err != nil {
				return fail(err)
			}
		}
		result, err := r.mkMulApp(newArgs, sortTerm)
		if err != nil {
			return fail(err)
		}
		result, err = r.mkMulAppCoeff(c, result, sortTerm)
		return RewriteDone, result, err
	}

	// sum of monomials
	sums := make([][]ast.ASTHandle, numArgs)
	szs := make([]uint32, numArgs)
	it := make([]uint32, numArgs)
	for i, arg := range args {
		terms, add, err := r.isAdd(arg)
		if err != nil {
			return fail(err)
		}
		if add {
			sums[i] = terms
		} else {
			sums[i] = []ast.ASTHandle{arg}
		}
		szs[i] = uint32(len(sums[i]))
	}
	var sum []ast.ASTHandle
	mArgs := make([]ast.ASTHandle, numArgs)
	for {
		if uint64(len(sum)) > uint64(r.monomialLimit) {
			return fail(ErrArithPolynomialMonomialBlowup)
		}
		for i := range args {
			mArgs[i] = sums[i][it[i]]
		}
		m, err := r.mkMulApp(mArgs, sortTerm)
		if err != nil {
			return fail(err)
		}
		sum = append(sum, m)
		if !productiter.Next(szs, it) {
			break
		}
	}
	result, err := r.makeAdd(sum, sortTerm)
	return Rewrite2, result, err
}

// sortMonomials is std::sort(new_args.begin(), new_args.end(), lt) with mon_lt.
func (r *ArithRewriter) sortMonomials(args []ast.ASTHandle) error {
	var cause error
	sort.SliceStable(args, func(i, j int) bool {
		less, e := r.monomialLess(args[i], args[j])
		if cause == nil {
			cause = e
		}
		return e == nil && less
	})
	return cause
}
