package polynomial

// Source: src/math/polynomial numeric invocation adapters.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func numericIntegerManager_addMulWithSourceReceipt(m *unumeral.IntegerManager, a, b, c, d *unumeral.Int, receipt *unumeral.IntegerAddMulInvocationReceipt, arithmetic *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(arithmeticOnlyUtility(m, arithmetic)).AddMulTracked(a, b, c, d, receipt)
}

func numericRationalManager_setIntegerWithArithmeticReceipt(m *unumeral.RationalManager, a *unumeral.Rational, n *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(arithmeticOnlyUtility(m.Integers(), receipt)).SetInteger(a, n)
}

func numericIntegerModularManager_addWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b, c *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(numericIntegerModularManager_arithmeticUtility(m, receipt)).Add(a, b, c)
}

func numericIntegerModularManager_mulWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b, c *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(numericIntegerModularManager_arithmeticUtility(m, receipt)).Mul(a, b, c)
}

func numericIntegerModularManager_addMulWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b, c, d *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	receipt = numericIntegerModularManager_arithmeticReceiptForReceiver(m, receipt)
	// receipt is now nil or valid for m, so the AddMul child receipt and the
	// allocation context follow without validating it again.
	var child *unumeral.IntegerAddMulInvocationReceipt
	if receipt != nil {
		child = &receipt.frame.addMul
	}
	if polynomialValidateOwnership && child != numericIntegerModularManager_arithmeticAddMulReceipt(m, receipt) {
		panic("polynomial: arithmetic AddMul receipt differs from its validated receipt")
	}
	e := m.WithAllocationContext(arithmeticOnlyUtility(m.Integers(), receipt)).AddMulTracked(a, b, c, d, child)
	if child != nil && receipt.frame.sourceCause == nil {
		receipt.frame.sourceCause = m.Integers().AddMulInvocationSourceCause(child)
	}
	return e
}

func numericIntegerModularManager_setInt64WithArithmeticReceipt(m *unumeral.IntegerModularManager, a *unumeral.Int, v int64, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(numericIntegerModularManager_arithmeticUtility(m, receipt)).SetInt64(a, v)
}

func numericIntegerModularManager_copyWithArithmeticReceipt(m *unumeral.IntegerModularManager, a, b *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(numericIntegerModularManager_arithmeticUtility(m, receipt)).Copy(a, b)
}

// arithmeticOnlyUtility is
// integerAllocationContextRef(integerAllocationInvocationContext{}.withArithmeticInvocation(r)).utility(z)
// without building, comparing and scanning the large context: in a context
// whose only owner is the arithmetic receipt every other owner branch of
// utilityWithRoots and roots is skipped, the context is zero exactly when r
// is nil, and roots reduces to arithmeticRoots. The roots record is bound
// with the same caller context, integer manager and policy. Checked and debug
// builds also run the general path and panic if the results differ.
func arithmeticOnlyUtility(z *unumeral.IntegerManager, r *polynomialArithmeticInvocationReceipt) unumeral.NumericAllocationContext {
	if !polynomialValidateOwnership {
		return arithmeticOnlyUtilityFast(z, r)
	}
	slow := integerAllocationContextRef((integerAllocationInvocationContext{}).withArithmeticInvocation(r)).utility(z)
	if fast := arithmeticOnlyUtilityFast(z, r); fast != slow {
		panic("polynomial: arithmetic-only utility differs from the general utility")
	}
	return slow
}
func arithmeticOnlyUtilityFast(z *unumeral.IntegerManager, r *polynomialArithmeticInvocationReceipt) unumeral.NumericAllocationContext {
	if r == nil {
		return unumeral.NumericAllocationContext{}
	}
	var roots *polynomialNumericRoots
	if r.invocation != 0 {
		roots = numericIntegerManager_arithmeticAllocationRoots(z, r)
	}
	if roots == nil {
		return unumeral.NumericAllocationContext{}.WithCaller(&invalidPolynomialNumericHooks)
	}
	roots.caller = integerAllocationInvocationContext{arithmetic: r, arithmeticInvocation: r.invocation}
	roots.integer = z
	roots.hooks.Policy = roots
	return unumeral.NumericAllocationContext{}.WithCaller(&roots.hooks)
}

// numericIntegerModularManager_arithmeticUtility is
// integerAllocationContextRef(numericIntegerModularManager_arithmeticAllocationContext(m, r)).utility(m.Integers()):
// the receipt resolved for m is nil or valid, and its context is the
// arithmetic-only context of arithmeticOnlyUtility.
func numericIntegerModularManager_arithmeticUtility(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) unumeral.NumericAllocationContext {
	return arithmeticOnlyUtility(m.Integers(), numericIntegerModularManager_arithmeticReceiptForReceiver(m, r))
}
