package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_tpl.h:22-77;
// src/ast/rewriter/bit_blaster/bit_blaster_tpl_def.h:27-157.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Engine is the concrete AST specialization of bit_blaster_tpl. Inputs borrow
// their handles. Returned reference slices own their expressions and must be
// released by the caller.
type Engine struct {
	config      *Config
	maxMemory   uint64
	cooperation *cooperate.Cooperation
}

func NewEngine(config *Config) (*Engine, error) {
	if config == nil || config.manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &Engine{config: config, maxMemory: ^uint64(0)}, nil
}

func (e *Engine) SetMaxMemoryMB(megabytes uint32) error {
	if e == nil || e.config == nil || e.config.manager == nil {
		return ast.ErrASTHandle
	}
	e.maxMemory = memory.MegabytesToBytes(megabytes)
	return nil
}

func (e *Engine) checkpoint() error {
	if e == nil || e.config == nil || e.config.manager == nil {
		return ast.ErrASTHandle
	}
	runtime := e.config.manager.NumeralService().RuntimeOwner()
	used, err := runtime.AllocationSize()
	if err != nil {
		return err
	}
	if used > e.maxMemory {
		return rewriter.NewRewriterException(messages.MaxMemory)
	}
	canceled, err := e.config.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return rewriter.NewRewriterException(e.config.manager.ResourceLimit().CancelMessage())
	}
	if e.cooperation != nil {
		return e.cooperation.Cooperate("bit-blaster")
	}
	return nil
}

func (e *Engine) BindCooperation(owner *cooperate.Cooperation) error {
	if e == nil || e.config == nil || owner == nil {
		return ast.ErrASTHandle
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	e.cooperation = owner
	return nil
}

func releaseRefs(m *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := range refs {
		if refs[i] != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}
	return result
}

func (e *Engine) handles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		h, err := e.config.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
		result[i] = h
	}
	return result, nil
}

// numerals is the rational manager of the engine's numerals (numeral =
// rational, the AST manager's NumeralService().Manager).
func (e *Engine) numerals() *unumeral.RationalManager {
	return e.config.manager.NumeralService().Manager
}

// Numeral is is_numeral(sz, bits, r): when every little-endian bit is true
// or false, out (a rational of the AST manager's NumeralService().Manager)
// receives their value, sum of power(i) over the true bits.
func (e *Engine) Numeral(bits []ast.ASTHandle, out *unumeral.Rational) (bool, error) {
	for _, bit := range bits {
		if !e.config.manager.IsTrue(bit) && !e.config.manager.IsFalse(bit) {
			return false, nil
		}
	}
	q := e.numerals()
	two := unumeral.Rational{Num: unumeral.NewInt(2), Den: unumeral.NewInt(1)}
	if err := q.SetInt32(out, 0); err != nil {
		return false, err
	}
	for i := len(bits) - 1; i >= 0; i-- {
		if err := q.Mul(out, &two, out); err != nil {
			return false, err
		}
		if e.config.manager.IsTrue(bits[i]) {
			if err := q.Inc(out); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// numeralValue is Numeral into a fresh rational the caller releases.
func (e *Engine) numeralValue(bits []ast.ASTHandle) (unumeral.Rational, bool, error) {
	out := unumeral.NewRational()
	ok, err := e.Numeral(bits, &out)
	if err != nil {
		return out, false, errors.Join(err, e.numerals().Del(&out))
	}
	return out, ok, nil
}

func (e *Engine) IsMinusOne(bits []ast.ASTHandle) bool {
	for _, bit := range bits {
		if !e.config.manager.IsTrue(bit) {
			return false
		}
	}
	return true
}

// NumToBits returns the low width bits of a nonnegative integer (a rational
// of the AST manager's NumeralService().Manager), least significant bit
// first, matching _num2bits in the source.
func (e *Engine) NumToBits(value *unumeral.Rational, width int) (out []ast.ASTRef, result error) {
	if value == nil || value.IsNeg() || width < 0 {
		return nil, ast.ErrASTDomain
	}
	// rational aux = v; rational two(2);
	q := e.numerals()
	aux, bit := unumeral.NewRational(), unumeral.NewRational()
	two := unumeral.Rational{Num: unumeral.NewInt(2), Den: unumeral.NewInt(1)}
	defer func() {
		result = errors.Join(result, q.Del(&aux), q.Del(&bit))
		if result != nil {
			result = errors.Join(result, releaseRefs(e.config.manager, out))
			out = nil
		}
	}()
	if err := q.Copy(&aux, value); err != nil {
		return nil, err
	}
	for i := 0; i < width; i++ {
		var h ast.ASTHandle
		// (aux % two).is_zero(); aux = div(aux, two)
		err := errors.Join(q.Mod(&aux, &two, &bit), q.IDiv(&aux, &two, &aux))
		if err != nil {
			return nil, err
		}
		if bit.IsZero() {
			h, err = e.config.manager.MkFalse()
		} else {
			h, err = e.config.manager.MkTrue()
		}
		if err != nil {
			return nil, err
		}
		ref, err := e.config.manager.NewRef(h)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func (e *Engine) MkHalfAdder(a, b ast.ASTHandle, out, carry ast.ASTRef) error {
	if err := e.config.simplify(ast.ASTOpXor, out, a, b); err != nil {
		return err
	}
	return e.config.simplify(ast.ASTOpAnd, carry, a, b)
}

func (e *Engine) MkFullAdder(a, b, carryIn ast.ASTHandle, out, carry ast.ASTRef) error {
	if err := e.config.MkXor3(a, b, carryIn, out); err != nil {
		return err
	}
	return e.config.MkCarry(a, b, carryIn, carry)
}

func (e *Engine) MkNeg(bits []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(bits) == 0 {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	truth, err := m.MkTrue()
	if err != nil {
		return nil, err
	}
	carry, err := m.NewRef(truth)
	if err != nil {
		return nil, err
	}
	defer func() {
		if carry != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&carry))
		}
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	for i, bit := range bits {
		notBit, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		if err = e.config.simplify(ast.ASTOpNot, notBit, bit); err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		notHandle, err := m.RefGet(notBit)
		if err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			_ = m.DelRef(&notBit)
			return nil, err
		}
		var nextCarry ast.ASTRef
		if i+1 == len(bits) {
			err = e.config.simplify(ast.ASTOpXor, bitOut, notHandle, carryHandle)
		} else {
			nextCarry, err = m.NewRef(ast.ASTHandle{})
			if err == nil {
				err = e.MkHalfAdder(notHandle, carryHandle, bitOut, nextCarry)
			}
		}
		if err != nil {
			_ = m.DelRef(&notBit)
			_ = m.DelRef(&bitOut)
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, err
		}
		out = append(out, bitOut)
		if nextCarry != (ast.ASTRef{}) {
			// cin = cout releases the previous carry before the
			// loop-scoped not_a dies.
			if err = m.DelRef(&carry); err != nil {
				_ = m.DelRef(&notBit)
				_ = m.DelRef(&nextCarry)
				return nil, err
			}
			carry = nextCarry
			if err = m.DelRef(&notBit); err != nil {
				return nil, err
			}
			continue
		}
		// The last bit uses mk_xor: cout is unchanged, so not_a dies first
		// and the carry at exit.
		if err = m.DelRef(&notBit); err != nil {
			return nil, err
		}
		if err = m.DelRef(&carry); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (e *Engine) MkAdder(a, b []ast.ASTHandle) (out []ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ErrASTDomain
	}
	m := e.config.manager
	falsehood, err := m.MkFalse()
	if err != nil {
		return nil, err
	}
	carry, err := m.NewRef(falsehood)
	if err != nil {
		return nil, err
	}
	defer func() {
		if carry != (ast.ASTRef{}) {
			result = errors.Join(result, m.DelRef(&carry))
		}
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
		}
	}()
	for i := range a {
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			return nil, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		var nextCarry ast.ASTRef
		if i+1 == len(a) {
			err = e.config.MkXor3(a[i], b[i], carryHandle, bitOut)
		} else {
			nextCarry, err = m.NewRef(ast.ASTHandle{})
			if err == nil {
				err = e.MkFullAdder(a[i], b[i], carryHandle, bitOut, nextCarry)
			}
		}
		if err != nil {
			_ = m.DelRef(&bitOut)
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, err
		}
		out = append(out, bitOut)
		if err = m.DelRef(&carry); err != nil {
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, err
		}
		carry = nextCarry
	}
	return out, nil
}

// MkSubtracter computes a-b modulo 2^len(a). The returned carry is true iff
// unsigned a is at least b. Both the bit refs and carry ref are caller-owned.
func (e *Engine) MkSubtracter(a, b []ast.ASTHandle) (out []ast.ASTRef, carry ast.ASTRef, result error) {
	if len(a) == 0 || len(a) != len(b) {
		return nil, ast.ASTRef{}, ast.ErrASTDomain
	}
	m := e.config.manager
	truth, err := m.MkTrue()
	if err != nil {
		return nil, ast.ASTRef{}, err
	}
	carry, err = m.NewRef(truth)
	if err != nil {
		return nil, ast.ASTRef{}, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, releaseRefs(m, out))
			out = nil
			if carry != (ast.ASTRef{}) {
				result = errors.Join(result, m.DelRef(&carry))
			}
		}
	}()
	for i := range a {
		notB, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, carry, err
		}
		if err = e.config.simplify(ast.ASTOpNot, notB, b[i]); err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		notBHandle, err := m.RefGet(notB)
		if err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		carryHandle, err := m.RefGet(carry)
		if err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		bitOut, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			_ = m.DelRef(&notB)
			return nil, carry, err
		}
		nextCarry, err := m.NewRef(ast.ASTHandle{})
		if err == nil {
			err = e.MkFullAdder(a[i], notBHandle, carryHandle, bitOut, nextCarry)
		}
		if err != nil {
			_ = m.DelRef(&notB)
			_ = m.DelRef(&bitOut)
			if nextCarry != (ast.ASTRef{}) {
				_ = m.DelRef(&nextCarry)
			}
			return nil, carry, err
		}
		out = append(out, bitOut)
		// cin = cout releases the previous carry before the loop-scoped
		// not_b dies.
		if err = m.DelRef(&carry); err != nil {
			_ = m.DelRef(&notB)
			_ = m.DelRef(&nextCarry)
			return nil, carry, err
		}
		carry = nextCarry
		if err = m.DelRef(&notB); err != nil {
			return nil, carry, err
		}
	}
	return out, carry, nil
}
