package numeral

// Source: src/util/mpz.cpp, mpn.cpp, mpzzp.h: primitive invocation boundaries.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// NumericInvocationHooks transports caller-owned lifetime policy. The hooks
// borrow the caller only during the operation; utility managers never store
// concrete polynomial or algebraic owners. Roots must validate caller identity
// and generation before returning ownership of allocation prefixes.

type NumericInvocationHooks struct {
	// Generation distinguishes reused caller storage when a caller opts in.
	Generation uint64
	Policy     NumericCallerPolicy
	Roots      func(*IntegerManager, *NumericAllocationRoots) *NumericAllocationRoots
	Prepare    func(*IntegerManager, string) error
	Record     func(*IntegerManager, string, NumericInvocationReceipt)

	ScalarRaw   func(*IntegerManager, string) (*IntegerScalarRawScope, error)
	Normalize   func(*IntegerModularManager, Int) (Int, error)
	Power       func(*IntegerModularManager) (*Int, func() error, func(error), error)
	SourceCause func(*IntegerManager) error
	Bound       func(bool) bool
}

type NumericInvocationReceipt struct {
	Raw           *IntegerRawInvocationReceipt
	Divides       *IntegerDividesNumeralInvocationReceipt
	PerfectSquare *IntegerPerfectSquareInvocationReceipt
}
type NumericAllocationContext = integerAllocationInvocationContext
type IntegerRawInvocationReceipt = integerRawInvocationReceipt
type IntegerDividesNumeralInvocationReceipt = integerDividesNumeralInvocationReceipt
type IntegerPerfectSquareInvocationReceipt = integerPerfectSquareInvocationReceipt

type IntegerScalarRawScope = integerScalarRawScope

func (c integerAllocationInvocationContext) WithCaller(h *NumericInvocationHooks) integerAllocationInvocationContext {
	c.Caller = h
	c.callerGeneration = 0
	if h != nil {
		c.callerGeneration = h.Generation
	}
	return c
}
func (c *integerAllocationInvocationContext) roots(m *IntegerManager) *NumericAllocationRoots {
	if !c.callerCurrent() {
		return nil
	}
	r := c.localRoots(m)
	if c.hasLocalInvocation() && r == nil {
		return nil
	}
	if c.Caller != nil {
		callerRoots := c.Caller.callRoots(m, r)
		if callerRoots == nil {
			return nil
		}
		if r != nil {
			return r
		}
		return callerRoots
	}
	return r
}
func (c *integerAllocationInvocationContext) hasLocalInvocation() bool {
	return c.rationalService != nil || c.mpfRationalSet != nil || c.mpfFormattedSet != nil || c.mpfInput != nil || c.mpfConversion != nil || c.mpfFormat != nil || c.mpfPower != nil || c.mpfRound != nil || c.mpfRoundSqrt != nil || c.BoolDivides != nil || c.raw != nil || c.pn != nil || c.addMul != nil || c.perfectSquare != nil
}

// prepareCallerRoots runs the caller's prepare once per allocation (the
// policy is idempotent while its frames do not change) and also returns the
// caller roots that prepare validated, when the caller supports it and
// changed nothing after validating them.
func (c *integerAllocationInvocationContext) prepareCallerRoots(m *IntegerManager, phase string) (*NumericAllocationRoots, error) {
	if !c.callerCurrent() {
		return nil, ErrNumericCallerOwner
	}
	if c.Caller == nil {
		return nil, nil
	}
	if p, ok := c.Caller.Policy.(NumericCallerPrepareRootsPolicy); ok {
		return p.NumericPrepareRoots(m, phase)
	}
	return nil, c.Caller.callPrepare(m, phase)
}

// rootsAfterPrepare is roots(m), given the caller roots prepareCallerRoots
// returned. Between the two calls allocateWithSourceContext only runs the
// arbitrary-float history prepares, which are no-ops unless one of their
// frames is present, so without those frames callRoots would return the
// same validated record. Local roots are computed and take precedence
// exactly as in roots.
func (c *integerAllocationInvocationContext) rootsAfterPrepare(m *IntegerManager, prepared *NumericAllocationRoots) *NumericAllocationRoots {
	if prepared == nil || c.mpfRationalSet != nil || c.mpfFormattedSet != nil || c.mpfRound != nil || c.mpfRoundSqrt != nil || c.mpfInput != nil {
		return c.roots(m)
	}
	if numericValidateCells {
		// Checked builds revalidate fully and check the reuse against it.
		full := c.roots(m)
		if reused := c.reusePreparedRoots(m, prepared); reused != full {
			panic("numeral: prepared caller roots differ from revalidated roots")
		}
		return full
	}
	return c.reusePreparedRoots(m, prepared)
}
func (c *integerAllocationInvocationContext) reusePreparedRoots(m *IntegerManager, prepared *NumericAllocationRoots) *NumericAllocationRoots {
	if !c.callerCurrent() {
		return nil
	}
	r := c.localRoots(m)
	if r == nil {
		if c.hasLocalInvocation() {
			return nil
		}
		return prepared
	}
	return r
}
func (c *integerAllocationInvocationContext) recordCaller(m *IntegerManager, event string, r NumericInvocationReceipt) {
	if !c.callerCurrent() {
		return
	}
	if c.Caller != nil && r.Raw != nil && r.Raw.Frame != nil {
		r.Raw.Frame.caller = c.Caller
	}
	if c.Caller != nil {
		c.Caller.callRecord(m, event, r)
	}
}
func (m *IntegerManager) numericCallerExecuting() bool {
	return m != nil && m.self == m && (m.callerExecuting != nil && m.callerExecuting() || callerBindingsExecuting(m.callerBindings))
}
func (m *NaturalManager) numericCallerExecuting() bool {
	return m != nil && m.self == m && (m.callerExecuting != nil && m.callerExecuting() || callerBindingsExecuting(m.callerBindings))
}

// SetCallerExecutionGuard binds the higher owner's synchronous reentry guard.
// The owner must clear its guard before destroying the borrowed manager.
func (m *IntegerManager) SetCallerExecutionGuard(guard func() bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.numericCallerExecuting() {
		return ErrNumericPending
	}
	m.callerExecuting = guard
	return nil
}
func (m *NaturalManager) SetCallerExecutionGuard(guard func() bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.ClosedField {
		return arena.ErrClosed
	}
	if m.numericCallerExecuting() {
		return ErrNumericPending
	}
	m.callerExecuting = guard
	return nil
}

// NumericCharacterCaller borrows primitive formatting storage from a caller.
type NumericCharacterPolicy interface {
	NumericCharacterValid(*NaturalManager, bool) bool
	NumericCharacterPrepare() error
}

type NumericCharacterCaller struct {
	Policy         NumericCharacterPolicy
	Characters     *IntegerCharacterScope
	Formatting     *NaturalFormattingScope
	Generation     uint64
	Valid          func(*NaturalManager, bool) bool
	PrepareStorage func() error
}
type IntegerCharacterScope = integerCharacterScope
type NaturalFormattingScope = naturalFormattingScope

func (m *IntegerModularManager) SetCallerExecutionGuard(guard func() bool) error {
	if m == nil || m.self != m {
		return ErrNumericOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.numericCallerExecuting() {
		return ErrNumericPending
	}
	m.CallerExecuting = guard
	return nil
}

func (c *NumericCharacterCaller) valid(n *NaturalManager, executing bool) bool {
	if c == nil {
		return false
	}
	if c.Policy != nil {
		return c.Policy.NumericCharacterValid(n, executing)
	}
	return c.Valid != nil && c.Valid(n, executing)
}
func (c *NumericCharacterCaller) prepare() error {
	if c.Policy != nil {
		return c.Policy.NumericCharacterPrepare()
	}
	if c.PrepareStorage != nil {
		return c.PrepareStorage()
	}
	return nil
}

func (c *integerAllocationInvocationContext) callerCurrent() bool {
	return c.Caller == nil || c.callerGeneration == c.Caller.Generation
}
