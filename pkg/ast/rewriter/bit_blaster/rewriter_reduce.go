package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:211-321.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type bitVectorUnary func([]ast.ASTHandle) ([]ast.ASTRef, error)
type bitVectorBinary func([]ast.ASTHandle, []ast.ASTHandle) ([]ast.ASTRef, error)
type bitVectorPredicate func([]ast.ASTHandle, []ast.ASTHandle, ast.ASTRef) error

// resetBits is expr_ref_vector::reset: it releases forward.
func (r *BitBlasterRewriter) resetBits(v *[]ast.ASTRef) error {
	err := releaseRefs(r.manager, *v)
	*v = nil
	return err
}

// MK_UNARY_REDUCE / MK_PARAMETRIC_UNARY_REDUCE.
func (r *BitBlasterRewriter) reduceUnary(term ast.ASTHandle, out ast.ASTRef, operation bitVectorUnary) error {
	if err := r.resetBits(&r.in1); err != nil {
		return err
	}
	bits, err := r.getBits(term)
	r.in1 = bits
	if err != nil {
		return err
	}
	if err = r.resetBits(&r.outBits); err != nil {
		return err
	}
	handles, err := r.blaster.handles(r.in1)
	if err != nil {
		return err
	}
	r.outBits, err = operation(handles)
	if err != nil {
		return err
	}
	return r.setBitVector(r.outBits, out)
}

// MK_BIN_REDUCE.
func (r *BitBlasterRewriter) reduceBinary(left, right ast.ASTHandle, out ast.ASTRef, operation bitVectorBinary) error {
	if err := errors.Join(r.resetBits(&r.in1), r.resetBits(&r.in2)); err != nil {
		return err
	}
	bits, err := r.getBits(left)
	r.in1 = bits
	if err != nil {
		return err
	}
	bits, err = r.getBits(right)
	r.in2 = bits
	if err != nil {
		return err
	}
	if err = r.resetBits(&r.outBits); err != nil {
		return err
	}
	leftBits, err := r.blaster.handles(r.in1)
	if err != nil {
		return err
	}
	rightBits, err := r.blaster.handles(r.in2)
	if err != nil {
		return err
	}
	r.outBits, err = operation(leftBits, rightBits)
	if err != nil {
		return err
	}
	return r.setBitVector(r.outBits, out)
}

// MK_BIN_PRED_REDUCE: m_out is not touched.
func (r *BitBlasterRewriter) reducePredicate(left, right ast.ASTHandle, out ast.ASTRef, operation bitVectorPredicate) error {
	if err := errors.Join(r.resetBits(&r.in1), r.resetBits(&r.in2)); err != nil {
		return err
	}
	bits, err := r.getBits(left)
	r.in1 = bits
	if err != nil {
		return err
	}
	bits, err = r.getBits(right)
	r.in2 = bits
	if err != nil {
		return err
	}
	leftBits, err := r.blaster.handles(r.in1)
	if err != nil {
		return err
	}
	rightBits, err := r.blaster.handles(r.in2)
	if err != nil {
		return err
	}
	return operation(leftBits, rightBits, out)
}

// reduce_redand / reduce_redor (MK_UNARY_REDUCE).
func (r *BitBlasterRewriter) reduceReduction(term ast.ASTHandle, out ast.ASTRef, and bool) error {
	return r.reduceUnary(term, out, func(handles []ast.ASTHandle) ([]ast.ASTRef, error) {
		bit, err := r.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		if and {
			err = r.blaster.MkReductionAnd(handles, bit)
		} else {
			err = r.blaster.MkReductionOr(handles, bit)
		}
		if err != nil {
			return nil, errors.Join(err, r.manager.DelRef(&bit))
		}
		return []ast.ASTRef{bit}, nil
	})
}

func (r *BitBlasterRewriter) reduceAC(args ast.AppArguments, out ast.ASTRef, operation bitVectorBinary) (result error) {
	if args.Count() == 0 {
		return ast.ErrASTDomain
	}
	first, err := args.At(0)
	if err != nil {
		return err
	}
	current, err := r.manager.NewRef(first)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.manager.DelRef(&current)) }()
	for i := uint32(1); i < args.Count(); i++ {
		right, err := args.At(i)
		if err != nil {
			return err
		}
		left, err := r.manager.RefGet(current)
		if err != nil {
			return err
		}
		next, err := r.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		if err = r.reduceBinary(left, right, next, operation); err != nil {
			_ = r.manager.DelRef(&next)
			return err
		}
		if err = r.manager.DelRef(&current); err != nil {
			_ = r.manager.DelRef(&next)
			return err
		}
		current = next
	}
	return r.manager.AssignRef(out, current)
}

func (r *BitBlasterRewriter) reduceITE(condition, thenTerm, elseTerm ast.ASTHandle, out ast.ASTRef) error {
	if err := errors.Join(r.resetBits(&r.in1), r.resetBits(&r.in2)); err != nil {
		return err
	}
	bits, err := r.getBits(thenTerm)
	r.in1 = bits
	if err != nil {
		return err
	}
	bits, err = r.getBits(elseTerm)
	r.in2 = bits
	if err != nil {
		return err
	}
	if err = r.resetBits(&r.outBits); err != nil {
		return err
	}
	thenBits, err := r.blaster.handles(r.in1)
	if err != nil {
		return err
	}
	elseBits, err := r.blaster.handles(r.in2)
	if err != nil {
		return err
	}
	r.outBits, err = r.blaster.MkMultiplexer(condition, thenBits, elseBits)
	if err != nil {
		return err
	}
	return r.setBitVector(r.outBits, out)
}

// appendBits is m_out.append / push_back: the vector takes its own pins.
func (r *BitBlasterRewriter) appendBits(refs []ast.ASTRef) error {
	for i := range refs {
		h, err := r.manager.RefGet(refs[i])
		if err != nil {
			return err
		}
		ref, err := r.manager.NewRef(h)
		if err != nil {
			return err
		}
		r.outBits = append(r.outBits, ref)
	}
	return nil
}

func (r *BitBlasterRewriter) reduceConcat(args ast.AppArguments, out ast.ASTRef) error {
	if err := r.resetBits(&r.outBits); err != nil {
		return err
	}
	for i := args.Count(); i > 0; {
		i--
		arg, err := args.At(i)
		if err != nil {
			return err
		}
		if err = r.resetBits(&r.in1); err != nil {
			return err
		}
		bits, err := r.getBits(arg)
		r.in1 = bits
		if err != nil {
			return err
		}
		if err = r.appendBits(r.in1); err != nil {
			return err
		}
	}
	return r.setBitVector(r.outBits, out)
}

func (r *BitBlasterRewriter) reduceExtract(low, high uint32, term ast.ASTHandle, out ast.ASTRef) error {
	if err := r.resetBits(&r.in1); err != nil {
		return err
	}
	bits, err := r.getBits(term)
	r.in1 = bits
	if err != nil {
		return err
	}
	if err = r.resetBits(&r.outBits); err != nil {
		return err
	}
	if low > high || uint64(high) >= uint64(len(r.in1)) {
		return ast.ErrASTDomain
	}
	if err = r.appendBits(r.in1[low : high+1]); err != nil {
		return err
	}
	return r.setBitVector(r.outBits, out)
}

func (r *BitBlasterRewriter) reduceSignExtend(extra uint32, term ast.ASTHandle, out ast.ASTRef) error {
	return r.reduceUnary(term, out, func(handles []ast.ASTHandle) ([]ast.ASTRef, error) {
		return r.blaster.MkSignExtend(handles, int(extra))
	})
}
