package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:908-1038.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type shiftDirection uint8

const (
	shiftLeft shiftDirection = iota
	shiftLogicalRight
	shiftArithmeticRight
)

func (e *Engine) constantShift(bits []ast.ASTHandle, amount *unumeral.Rational, direction shiftDirection) ([]ast.ASTRef, error) {
	width := len(bits)
	n := width
	q := e.numerals()
	if small, err := q.IsUint64(amount); err != nil {
		return nil, err
	} else if small {
		v, err := q.Uint64(amount)
		if err != nil {
			return nil, err
		}
		if v < uint64(width) {
			n = int(v)
		}
	}
	falsehood, err := e.config.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	shifted := make([]ast.ASTHandle, width)
	for i := 0; i < width; i++ {
		switch direction {
		case shiftLeft:
			if i < n {
				shifted[i] = falsehood
			} else {
				shifted[i] = bits[i-n]
			}
		case shiftLogicalRight:
			if i+n < width {
				shifted[i] = bits[i+n]
			} else {
				shifted[i] = falsehood
			}
		case shiftArithmeticRight:
			if i+n < width {
				shifted[i] = bits[i+n]
			} else {
				shifted[i] = bits[width-1]
			}
		}
	}
	return e.copyBits(shifted)
}

func (e *Engine) shift(bits, amountBits []ast.ASTHandle, direction shiftDirection) (out []ast.ASTRef, result error) {
	if len(bits) == 0 || len(bits) != len(amountBits) {
		return nil, ast.ErrASTDomain
	}
	amount, yes, err := e.numeralValue(amountBits)
	if err != nil {
		return nil, err
	}
	if yes {
		out, err := e.constantShift(bits, &amount, direction)
		if err = errors.Join(err, e.numerals().Del(&amount)); err != nil {
			return nil, errors.Join(err, releaseRefs(e.config.manager, out))
		}
		return out, nil
	}
	if err = e.numerals().Del(&amount); err != nil {
		return nil, err
	}
	m := e.config.manager
	out, result = e.copyBits(bits)
	if result != nil {
		return nil, result
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	falsehood, err := m.MkFalse()
	if err != nil {
		return nil, err
	}
	stage := 0
	shift := 1
	// for (; i < sz; ++i) { checkpoint(); if (shift_i >= sz) break; ... }:
	// the checkpoint also runs on the iteration that ends the loop.
	for stage < len(bits) {
		if err := e.checkpoint(); err != nil {
			return nil, err
		}
		if shift >= len(bits) {
			break
		}
		current, err := e.handles(out)
		if err != nil {
			return nil, err
		}
		next := make([]ast.ASTRef, 0, len(bits))
		for index := range bits {
			shifted := falsehood
			switch direction {
			case shiftLeft:
				if shift <= index {
					shifted = current[index-shift]
				}
			case shiftLogicalRight:
				if shift+index < len(bits) {
					shifted = current[index+shift]
				}
			case shiftArithmeticRight:
				shifted = bits[len(bits)-1]
				if shift+index < len(bits) {
					shifted = current[index+shift]
				}
			}
			ref, err := e.gateRef(ast.ASTOpIte, amountBits[stage], shifted, current[index])
			if err != nil {
				_ = releaseRefs(m, next)
				return nil, err
			}
			next = append(next, ref)
		}
		if err = releaseRefs(m, out); err != nil {
			_ = releaseRefs(m, next)
			return nil, err
		}
		out = next
		stage++
		if shift > len(bits)/2 {
			shift = len(bits)
		} else {
			shift *= 2
		}
	}
	isLarge, err := m.NewRef(falsehood)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&isLarge)) }()
	for ; stage < len(bits); stage++ {
		current, err := m.RefGet(isLarge)
		if err != nil {
			return nil, err
		}
		if err = e.config.simplify(ast.ASTOpOr, isLarge, current, amountBits[stage]); err != nil {
			return nil, err
		}
	}
	large, err := m.RefGet(isLarge)
	if err != nil {
		return nil, err
	}
	current, err := e.handles(out)
	if err != nil {
		return nil, err
	}
	for index := range out {
		oversized := falsehood
		if direction == shiftArithmeticRight {
			oversized = bits[len(bits)-1]
		}
		replacement, err := e.gateRef(ast.ASTOpIte, large, oversized, current[index])
		if err != nil {
			return nil, err
		}
		if err = m.DelRef(&out[index]); err != nil {
			_ = m.DelRef(&replacement)
			return nil, err
		}
		out[index] = replacement
	}
	return out, nil
}

func (e *Engine) MkShiftLeft(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.shift(bits, amount, shiftLeft)
}

func (e *Engine) MkLogicalShiftRight(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.shift(bits, amount, shiftLogicalRight)
}

func (e *Engine) MkArithmeticShiftRight(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.shift(bits, amount, shiftArithmeticRight)
}
