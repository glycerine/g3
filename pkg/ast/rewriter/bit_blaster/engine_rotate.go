package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:1040-1086.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (e *Engine) extendedRotate(left bool, bits, amountBits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(bits) == 0 || len(bits) != len(amountBits) {
		return nil, ast.ErrASTDomain
	}
	// numeral k; if (is_numeral(sz, b_bits, k)) { r = k % sz; ... }
	q := e.numerals()
	amount, yes, err := e.numeralValue(amountBits)
	if err != nil {
		return nil, err
	}
	if yes {
		modulus := unumeral.NewRational()
		var n uint64
		err = errors.Join(q.SetUint64(&modulus, uint64(len(bits))), q.Mod(&amount, &modulus, &amount))
		if err == nil {
			n, err = q.Uint64(&amount)
		}
		if err = errors.Join(err, q.Del(&modulus), q.Del(&amount)); err != nil {
			return nil, err
		}
		if left {
			return e.MkRotateLeft(bits, n)
		}
		return e.MkRotateRight(bits, n)
	}
	if err = q.Del(&amount); err != nil {
		return nil, err
	}
	m := e.config.manager
	sz := unumeral.Rational{Num: unumeral.NewInt(int32(len(bits))), Den: unumeral.NewInt(1)}
	widthBits, err := e.NumToBits(&sz, len(bits))
	if err != nil {
		return nil, err
	}
	var masked, equalities []ast.ASTRef
	defer func() {
		result = errors.Join(result, releaseGroups(m, widthBits, masked, equalities))
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	widthHandles, err := e.handles(widthBits)
	if err != nil {
		return nil, err
	}
	masked, err = e.MkUnsignedRem(amountBits, widthHandles)
	if err != nil {
		return nil, err
	}
	maskedHandles, err := e.handles(masked)
	if err != nil {
		return nil, err
	}
	equalities, err = e.MkEqs(maskedHandles)
	if err != nil {
		return nil, err
	}
	equalityHandles, err := e.handles(equalities)
	if err != nil {
		return nil, err
	}
	for index := range bits {
		if err := e.checkpoint(); err != nil {
			return nil, err
		}
		current, err := m.NewRef(bits[index])
		if err != nil {
			return nil, err
		}
		for amount := 1; amount < len(bits); amount++ {
			source := (index + amount) % len(bits)
			if left {
				source = (len(bits) + index - amount) % len(bits)
			}
			currentHandle, err := m.RefGet(current)
			if err != nil {
				_ = m.DelRef(&current)
				return nil, err
			}
			next, err := e.gateRef(ast.ASTOpIte, equalityHandles[amount], bits[source], currentHandle)
			if err != nil {
				_ = m.DelRef(&current)
				return nil, err
			}
			if err = m.DelRef(&current); err != nil {
				_ = m.DelRef(&next)
				return nil, err
			}
			current = next
		}
		out = append(out, current)
	}
	return out, nil
}

func (e *Engine) MkExtendedRotateLeft(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.extendedRotate(true, bits, amount)
}

func (e *Engine) MkExtendedRotateRight(bits, amount []ast.ASTHandle) ([]ast.ASTRef, error) {
	return e.extendedRotate(false, bits, amount)
}
