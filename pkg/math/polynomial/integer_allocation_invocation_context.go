package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/algebraic_numbers.cpp:1565-1577; src/util/mpz.h:124-149.
// Source: src/util/mpz.h:allocate; src/util/mpz.cpp scratch/publication callers.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Explicit passed invocation identity. It carries no allocation snapshot and
// creates no source numeral, reservation, or ambient callback owner hook.

type integerAllocationInvocationContext struct {
	squareFreeCall                 *upolySquareFreeCallScope
	squareFreeCallInvocation       uint64
	squareRoot                     *polynomialSqrtInvocationReceipt
	squareRootInvocation           uint64
	defaultNormalization           *upolyDefaultNormalizeScope
	defaultNormalizationInvocation uint64
	berlekamp                      *upolyBerlekampNumeric
	berlekampGeneration            uint64
	sexprArithmetic                *sexprPolynomialArithmeticScope
	sexprArithmeticInvocation      uint64
	sturmIsolation                 *upolySturmIsolationScope
	sturmIsolationInvocation       uint64
	sequenceSturm                  *upolySequenceSturmScope
	sequenceSturmInvocation        uint64
	rootID                         *upolyRootIDScope
	rootIDInvocation               uint64
	knuth                          *upolyKnuthScope
	knuthInvocation                uint64

	rootUpperBound              *upolyRootUpperBoundScope
	removeOneHalfRoot           *upolyRemoveOneHalfRootScope
	rootUpperBoundInvocation    uint64
	removeOneHalfRootInvocation uint64
	oneHalfRoot                 *upolyOneHalfRootScope
	oneHalfRootInvocation       uint64
	integerEvaluation           *upolyIntegerEvaluationScope
	integerEvaluationInvocation uint64

	rationalEvaluation           *upolyRationalEvaluationScope
	rationalEvaluationInvocation uint64
	binaryEvaluation             *upolyBinaryEvaluationScope
	binaryEvaluationInvocation   uint64
	algebraicNegEvalInvocation   uint64
	algebraicNeg                 *algebraicNegScope
	algebraicNegInvocation       uint64
	algebraicNegEval             bool

	upolyDisplay           *upolyDisplayScope
	upolyDisplayInvocation uint64

	factorNumeralVector           *polynomialFactorNumeralVectorReceipt
	factorNumeralVectorInvocation uint64
	toPolynomial                  *polynomialToPolynomialInvocationReceipt
	toPolynomialInvocation        uint64
	factorMemberSet               *polynomialFactorMemberSetReceipt
	factorMemberSetInvocation     uint64
	factorConstant                *polynomialFactorConstantInvocationReceipt
	factorConstantInvocation      uint64
	primePreparation              *upolyFactorPrimePreparationScope
	primePreparationInvocation    uint64
	primePreparationManager       *UPolynomialManager
	preparation                   *upolyFactorPreparationScope
	preparationInvocation         uint64
	trial                         *upolyRecombinationTrialScope
	recombination                 *upolyRecombinationStepScope
	recombinationInvocation       uint64
	recombinationManager          *UPolynomialManager
	somNormalize                  *polynomialSOMNormalizeInvocationReceipt
	somNormalizeInvocation        uint64
	normalization                 *polynomialNormalizeInvocationReceipt
	normalizationInvocation       uint64
	trialInvocation               uint64
	trialManager                  *UPolynomialManager
	henselCaller                  *upolyHenselCallerScope
	henselCallerInvocation        uint64
	henselCallerManager           *UPolynomialManager
	henselPair                    *upolyHenselPairInvocationReceipt
	henselPairInvocation          uint64
	henselManager                 *UPolynomialManager
	iccpConstant                  *polynomialICCPConstantInvocationReceipt
	iccpConstantInvocation        uint64
	primeTrial                    *upolyPrimeTrialInvocationReceipt
	primeTrialInvocation          uint64
	pseudo                        *polynomialPseudoDivisionInvocationReceipt
	pseudoInvocation              uint64
	mignotte                      *upolyMignotteInvocationReceipt
	mignotteInvocation            uint64
	divides                       *polynomialDividesInvocationReceipt
	dividesInvocation             uint64

	neg                   *polynomialNegInvocationReceipt
	negInvocation         uint64
	arithmeticInvocation  uint64
	scalarExact           *polynomialScalarExactDivInvocationReceipt
	scalarExactInvocation uint64
	exactDiv              *polynomialExactDivInvocationReceipt
	exactDivInvocation    uint64
	content               *polynomialContentInvocationReceipt
	contentInvocation     uint64

	arithmetic *polynomialArithmeticInvocationReceipt

	derivative                       *polynomialDerivativeInvocationReceipt
	scalarDiv                        *polynomialScalarDivInvocationReceipt
	factorFrontendConstant           *upolyFactorConstantBinding
	factorFrontendConstantInvocation uint64

	factorFrontend           *upolyFactorFrontendScope
	factorFrontendInvocation uint64
}

// roots skips absent owners before invoking their validators. The context is
// large and the validators take it by value, so calling every validator for
// every numeric operation needlessly copies it many times.
func (c *integerAllocationInvocationContext) roots(m *unumeral.IntegerManager) *polynomialNumericRoots {
	if c.squareRoot != nil {
		r := c.squareRoot
		if c.squareRootInvocation == 0 || c.squareRootInvocation != r.invocation || r.manager == nil || !r.manager.matchesSqrtInvocation(r) || r.manager.core.integer != m || !r.frame.executing || r.manager.core.callbackDepth != 0 {
			return nil
		}
		return &r.frame.numeric
	}

	// Every present owner's validator must accept, then the first present
	// owner in priority order supplies the roots. The validators are pure,
	// so each runs once and its result serves both passes.
	if c.knuth != nil {
		if q := c.knuthFrame(m); q != nil {
			return &q.numeric
		}
		return nil
	}

	var rootUpper *upolyRootUpperBoundScope
	if c.rootUpperBound != nil {
		if rootUpper = c.rootUpperBoundFrame(m); rootUpper == nil {
			return nil
		}
	}
	var removeOneHalf *upolyRemoveOneHalfRootScope
	if c.removeOneHalfRoot != nil {
		if removeOneHalf = c.removeOneHalfRootFrame(m); removeOneHalf == nil {
			return nil
		}
	}
	var oneHalf *upolyOneHalfRootScope
	if c.oneHalfRoot != nil {
		if oneHalf = c.oneHalfRootFrame(m); oneHalf == nil {
			return nil
		}
	}
	var integerEval *upolyIntegerEvaluationScope
	if c.integerEvaluation != nil {
		if integerEval = c.integerEvaluationFrame(m); integerEval == nil {
			return nil
		}
	}
	var rationalEval *upolyRationalEvaluationScope
	if c.rationalEvaluation != nil {
		if rationalEval = c.rationalEvaluationFrame(m); rationalEval == nil {
			return nil
		}
	}
	var binaryEval *upolyBinaryEvaluationScope
	if c.binaryEvaluation != nil {
		if binaryEval = c.binaryEvaluationFrame(m); binaryEval == nil {
			return nil
		}
	}
	var neg *algebraicNegScope
	if c.algebraicNeg != nil {
		if neg = c.algebraicNegFrame(m); neg == nil {
			return nil
		}
	}
	var display *upolyDisplayScope
	if c.upolyDisplay != nil {
		if display = c.upolyDisplayFrame(m); display == nil {
			return nil
		}
	}

	var numeralVector, toPolynomial, memberSet, factorConstant *polynomialNumericRoots
	if c.factorNumeralVector != nil {
		if numeralVector = numericIntegerManager_factorNumeralVectorRoots(m, c); numeralVector == nil {
			return nil
		}
	}
	if c.toPolynomial != nil {
		if toPolynomial = numericIntegerManager_toPolynomialAllocationRoots(m, c); toPolynomial == nil {
			return nil
		}
	}
	if c.factorMemberSet != nil {
		if memberSet = numericIntegerManager_factorMemberSetRoots(m, c); memberSet == nil {
			return nil
		}
	}
	if c.factorConstant != nil {
		if factorConstant = numericIntegerManager_factorConstantContextRoots(m, c); factorConstant == nil {
			return nil
		}
	}
	var frontendConstant *upolyFactorConstantBinding
	if c.factorFrontendConstant != nil {
		if frontendConstant = c.factorFrontendConstantFrame(m); frontendConstant == nil {
			return nil
		}
	}

	var frontend *upolyFactorFrontendScope
	if c.factorFrontend != nil {
		if frontend = c.factorFrontendFrame(m); frontend == nil {
			return nil
		}
	}

	var primePreparation *upolyFactorPrimePreparationScope
	if c.primePreparation != nil {
		if primePreparation = c.primePreparationFrame(m); primePreparation == nil {
			return nil
		}
	}
	var preparation *upolyFactorPreparationScope
	if c.preparation != nil {
		if preparation = c.preparationFrame(m); preparation == nil {
			return nil
		}
	}
	var recombination *upolyRecombinationStepScope
	if c.recombination != nil {
		if recombination = c.recombinationFrame(m); recombination == nil {
			return nil
		}
	}
	var trial *upolyRecombinationTrialScope
	if c.trial != nil {
		if trial = c.trialFrame(m); trial == nil {
			return nil
		}
	}
	var henselCaller *upolyHenselCallerScope
	if c.henselCaller != nil {
		if henselCaller = c.henselCallerFrame(m); henselCaller == nil {
			return nil
		}
	}
	var henselPair *upolyHenselPairInvocationReceipt
	if c.henselPair != nil {
		if henselPair = c.henselPairReceipt(m); henselPair == nil {
			return nil
		}
	}
	var iccp *polynomialNumericRoots
	if c.iccpConstant != nil {
		if iccp = numericIntegerManager_iccpConstantContextRoots(m, c); iccp == nil {
			return nil
		}
	}
	var primeTrial *upolyPrimeTrialInvocationReceipt
	if c.primeTrial != nil {
		if primeTrial = c.primeTrialReceipt(m); primeTrial == nil {
			return nil
		}
	}
	var pseudo *polynomialNumericRoots
	if c.pseudo != nil {
		if pseudo = numericIntegerManager_pseudoContextRoots(m, c); pseudo == nil {
			return nil
		}
	}
	var mignotte *upolyMignotteInvocationReceipt
	if c.mignotte != nil {
		if mignotte = c.mignotteReceipt(m); mignotte == nil {
			return nil
		}
	}

	var negRoots *polynomialNumericRoots
	if c.neg != nil {
		if c.negInvocation == 0 || c.negInvocation != c.neg.invocation {
			return nil
		}
		if negRoots = numericIntegerManager_negAllocationRoots(m, c.neg); negRoots == nil {
			return nil
		}
	}
	var som *polynomialSOMNormalizeInvocationReceipt
	if c.somNormalize != nil {
		if som = c.somNormalizeReceipt(m); som == nil {
			return nil
		}
	}
	var normalization *polynomialNormalizeInvocationReceipt
	if c.normalization != nil {
		if normalization = c.normalizationReceipt(m); normalization == nil {
			return nil
		}
	}
	var content *polynomialContentInvocationReceipt
	if c.content != nil {
		if content = c.contentReceipt(m); content == nil {
			return nil
		}
	}
	if c.factorMemberSet != nil {
		return memberSet
	}
	if c.factorConstant != nil {
		return factorConstant
	}

	if c.factorNumeralVector != nil {
		return numeralVector
	}
	if c.toPolynomial != nil {
		return toPolynomial
	}

	if c.upolyDisplay != nil {
		return &display.numeric
	}
	if c.iccpConstant != nil {
		return iccp
	}
	if c.pseudo != nil {
		return pseudo
	}
	if c.divides != nil {
		return numericIntegerManager_dividesContextRoots(m, c)
	}
	if c.scalarExact != nil {
		return numericIntegerManager_scalarExactContextRoots(m, c)
	}
	if c.exactDiv != nil {
		return numericIntegerManager_exactDivContextRoots(m, c)
	}
	if c.somNormalize != nil {
		return numericIntegerManager_somNormalizeAllocationRoots(m, som)
	}
	if c.normalization != nil {
		return numericIntegerManager_normalizationAllocationRoots(m, normalization)
	}
	if c.content != nil {
		return numericIntegerManager_contentAllocationRoots(m, content)
	}
	if c.neg != nil {
		return negRoots
	}
	if c.scalarDiv != nil {
		return numericIntegerManager_scalarDivAllocationRoots(m, c.scalarDiv)
	}
	if c.derivative != nil {
		return numericIntegerManager_derivativeAllocationRoots(m, c.derivative)
	}

	if c.rootUpperBound != nil {
		return &rootUpper.numeric
	}
	if c.removeOneHalfRoot != nil {
		return &removeOneHalf.numeric
	}
	if c.oneHalfRoot != nil {
		return &oneHalf.numeric
	}
	if c.integerEvaluation != nil {
		return &integerEval.numeric
	}
	if c.rationalEvaluation != nil {
		return &rationalEval.numeric
	}
	if c.binaryEvaluation != nil {
		return &binaryEval.numeric
	}
	if c.algebraicNeg != nil {
		if c.algebraicNegEval {
			return &neg.eval.numeric
		}
		return &neg.numeric
	}
	if c.primePreparation != nil {
		return &primePreparation.numeric
	}
	if c.factorFrontendConstant != nil {
		if frontendConstant.record != nil {
			return &frontendConstant.record.numeric
		}
		return &frontendConstant.numeric
	}
	if c.factorFrontend != nil {
		return &frontend.numeric
	}
	if c.preparation != nil {
		return &preparation.numeric
	}
	if c.recombination != nil {
		return &recombination.numeric
	}
	if c.trial != nil {
		return &trial.numeric
	}
	if c.primeTrial != nil {
		return &primeTrial.frame.numeric
	}
	if c.henselPair != nil {
		return &henselPair.frame.numeric
	}
	if c.henselCaller != nil {
		return &henselCaller.numeric
	}
	if c.mignotte != nil {
		return &mignotte.frame.numeric
	}
	if c.arithmetic != nil {
		return c.arithmeticRoots(m)
	}
	return nil
}

// arithmeticRoots is numericIntegerManager_arithmeticAllocationRoots(m,
// c.arithmeticReceipt(m)) without validating the receipt twice: the receipt
// is c.arithmetic exactly when its roots are non-nil, and nil otherwise.
func (c *integerAllocationInvocationContext) arithmeticRoots(m *unumeral.IntegerManager) *polynomialNumericRoots {
	if c.arithmetic != nil && c.arithmeticInvocation != 0 && c.arithmeticInvocation == c.arithmetic.invocation {
		return numericIntegerManager_arithmeticAllocationRoots(m, c.arithmetic)
	}
	return nil
}

func numericIntegerManager_copyFromOwnerWithArithmeticReceipt(m *unumeral.IntegerManager, out *unumeral.Int, owner *unumeral.IntegerManager, value *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(integerAllocationContextRef(((integerAllocationInvocationContext{}).withArithmeticInvocation(receipt))).utility(m)).CopyFromOwnerTracked(out, owner, value, nil)
}
func numericIntegerModularManager_arithmeticReceiptForReceiver(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) *polynomialArithmeticInvocationReceipt {
	if m == nil || m.Integers() == nil || r == nil || r.manager == nil || r.manager.modular != m || numericIntegerManager_arithmeticAllocationRoots(m.Integers(), r) == nil {
		return nil
	}
	return r
}
func numericIntegerModularManager_arithmeticAllocationContext(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) integerAllocationInvocationContext {
	return (integerAllocationInvocationContext{}).withArithmeticInvocation(numericIntegerModularManager_arithmeticReceiptForReceiver(m, r))
}
func numericIntegerModularManager_arithmeticAddMulReceipt(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) *unumeral.IntegerAddMulInvocationReceipt {
	if numericIntegerModularManager_arithmeticReceiptForReceiver(m, r) == nil {
		return nil
	}
	return &r.frame.addMul
}
func numericIntegerModularManager_normalizeWithArithmeticReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, r *polynomialArithmeticInvocationReceipt) error {
	if numericIntegerModularManager_arithmeticReceiptForReceiver(m, r) == nil {
		return m.PNormalize(out)
	}
	e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
	if r.frame.sourceCause == nil {
		r.frame.sourceCause = nil
	}
	return e
}

func (c integerAllocationInvocationContext) withArithmeticInvocation(r *polynomialArithmeticInvocationReceipt) integerAllocationInvocationContext {
	c.arithmetic = r
	c.arithmeticInvocation = 0
	if r != nil {
		c.arithmeticInvocation = r.invocation
	}
	return c
}
func (c *integerAllocationInvocationContext) arithmeticReceipt(m *unumeral.IntegerManager) *polynomialArithmeticInvocationReceipt {
	if c.arithmetic != nil && c.arithmeticInvocation != 0 && c.arithmeticInvocation == c.arithmetic.invocation && numericIntegerManager_arithmeticAllocationRoots(m, c.arithmetic) != nil {
		return c.arithmetic
	}
	return nil
}

// integerAllocationContextRef lets a pointer-receiver validator run on a
// context returned by value.
func integerAllocationContextRef(c integerAllocationInvocationContext) *integerAllocationInvocationContext {
	return &c
}
