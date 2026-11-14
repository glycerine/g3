package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp mk_bvsmul_no_overflow (upstream
// z3 4.17.0 algorithm, backported to the fork; covers bvsmul_noudfl too)
// and mk_bvumul_no_overflow.

import "github.com/glycerine/g3/pkg/ast"

type BVMultiplyBound uint8

const (
	BVSignedNoOverflow BVMultiplyBound = iota
	BVUnsignedNoOverflow
	BVSignedNoUnderflow
)

func (p *BVRewriter) RewriteMultiplyBound(args ast.AppArguments, out ast.ASTRef, mode BVMultiplyBound) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	if mode > BVSignedNoUnderflow {
		return RewriteFailed, ast.ErrASTDomain
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	if ast.ASTSourceDebug && args.Count() != 2 {
		return st, ast.ErrASTDomain
	}
	m := p.manager
	n, e := ast.NewBitVectorMultiplyBoundNumerals(m)
	p.mulBound = n
	if e != nil {
		return st, e
	}
	var width uint32
	first, e := args.At(0)
	if e != nil {
		return st, e
	}
	num1, e := n.Read(0, first, &width)
	if e != nil {
		return st, e
	}
	second, e := args.At(1)
	if e != nil {
		return st, e
	}
	num2, e := n.Read(1, second, &width)
	if e != nil {
		return st, e
	}
	result := false
	matched := false
	fastOperand := func(i int) (bool, error) {
		// At signed width one, the bit pattern 1 means -1, not +1.
		if mode != BVUnsignedNoOverflow && width == 1 {
			return n.IsZero(i)
		}
		return n.ZeroOrOne(i)
	}
	if num1 {
		matched, e = fastOperand(0)
		if e != nil {
			return st, e
		}
	}
	if !matched && num2 {
		matched, e = fastOperand(1)
		if e != nil {
			return st, e
		}
	}
	if matched {
		result = true
	} else if num1 && num2 {
		matched = true
		// Upstream z3 (4.17.0) mk_bvsmul_no_overflow, backported to the
		// fork (src/ast/rewriter/bv_rewriter.cpp): magnitudes with sign
		// flags. z3 4.7.1 folded the unsigned product (#xa * #x3 -> false).
		if mode != BVUnsignedNoOverflow {
			if e = n.Power(2, width); e != nil {
				return st, e
			}
			if e = n.Power(3, width-1); e != nil {
				return st, e
			}
			sign0, e := n.Magnitude(0)
			if e != nil {
				return st, e
			}
			sign1, e := n.Magnitude(1)
			if e != nil {
				return st, e
			}
			if e = n.Product(4); e != nil {
				return st, e
			}
			if mode == BVSignedNoOverflow {
				// sign0 != sign1 || r < lim
				below, e := n.Compare(4, 3, false)
				if e != nil {
					return st, e
				}
				result = sign0 != sign1 || below
			} else {
				// sign0 == sign1 || r <= lim, i.e. !(lim < r)
				limBelow, e := n.Compare(3, 4, false)
				if e != nil {
					return st, e
				}
				result = sign0 == sign1 || !limBelow
			}
		} else {
			if e = n.Product(2); e != nil {
				return st, e
			}
			if e = n.Power(3, width); e != nil {
				return st, e
			}
			result, e = n.Compare(2, 3, false)
		}
		if e != nil {
			return st, e
		}
	}
	if matched {
		var h ast.ASTHandle
		if result {
			h, e = m.MkTrue()
		} else {
			h, e = m.MkFalse()
		}
		if e != nil {
			return st, e
		}
		st = RewriteDone
		if err = m.SetRef(out, h); err != nil {
			return
		}
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.mulBound = nil
	return
}
