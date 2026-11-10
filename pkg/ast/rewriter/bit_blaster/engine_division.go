package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:439-509.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *Engine) MkUnsignedDivRem(a, b []ast.ASTHandle) (quotient, remainder []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, nil, ast.ErrASTDomain
	}
	m := e.config.manager
	width := len(a)
	quotient = make([]ast.ASTRef, width)
	partial := make([]ast.ASTRef, 0, width)
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(m, quotient), releaseRefs(m, partial))
			quotient, remainder = nil, nil
		}
	}()
	most, err := m.NewRef(a[width-1])
	if err != nil {
		return nil, nil, err
	}
	partial = append(partial, most)
	falsehood, err := m.MkFalse()
	if err != nil {
		return nil, nil, err
	}
	for len(partial) < width {
		ref, err := m.NewRef(falsehood)
		if err != nil {
			return nil, nil, err
		}
		partial = append(partial, ref)
	}

	for stage := 0; stage < width; stage++ {
		if err := e.checkpoint(); err != nil {
			return nil, nil, err
		}
		partialBits, err := e.handles(partial)
		if err != nil {
			return nil, nil, err
		}
		difference, carry, err := e.MkSubtracter(partialBits, b)
		if err != nil {
			return nil, nil, err
		}
		quotient[width-stage-1] = carry
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			_ = releaseRefs(m, difference)
			return nil, nil, err
		}
		differenceBits, err := e.handles(difference)
		if err != nil {
			_ = releaseRefs(m, difference)
			return nil, nil, err
		}
		// update p in place: p.set(j, mk_ite(q, t[j-1], p[j-1])) for j from
		// sz-1 down to 1, then p.set(0, a[sz-i-2]); the last stage sets
		// p[j] = mk_ite(q, t[j], p[j]) upward. Each set releases the old bit.
		set := func(j int, h ast.ASTHandle) error {
			return m.SetRef(partial[j], h)
		}
		ite := func(j int, then, otherwise ast.ASTHandle) error {
			ref, err := e.gateRef(ast.ASTOpIte, carryHandle, then, otherwise)
			if err != nil {
				return err
			}
			h, err := m.RefGet(ref)
			if err == nil {
				err = set(j, h)
			}
			return errors.Join(err, m.DelRef(&ref))
		}
		if stage+1 < width {
			for j := width - 1; j > 0; j-- {
				if err = ite(j, differenceBits[j-1], partialBits[j-1]); err != nil {
					_ = releaseRefs(m, difference)
					return nil, nil, err
				}
			}
			if err = set(0, a[width-stage-2]); err != nil {
				_ = releaseRefs(m, difference)
				return nil, nil, err
			}
		} else {
			for j := 0; j < width; j++ {
				if err = ite(j, differenceBits[j], partialBits[j]); err != nil {
					_ = releaseRefs(m, difference)
					return nil, nil, err
				}
			}
		}
		// t.reset() at the next stage (or ~t on return).
		if err = releaseRefs(m, difference); err != nil {
			return nil, nil, err
		}
	}
	remainder = partial
	partial = nil
	return quotient, remainder, nil
}

func (e *Engine) MkUnsignedDiv(a, b []ast.ASTHandle) (quotient []ast.ASTRef, result error) {
	quotient, remainder, err := e.MkUnsignedDivRem(a, b)
	if err != nil {
		return nil, err
	}
	if err = releaseRefs(e.config.manager, remainder); err != nil {
		return nil, errors.Join(err, releaseRefs(e.config.manager, quotient))
	}
	return quotient, nil
}

func (e *Engine) MkUnsignedRem(a, b []ast.ASTHandle) (remainder []ast.ASTRef, result error) {
	quotient, remainder, err := e.MkUnsignedDivRem(a, b)
	if err != nil {
		return nil, err
	}
	if err = releaseRefs(e.config.manager, quotient); err != nil {
		return nil, errors.Join(err, releaseRefs(e.config.manager, remainder))
	}
	return remainder, nil
}
