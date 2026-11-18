package rewriter

// Source: src/ast/rewriter/bv_bounds.cpp:68-238,332-450.

import (
	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (b *BVBounds) basicBinary(h ast.ASTHandle, op int32) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	yes, err := b.isOperation(h, b.manager.BasicFamilyID(), op)
	if err != nil || !yes {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	n, err := b.manager.GetNumArgs(h)
	if err != nil || n != 2 {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := b.manager.GetArg(h, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := b.manager.GetArg(h, 1)
	return left, right, err == nil, err
}

// splitRewrittenULE recognizes the extract-and-ULE shape emitted by the BV
// rewriter for a shortened unsigned comparison.
func (b *BVBounds) splitRewrittenULE(h ast.ASTHandle) (ast.ASTHandle, *unumeral.Rational, bool, error) {
	equality, ule, yes, err := b.basicBinary(h, ast.ASTOpAnd)
	if err != nil || !yes {
		return ast.ASTHandle{}, nil, false, err
	}
	eqLeft, eqRight, yes, err := b.basicBinary(equality, ast.ASTOpEq)
	if err != nil || !yes {
		return ast.ASTHandle{}, nil, false, err
	}
	var uleLeft, uleRight ast.ASTHandle
	yes, err = b.manager.BitVectorULEOperands(ule, &uleLeft, &uleRight)
	if err != nil || !yes {
		return ast.ASTHandle{}, nil, false, err
	}
	var eqBase ast.ASTHandle
	var eqLow, eqHigh uint32
	yes, err = b.manager.BitVectorExtractValue(eqLeft, &eqLow, &eqHigh, &eqBase)
	if err != nil || !yes {
		return ast.ASTHandle{}, nil, false, err
	}
	baseWidth, err := b.manager.BitVectorSize(eqBase)
	if err != nil || eqHigh != baseWidth-1 {
		return ast.ASTHandle{}, nil, false, err
	}
	eqValue, eqWidth, yes, err := b.numeral(eqRight)
	if err != nil || !yes || !eqValue.IsZero() {
		return ast.ASTHandle{}, nil, false, err
	}
	var uleBase ast.ASTHandle
	var uleLow, uleHigh uint32
	yes, err = b.manager.BitVectorExtractValue(uleLeft, &uleLow, &uleHigh, &uleBase)
	if err != nil || !yes || uleBase != eqBase || uleHigh+1 != eqLow || uleLow != 0 {
		return ast.ASTHandle{}, nil, false, err
	}
	value, uleWidth, yes, err := b.numeral(uleRight)
	if err != nil || !yes || baseWidth != uleWidth+eqWidth {
		return ast.ASTHandle{}, nil, false, err
	}
	return eqBase, value, true, nil
}

func (b *BVBounds) signed(value *unumeral.Rational, width uint32) *unumeral.Rational {
	out := b.cpy(value)
	if width != 0 && b.cmp(out, b.pow(width-1)) >= 0 {
		b.num.sub(out, out, b.pow(width))
	}
	return out
}

func (b *BVBounds) convert(h ast.ASTHandle, intervals *[]bvBoundInterval, negated bool) (bvConversion, error) {
	prefix := "new constraint: "
	if negated {
		prefix += "~"
	}
	if err := b.traceValue("convert", 103, prefix, h, ""); err != nil {
		return bvUndefined, err
	}
	if yes, arg, err := b.manager.IsNot(h); err != nil {
		return bvUndefined, err
	} else if yes {
		negated, h = !negated, arg
	}
	if value, constant, yes, err := b.splitRewrittenULE(h); err != nil {
		return bvUndefined, err
	} else if yes {
		if bound, e := b.toBound(value); e != nil {
			return bvUndefined, e
		} else if bound {
			return b.record(value, b.z(0), constant, negated, intervals)
		}
	}
	// (= ((_ extract (width-1) low) x) 0) bounds the high portion of x.
	if left, right, yes, err := b.basicBinary(h, ast.ASTOpEq); err != nil {
		return bvUndefined, err
	} else if yes {
		if zero, _, numeral, e := b.numeral(right); e != nil {
			return bvUndefined, e
		} else if numeral && zero.IsZero() {
			var base ast.ASTHandle
			var low, high uint32
			if extract, e := b.manager.BitVectorExtractValue(left, &low, &high, &base); e != nil {
				return bvUndefined, e
			} else if extract {
				width, e := b.manager.BitVectorSize(base)
				if e != nil {
					return bvUndefined, e
				}
				if high+1 == width {
					return b.record(base, b.z(0), b.subR(b.pow(low), b.z(1)), negated, intervals)
				}
			}
		}
	}
	var left, right ast.ASTHandle
	if yes, err := b.manager.BitVectorULEOperands(h, &left, &right); err != nil {
		return bvUndefined, err
	} else if yes {
		width, err := b.manager.BitVectorSize(left)
		if err != nil {
			return bvUndefined, err
		}
		leftBound, err := b.toBound(left)
		if err != nil {
			return bvUndefined, err
		}
		rightBound, err := b.toBound(right)
		if err != nil {
			return bvUndefined, err
		}
		lv, lw, ln, err := b.numeral(left)
		if err != nil {
			return bvUndefined, err
		}
		rv, rw, rn, err := b.numeral(right)
		if err != nil {
			return bvUndefined, err
		}
		if leftBound && rn && rw == width {
			return b.record(left, b.z(0), rv, negated, intervals)
		}
		if rightBound && ln && lw == width {
			return b.record(right, lv, b.max(width), negated, intervals)
		}
		var a, x ast.ASTHandle
		if add, e := b.manager.BitVectorAddOperands(left, &a, &x); e != nil {
			return bvUndefined, e
		} else if add {
			if av, aw, number, e := b.numeral(a); e != nil {
				return bvUndefined, e
			} else if number && aw == width {
				if bound, e := b.toBound(x); e != nil {
					return bvUndefined, e
				} else if bound && x == right {
					if av.IsZero() {
						if negated {
							return bvUnsat, nil
						}
						return bvConverted, nil
					}
					return b.record(right, b.subR(b.pow(width), av), b.max(width), negated, intervals)
				}
			}
		}
		if add, e := b.manager.BitVectorAddOperands(right, &a, &x); e != nil {
			return bvUndefined, e
		} else if add {
			av, aw, an, e := b.numeral(a)
			if e != nil {
				return bvUndefined, e
			}
			bound, e := b.toBound(x)
			if e != nil {
				return bvUndefined, e
			}
			if an && aw == width && bound && ln && lw == width {
				modulus, one := b.pow(width), b.z(1)
				if lv.IsZero() {
					if negated {
						return bvUnsat, nil
					}
					return bvConverted, nil
				}
				if b.cmp(lv, av) < 0 {
					lo := b.subR(modulus, av)
					hi := b.subR(b.addR(modulus, lv), b.addR(av, one))
					if b.cmp(lo, hi) <= 0 {
						return b.record(x, lo, hi, !negated, intervals)
					}
					if negated {
						return bvUnsat, nil
					}
					return bvConverted, nil
				}
				lo := b.subR(lv, av)
				hi := b.subR(b.subR(modulus, av), one)
				if b.cmp(lo, hi) <= 0 {
					return b.record(x, lo, hi, negated, intervals)
				}
				if negated {
					return bvConverted, nil
				}
				return bvUnsat, nil
			}
		}
		if add, e := b.manager.BitVectorAddOperands(left, &a, &x); e != nil {
			return bvUndefined, e
		} else if add {
			av, aw, an, e := b.numeral(a)
			if e != nil {
				return bvUndefined, e
			}
			bound, e := b.toBound(x)
			if e != nil {
				return bvUndefined, e
			}
			if an && aw == width && bound && rn && rw == width && av.IsPos() && rv.IsPos() {
				modulus, one := b.pow(width), b.z(1)
				if b.cmp(av, rv) <= 0 {
					lo := b.addR(b.subR(rv, av), one)
					hi := b.subR(b.subR(modulus, av), one)
					if b.cmp(lo, hi) <= 0 {
						return b.record(x, lo, hi, !negated, intervals)
					}
					if negated {
						return bvUnsat, nil
					}
					return bvConverted, nil
				}
				lo := b.subR(modulus, av)
				hi := b.addR(lo, rv)
				return b.record(x, lo, hi, negated, intervals)
			}
		}
		v1, c1, ok1, e := b.constantAdd(width, left)
		if e != nil {
			return bvUndefined, e
		}
		v2, c2, ok2, e := b.constantAdd(width, right)
		if e != nil {
			return bvUndefined, e
		}
		if ok1 && ok2 && v1 == v2 {
			if b.cmp(c1, c2) == 0 {
				if negated {
					return bvUnsat, nil
				}
				return bvConverted, nil
			}
			modulus, one := b.pow(width), b.z(1)
			if b.cmp(c1, c2) < 0 {
				return b.record(v1, b.subR(modulus, c2), b.subR(b.subR(modulus, c1), one), !negated, intervals)
			}
			return b.record(v1, b.subR(modulus, c1), b.subR(b.subR(modulus, c2), one), negated, intervals)
		}
	}
	if yes, err := b.manager.BitVectorSLEOperands(h, &left, &right); err != nil {
		return bvUndefined, err
	} else if yes {
		width, err := b.manager.BitVectorSize(left)
		if err != nil {
			return bvUndefined, err
		}
		if bound, e := b.toBound(left); e != nil {
			return bvUndefined, e
		} else if value, w, numeral, e := b.numeral(right); e != nil {
			return bvUndefined, e
		} else if bound && numeral && w == width {
			minimum := b.negR(b.pow(width - 1))
			return b.convertSigned(left, minimum, b.signed(value, width), negated, intervals)
		}
		if bound, e := b.toBound(right); e != nil {
			return bvUndefined, e
		} else if value, w, numeral, e := b.numeral(left); e != nil {
			return bvUndefined, e
		} else if bound && numeral && w == width {
			maximum := b.subR(b.pow(width-1), b.z(1))
			return b.convertSigned(right, b.signed(value, width), maximum, negated, intervals)
		}
	}
	return bvUndefined, nil
}

// AddConstraint adds every interval recognized from one source constraint.
func (b *BVBounds) AddConstraint(h ast.ASTHandle) (_ bool, resultErr error) {
	defer b.numeralFailure(&resultErr)
	if err := b.open(); err != nil {
		return false, err
	}
	if !b.okay {
		return false, nil
	}
	if err := b.traceValue("add_constraint", 333, "new constraint", h, ""); err != nil {
		return false, err
	}
	negated := false
	if yes, arg, err := b.manager.IsNot(h); err != nil {
		return false, err
	} else if yes {
		negated, h = true, arg
	}
	var left, right ast.ASTHandle
	if yes, err := b.manager.BitVectorULEOperands(h, &left, &right); err != nil {
		return false, err
	} else if yes {
		width, err := b.manager.BitVectorSize(left)
		if err != nil {
			return false, err
		}
		leftBound, err := b.toBound(left)
		if err != nil {
			return false, err
		}
		rightBound, err := b.toBound(right)
		if err != nil {
			return false, err
		}
		lv, lw, ln, err := b.numeral(left)
		if err != nil {
			return false, err
		}
		rv, rw, rn, err := b.numeral(right)
		if err != nil {
			return false, err
		}
		if leftBound && rn && rw == width {
			return b.AddBoundUnsigned(left, b.z(0), rv, negated)
		}
		if rightBound && ln && lw == width {
			return b.AddBoundUnsigned(right, lv, b.max(width), negated)
		}
		var coefficient, value ast.ASTHandle
		if add, e := b.manager.BitVectorAddOperands(left, &coefficient, &value); e != nil {
			return false, e
		} else if add {
			if c, w, numeral, e := b.numeral(coefficient); e != nil {
				return false, e
			} else if bound, e := b.toBound(value); e != nil {
				return false, e
			} else if numeral && w == width && bound && value == right {
				if !c.IsPos() {
					return b.okay, nil
				}
				return b.AddBoundUnsigned(right, b.subR(b.pow(width), c), b.max(width), negated)
			}
		}
		if add, e := b.manager.BitVectorAddOperands(right, &coefficient, &value); e != nil {
			return false, e
		} else if add {
			c, w, numeral, e := b.numeral(coefficient)
			if e != nil {
				return false, e
			}
			bound, e := b.toBound(value)
			if e != nil {
				return false, e
			}
			if numeral && w == width && bound && ln && lw == width && c.IsPos() && lv.IsPos() {
				modulus, one := b.pow(width), b.z(1)
				if b.cmp(lv, c) < 0 {
					lo := b.subR(modulus, c)
					hi := b.subR(b.addR(modulus, lv), b.addR(c, one))
					if b.cmp(lo, hi) <= 0 {
						return b.AddBoundUnsigned(value, lo, hi, !negated)
					}
					return b.okay, nil
				}
				lo := b.subR(lv, c)
				hi := b.subR(b.subR(modulus, c), one)
				if b.cmp(lo, hi) <= 0 {
					return b.AddBoundUnsigned(value, lo, hi, negated)
				}
				return b.okay, nil
			}
		}
		if add, e := b.manager.BitVectorAddOperands(left, &coefficient, &value); e != nil {
			return false, e
		} else if add {
			c, w, numeral, e := b.numeral(coefficient)
			if e != nil {
				return false, e
			}
			bound, e := b.toBound(value)
			if e != nil {
				return false, e
			}
			if numeral && w == width && bound && rn && rw == width && c.IsPos() && rv.IsPos() {
				modulus, one := b.pow(width), b.z(1)
				if b.cmp(c, rv) <= 0 {
					lo := b.addR(b.subR(rv, c), one)
					hi := b.subR(b.subR(modulus, c), one)
					if b.cmp(lo, hi) <= 0 {
						return b.AddBoundUnsigned(value, lo, hi, !negated)
					}
					return b.okay, nil
				}
				lo := b.subR(modulus, c)
				return b.AddBoundUnsigned(value, lo, b.addR(lo, rv), negated)
			}
		}
		v1, c1, ok1, err := b.constantAdd(width, left)
		if err != nil {
			return false, err
		}
		v2, c2, ok2, err := b.constantAdd(width, right)
		if err != nil {
			return false, err
		}
		if ok1 && ok2 && v1 == v2 {
			if b.cmp(c1, c2) == 0 {
				return b.okay, nil
			}
			modulus, one := b.pow(width), b.z(1)
			if b.cmp(c1, c2) < 0 {
				return b.AddBoundUnsigned(v1, b.subR(modulus, c2), b.subR(b.subR(modulus, c1), one), !negated)
			}
			return b.AddBoundUnsigned(v1, b.subR(modulus, c1), b.subR(b.subR(modulus, c2), one), negated)
		}
	}
	if yes, err := b.manager.BitVectorSLEOperands(h, &left, &right); err != nil {
		return false, err
	} else if yes {
		width, err := b.manager.BitVectorSize(left)
		if err != nil {
			return false, err
		}
		if bound, e := b.toBound(left); e != nil {
			return false, e
		} else if value, w, numeral, e := b.numeral(right); e != nil {
			return false, e
		} else if bound && numeral && w == width {
			minimum := b.negR(b.pow(width - 1))
			return b.AddBoundSigned(left, minimum, b.signed(value, width), negated)
		}
		if bound, e := b.toBound(right); e != nil {
			return false, e
		} else if value, w, numeral, e := b.numeral(left); e != nil {
			return false, e
		} else if bound && numeral && w == width {
			maximum := b.subR(b.pow(width-1), b.z(1))
			return b.AddBoundSigned(right, b.signed(value, width), maximum, negated)
		}
	}
	return b.okay, nil
}
