package rewriter

// Source: src/ast/rewriter/arith_rewriter.cpp:1154-1655.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

const (
	arithOpSin   int32 = 19
	arithOpCos   int32 = 20
	arithOpTan   int32 = 21
	arithOpASin  int32 = 22
	arithOpACos  int32 = 23
	arithOpATan  int32 = 24
	arithOpSinh  int32 = 25
	arithOpCosh  int32 = 26
	arithOpTanh  int32 = 27
	arithOpASinh int32 = 28
	arithOpACosh int32 = 29
	arithOpATanh int32 = 30
	arithOpPi    int32 = 31
)

func (r *ArithRewriter) pi() (ast.ASTHandle, error) { return r.app(arithOpPi) }

func (r *ArithRewriter) negativeTerm(h ast.ASTHandle) (_ ast.ASTHandle, _ bool, err error) {
	_, args, mul, err := r.operation(h, arithOpMul)
	if err != nil || !mul || len(args) != 2 {
		return ast.ASTHandle{}, false, err
	}
	// rational r
	coefficient := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &coefficient)) }()
	_, numeral, err := r.numeral(args[0], &coefficient)
	if err != nil || !numeral || !coefficient.IsMinusOne() {
		return ast.ASTHandle{}, false, err
	}
	return args[1], true, nil
}

// arithQFrac is rational(n, d) for a constant fraction in lowest terms.
func arithQFrac(n, d int32) unumeral.Rational {
	return unumeral.Rational{Num: unumeral.NewInt(n), Den: unumeral.NewInt(d)}
}

// arithQIs is k == rational(n, d) for a fraction in lowest terms: a
// normalized rational equal to it has the same inline numerator and
// denominator.
func arithQIs(k *unumeral.Rational, n, d int32) bool {
	return k.Num.IsSmall() && k.Den.IsSmall() && k.Num.Val == n && k.Den.Val == d
}

// makeNumeralFrac is m_util.mk_numeral(rational(n, d), is_int).
func (r *ArithRewriter) makeNumeralFrac(n, d int32, integer bool) (ast.ASTHandle, error) {
	value := arithQFrac(n, d)
	return r.makeNumeral(&value, integer)
}

// piMultiple is is_pi_multiple(t, k): k receives the coefficient.
func (r *ArithRewriter) piMultiple(h ast.ASTHandle, k *unumeral.Rational) (bool, error) {
	_, args, pi, err := r.operation(h, arithOpPi)
	if err != nil {
		return false, err
	}
	if pi && len(args) == 0 {
		return true, r.q().SetInt32(k, 1)
	}
	_, args, mul, err := r.operation(h, arithOpMul)
	if err != nil || !mul || len(args) != 2 {
		return false, err
	}
	_, piArgs, pi, err := r.operation(args[1], arithOpPi)
	if err != nil || !pi || len(piArgs) != 0 {
		return false, err
	}
	_, numeral, err := r.numeral(args[0], k)
	return numeral, err
}

// arithPiPeriod sets out = mod(floor(k), rational(2)) + k - floor(k).
func arithPiPeriod(q *unumeral.RationalManager, k, out *unumeral.Rational) (err error) {
	floor, two := unumeral.NewRational(), arithQFrac(2, 1)
	defer func() { err = errors.Join(err, arithQDel(q, &floor)) }()
	if err = q.Floor(k, &floor); err != nil {
		return err
	}
	if err = q.Sub(k, &floor, out); err != nil {
		return err
	}
	if err = q.Mod(&floor, &two, &floor); err != nil {
		return err
	}
	return q.Add(&floor, out, out)
}

// sqrt is mk_sqrt(rational(n, d)): (^ n/d 1/2).
func (r *ArithRewriter) sqrt(n, d int32) (ast.ASTHandle, error) {
	base, err := r.makeNumeralFrac(n, d, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	half, err := r.makeNumeralFrac(1, 2, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.app(arithOpPower, base, half)
}

func (r *ArithRewriter) signed(h ast.ASTHandle, negative bool) (ast.ASTHandle, error) {
	if !negative {
		return h, nil
	}
	return r.app(arithOpUMinus, h)
}

// sinValue is mk_sin_value(k).
func (r *ArithRewriter) sinValue(k *unumeral.Rational) (_ ast.ASTHandle, _ bool, err error) {
	q := r.q()
	// rational k_prime
	kPrime := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(q, &kPrime)) }()
	if err = arithPiPeriod(q, k, &kPrime); err != nil {
		return ast.ASTHandle{}, false, err
	}
	cmp, err := q.Compare(&kPrime, arithQOne())
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	negative := cmp >= 0
	if negative {
		if err = q.Dec(&kPrime); err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	makeRat := func(n, d int32) (ast.ASTHandle, bool, error) {
		if negative {
			n = -n
		}
		h, err := r.makeNumeralFrac(n, d, false)
		return h, err == nil, err
	}
	is := func(n, d int32) bool { return arithQIs(&kPrime, n, d) }
	if kPrime.IsZero() || kPrime.IsOne() {
		h, err := r.makeNumeralInt(0, false)
		return h, err == nil, err
	}
	if is(1, 2) {
		return makeRat(1, 1)
	}
	if is(1, 6) || is(5, 6) {
		return makeRat(1, 2)
	}
	if is(1, 4) || is(3, 4) {
		h, err := r.sqrt(1, 2)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err = r.signed(h, negative)
		return h, err == nil, err
	}
	if is(1, 3) || is(2, 3) {
		root, err := r.sqrt(3, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		two, err := r.makeNumeralInt(2, false)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err := r.app(arithOpDiv, root, two)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err = r.signed(h, negative)
		return h, err == nil, err
	}
	if is(1, 12) || is(11, 12) || is(5, 12) || is(7, 12) {
		six, err := r.sqrt(6, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		twoRoot, err := r.sqrt(2, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		operator := arithOpSub
		if is(5, 12) || is(7, 12) {
			operator = arithOpAdd
		}
		numerator, err := r.app(operator, six, twoRoot)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		four, err := r.makeNumeralInt(4, false)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err := r.app(arithOpDiv, numerator, four)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		h, err = r.signed(h, negative)
		return h, err == nil, err
	}
	return ast.ASTHandle{}, false, nil
}

// piOffset is is_pi_offset(t, k, m): k receives m's coefficient.
func (r *ArithRewriter) piOffset(h ast.ASTHandle, k *unumeral.Rational) (ast.ASTHandle, bool, error) {
	_, args, add, err := r.operation(h, arithOpAdd)
	if err != nil || !add {
		return ast.ASTHandle{}, false, err
	}
	for _, arg := range args {
		yes, e := r.piMultiple(arg, k)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if yes {
			return arg, true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) integerPiMultiple(h ast.ASTHandle, even bool) (_ bool, err error) {
	_, args, mul, err := r.operation(h, arithOpMul)
	if err != nil || !mul || len(args) != 2 {
		return false, err
	}
	isPair := func(left, right ast.ASTHandle) (bool, error) {
		_, piArgs, pi, e := r.operation(left, arithOpPi)
		if e != nil {
			return false, e
		}
		toReal, _, e := r.unary(right, arithOpToReal)
		return pi && len(piArgs) == 0 && toReal, e
	}
	q := r.q()
	// rational k(1)
	coefficient := arithQFrac(1, 1)
	defer func() { err = errors.Join(err, arithQDel(q, &coefficient)) }()
	pair := args
	if _, numeral, e := r.numeral(args[0], &coefficient); e != nil {
		return false, e
	} else if numeral {
		if !coefficient.IsInt() {
			return false, nil
		}
		_, pair, mul, err = r.operation(args[1], arithOpMul)
		if err != nil || !mul || len(pair) != 2 {
			return false, err
		}
	}
	if even {
		if isEven, e := q.IsEven(&coefficient); e != nil || !isEven {
			return false, e
		}
	}
	if yes, e := isPair(pair[0], pair[1]); e != nil || yes {
		return yes, e
	}
	return isPair(pair[1], pair[0])
}

func (r *ArithRewriter) integerPiOffset(h ast.ASTHandle, even bool) (ast.ASTHandle, bool, error) {
	_, args, add, err := r.operation(h, arithOpAdd)
	if err != nil || !add {
		return ast.ASTHandle{}, false, err
	}
	for _, arg := range args {
		yes, e := r.integerPiMultiple(arg, even)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if yes {
			return arg, true, nil
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (r *ArithRewriter) subtractOffset(arg, offset ast.ASTHandle) (ast.ASTHandle, error) {
	return r.app(arithOpSub, arg, offset)
}

func (r *ArithRewriter) rewriteSin(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// rational k, k_prime
	coefficient, period := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(r.q(), &period, &coefficient)) }()
	if yes, child, err := r.unary(arg, arithOpASin); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	if yes, child, err := r.unary(arg, arithOpACos); err != nil {
		return RewriteFailed, err
	} else if yes {
		one, e := r.makeNumeralInt(1, false)
		if e != nil {
			return RewriteFailed, e
		}
		square, e := r.app(arithOpMul, child, child)
		if e != nil {
			return RewriteFailed, e
		}
		difference, e := r.app(arithOpSub, one, square)
		if e != nil {
			return RewriteFailed, e
		}
		root, e := r.sqrt(1, 1)
		if e != nil {
			return RewriteFailed, e
		}
		half, e := r.manager.GetArg(root, 1)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpPower, difference, half)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteFull, r.manager.SetRef(out, result)
	}
	if zero, err := r.zero(arg); err != nil {
		return RewriteFailed, err
	} else if zero {
		return RewriteDone, r.manager.SetRef(out, arg)
	}
	if yes, err := r.piMultiple(arg, &coefficient); err != nil {
		return RewriteFailed, err
	} else if yes {
		result, matched, e := r.sinValue(&coefficient)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			return RewriteFull, r.manager.SetRef(out, result)
		}
	}
	if offset, yes, err := r.piOffset(arg, &coefficient); err != nil {
		return RewriteFailed, err
	} else if yes {
		if e := arithPiPeriod(r.q(), &coefficient, &period); e != nil {
			return RewriteFailed, e
		}
		// k_prime outside {0, 1/2, 1, 3/2} falls through to
		// is_2_pi_integer_offset.
		if period.IsZero() || arithQIs(&period, 1, 2) || period.IsOne() || arithQIs(&period, 3, 2) {
			base, e := r.subtractOffset(arg, offset)
			if e != nil {
				return RewriteFailed, e
			}
			var result ast.ASTHandle
			status := Rewrite2
			switch {
			case period.IsZero():
				result, e = r.app(arithOpSin, base)
			case arithQIs(&period, 1, 2):
				result, e = r.app(arithOpCos, base)
			case period.IsOne():
				result, e = r.app(arithOpSin, base)
				if e == nil {
					result, e = r.app(arithOpUMinus, result)
				}
				status = Rewrite3
			case arithQIs(&period, 3, 2):
				result, e = r.app(arithOpCos, base)
				if e == nil {
					result, e = r.app(arithOpUMinus, result)
				}
				status = Rewrite3
			}
			if e != nil {
				return RewriteFailed, e
			}
			return status, r.manager.SetRef(out, result)
		}
	}
	if offset, yes, err := r.integerPiOffset(arg, true); err != nil {
		return RewriteFailed, err
	} else if yes {
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpSin, base)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) rewriteCos(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// rational k, k_prime
	coefficient, period := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(r.q(), &period, &coefficient)) }()
	if yes, child, err := r.unary(arg, arithOpACos); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	if zero, err := r.zero(arg); err != nil {
		return RewriteFailed, err
	} else if zero {
		one, e := r.makeNumeralInt(1, false)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, r.manager.SetRef(out, one)
	}
	if yes, err := r.piMultiple(arg, &coefficient); err != nil {
		return RewriteFailed, err
	} else if yes {
		// k + rational(1, 2)
		half := arithQFrac(1, 2)
		if e := r.q().Add(&coefficient, &half, &coefficient); e != nil {
			return RewriteFailed, e
		}
		result, matched, e := r.sinValue(&coefficient)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			return RewriteFull, r.manager.SetRef(out, result)
		}
	}
	if offset, yes, err := r.piOffset(arg, &coefficient); err != nil {
		return RewriteFailed, err
	} else if yes {
		if e := arithPiPeriod(r.q(), &coefficient, &period); e != nil {
			return RewriteFailed, e
		}
		// k_prime outside {0, 1/2, 1, 3/2} falls through to
		// is_2_pi_integer_offset.
		if period.IsZero() || arithQIs(&period, 1, 2) || period.IsOne() || arithQIs(&period, 3, 2) {
			base, e := r.subtractOffset(arg, offset)
			if e != nil {
				return RewriteFailed, e
			}
			var result ast.ASTHandle
			status := Rewrite2
			switch {
			case period.IsZero():
				result, e = r.app(arithOpCos, base)
			case arithQIs(&period, 1, 2):
				result, e = r.app(arithOpSin, base)
				if e == nil {
					result, e = r.app(arithOpUMinus, result)
				}
				status = Rewrite3
			case period.IsOne():
				result, e = r.app(arithOpCos, base)
				if e == nil {
					result, e = r.app(arithOpUMinus, result)
				}
				status = Rewrite3
			case arithQIs(&period, 3, 2):
				result, e = r.app(arithOpSin, base)
			}
			if e != nil {
				return RewriteFailed, e
			}
			return status, r.manager.SetRef(out, result)
		}
	}
	if offset, yes, err := r.integerPiOffset(arg, true); err != nil {
		return RewriteFailed, err
	} else if yes {
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpCos, base)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) rewriteTan(arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// rational k, k_prime
	coefficient, period := unumeral.NewRational(), unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel2(r.q(), &period, &coefficient)) }()
	if yes, child, err := r.unary(arg, arithOpATan); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	if zero, err := r.zero(arg); err != nil {
		return RewriteFailed, err
	} else if zero {
		return RewriteDone, r.manager.SetRef(out, arg)
	}
	if yes, err := r.piMultiple(arg, &coefficient); err != nil {
		return RewriteFailed, err
	} else if yes {
		numerator, matched, e := r.sinValue(&coefficient)
		if e != nil {
			return RewriteFailed, e
		}
		if matched {
			if zero, e := r.zero(numerator); e != nil {
				return RewriteFailed, e
			} else if zero {
				return RewriteDone, r.manager.SetRef(out, numerator)
			}
			// mk_sin_value(k + rational(1, 2)); period is free here
			half := arithQFrac(1, 2)
			if e = r.q().Add(&coefficient, &half, &period); e != nil {
				return RewriteFailed, e
			}
			denominator, denominatorMatched, e := r.sinValue(&period)
			if e != nil {
				return RewriteFailed, e
			}
			if denominatorMatched {
				if zero, e := r.zero(denominator); e != nil {
					return RewriteFailed, e
				} else if !zero {
					result, e := r.app(arithOpDiv, numerator, denominator)
					if e != nil {
						return RewriteFailed, e
					}
					return RewriteFull, r.manager.SetRef(out, result)
				}
			}
		}
	}
	if offset, yes, err := r.piOffset(arg, &coefficient); err != nil {
		return RewriteFailed, err
	} else if yes {
		// k - floor(k) == 0
		if coefficient.IsInt() {
			base, e := r.subtractOffset(arg, offset)
			if e != nil {
				return RewriteFailed, e
			}
			result, e := r.app(arithOpTan, base)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, result)
		}
	}
	if offset, yes, err := r.integerPiOffset(arg, false); err != nil {
		return RewriteFailed, err
	} else if yes {
		base, e := r.subtractOffset(arg, offset)
		if e != nil {
			return RewriteFailed, e
		}
		result, e := r.app(arithOpTan, base)
		if e != nil {
			return RewriteFailed, e
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	if r.expandTan {
		sine, err := r.app(arithOpSin, arg)
		if err != nil {
			return RewriteFailed, err
		}
		cosine, err := r.app(arithOpCos, arg)
		if err != nil {
			return RewriteFailed, err
		}
		result, err := r.app(arithOpDiv, sine, cosine)
		if err != nil {
			return RewriteFailed, err
		}
		return Rewrite2, r.manager.SetRef(out, result)
	}
	return RewriteFailed, nil
}

// piFraction is (* rational(n, d) pi).
func (r *ArithRewriter) piFraction(n, d int32) (ast.ASTHandle, error) {
	coefficient, err := r.makeNumeralFrac(n, d, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	pi, err := r.pi()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.app(arithOpMul, coefficient, pi)
}

func (r *ArithRewriter) rewriteInverseTrig(kind int32, arg ast.ASTHandle, out ast.ASTRef) (_ RewriteStatus, err error) {
	// rational k
	value := unumeral.NewRational()
	defer func() { err = errors.Join(err, arithQDel(r.q(), &value)) }()
	_, numeral, err := r.numeral(arg, &value)
	if err != nil {
		return RewriteFailed, err
	}
	if numeral {
		if value.IsZero() {
			if kind == arithOpACos {
				h, e := r.piFraction(1, 2)
				if e != nil {
					return RewriteFailed, e
				}
				return Rewrite2, r.manager.SetRef(out, h)
			}
			return RewriteDone, r.manager.SetRef(out, arg)
		}
		less := false
		if kind == arithOpASin || kind == arithOpATan {
			minusOne := arithQFrac(-1, 1)
			cmp, e := r.q().Compare(&value, &minusOne)
			if e != nil {
				return RewriteFailed, e
			}
			less = cmp < 0
		}
		if less {
			if e := r.q().Neg(&value); e != nil {
				return RewriteFailed, e
			}
			positive, e := r.makeNumeral(&value, false)
			if e != nil {
				return RewriteFailed, e
			}
			inner, e := r.app(kind, positive)
			if e != nil {
				return RewriteFailed, e
			}
			result, e := r.app(arithOpUMinus, inner)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, result)
		}
		// the coefficient n/d of pi, when d != 0
		n, d := int32(0), int32(0)
		is := func(n, d int32) bool { return arithQIs(&value, n, d) }
		switch kind {
		case arithOpASin:
			if is(1, 1) {
				n, d = 1, 2
			} else if is(-1, 1) {
				n, d = -1, 2
			} else if is(1, 2) {
				n, d = 1, 6
			} else if is(-1, 2) {
				n, d = -1, 6
			}
		case arithOpACos:
			if is(1, 1) {
				zero, e := r.makeNumeralInt(0, false)
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, r.manager.SetRef(out, zero)
			} else if is(-1, 1) {
				pi, e := r.pi()
				if e != nil {
					return RewriteFailed, e
				}
				return RewriteDone, r.manager.SetRef(out, pi)
			} else if is(1, 2) {
				n, d = 1, 3
			} else if is(-1, 2) {
				n, d = 2, 3
			}
		case arithOpATan:
			if is(1, 1) {
				n, d = 1, 4
			} else if is(-1, 1) {
				n, d = -1, 4
			}
		}
		if d != 0 {
			h, e := r.piFraction(n, d)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, h)
		}
	}
	if kind == arithOpASin || kind == arithOpATan {
		if child, yes, e := r.negativeTerm(arg); e != nil {
			return RewriteFailed, e
		} else if yes {
			inner, e := r.app(kind, child)
			if e != nil {
				return RewriteFailed, e
			}
			negative, e := r.app(arithOpUMinus, inner)
			if e != nil {
				return RewriteFailed, e
			}
			return Rewrite2, r.manager.SetRef(out, negative)
		}
	}
	return RewriteFailed, nil
}

func (r *ArithRewriter) rewriteHyperbolic(kind int32, inverse int32, arg ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if yes, child, err := r.unary(arg, inverse); err != nil {
		return RewriteFailed, err
	} else if yes {
		return RewriteDone, r.manager.SetRef(out, child)
	}
	child, negative, err := r.negativeTerm(arg)
	if err != nil || !negative {
		return RewriteFailed, err
	}
	inner, err := r.app(kind, child)
	if err != nil {
		return RewriteFailed, err
	}
	if kind == arithOpCosh {
		return RewriteDone, r.manager.SetRef(out, inner)
	}
	result, err := r.app(arithOpUMinus, inner)
	if err != nil {
		return RewriteFailed, err
	}
	return Rewrite2, r.manager.SetRef(out, result)
}
