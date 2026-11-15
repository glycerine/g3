package polynomial

// Source: src/math/polynomial/upolynomial.h:401-423.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialFactorToNumeralVectorTransportCannotRetireExecutingOwner(t *testing.T) {
	polynomialToNumeralVectorTransportCannotRetireExecutingOwner(t, false)
}
func polynomialToNumeralVectorTransportCannotRetireExecutingOwner(t *testing.T, selected bool) {
	for _, operation := range []string{"fullSource", "fullClose", "coreSource", "coreClose", "integerSource", "integerClose", "rationalSource", "rationalClose", "publicSet", "typedSource", "typedOwn"} {
		t.Run(operation, func(t *testing.T) {
			b := arena.NewBudget(1 << 27)
			var full *MultivariatePolynomialManager
			var fs *PolynomialFactors
			var q *unumeral.RationalManager
			var callbackOut, callbackInput unumeral.Int
			var p Polynomial
			var output upolyVectorState
			var captured polynomialFactorNumeralVectorReceipt
			active, seen := false, false
			rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, ThreadLocal: true, SizeTBits: 64, Transport: func(f umemory.Failure) error {
				if active {
					seen = true
					captured = full.factorNumeralVector.direct
					before := rtStateForFactors(fs)
					fullClosing, fullSource, fullStage := full.closing, full.sourceStarted, full.cleanupStage
					coreClosing, coreSource, coreStage := full.core.closing, full.core.sourceStarted, full.core.closeStage
					z := full.core.integer
					zClosed, zHeader, zStage, zStarted := z.Closed(), z.HeaderBytes, z.SourceFinalizeStage, z.SourceFinalizing
					qClosing, qStarted, qCause := q.Closing, q.SourceFinalizing, q.CleanupCauseField
					used := b.Used()
					oldOut := callbackOut
					oldValue, inspectError := z.Compare(&callbackOut, &z.Two64)
					serviceMust(t, inspectError)
					var err error
					switch operation {
					case "source":
						err = fs.SourceFinalize()
					case "close":
						err = fs.CloseChecked()
					case "cleanup":
						err = fs.Cleanup()
					case "fullSource":
						err = full.SourceFinalize()
					case "fullClose":
						err = full.CloseChecked()
					case "coreSource":
						err = full.core.SourceFinalizePrefix()
					case "coreClose":
						err = full.core.CloseChecked()
					case "integerSource":
						err = z.SourceFinalize()
					case "integerClose":
						err = z.CloseChecked()
					case "rationalSource":
						err = q.SourceFinalize()
					case "rationalClose":
						err = q.CloseChecked()
					case "typedCopy":
						err = full.upm.modular.WithAllocationContext(integerAllocationContextRef((integerAllocationInvocationContext{factorNumeralVector: &full.factorNumeralVector.direct, factorNumeralVectorInvocation: full.factorNumeralVector.invocation})).utility(full.upm.modular.Integers())).Copy(&callbackOut, &callbackInput)
					case "typedSource":
						err = full.finishFactorNumeralVectorSource()
					case "typedOwn":
						err = full.cleanupFactorNumeralVector(&full.factorNumeralVector.direct, true)
					case "publicSet":
						err = full.toNumeralVectorSource(p, 0, selected, &output, nil)

					}
					newValue, inspectError := z.Compare(&callbackOut, &z.Two64)
					serviceMust(t, inspectError)
					if err != ErrPolynomialPending || before != rtStateForFactors(fs) || used != b.Used() || full.closing != fullClosing || full.sourceStarted != fullSource || full.cleanupStage != fullStage || full.core.closing != coreClosing || full.core.sourceStarted != coreSource || full.core.closeStage != coreStage || z.Closed() != zClosed || z.HeaderBytes != zHeader || z.SourceFinalizeStage != zStage || z.SourceFinalizing != zStarted || q.Closing != qClosing || q.SourceFinalizing != qStarted || q.CleanupCauseField != qCause || callbackOut != oldOut || newValue != oldValue {
						t.Errorf("executing factor owner changed: %v before=%+v after=%+v", err, before, rtStateForFactors(fs))
					}
				}
				return f
			}})
			serviceMust(t, e)
			cfg := serviceConfig(b).Integers
			cfg.Memory = rt
			q, e = unumeral.NewRationalManager(unumeral.RationalConfig{Integers: cfg})
			serviceMust(t, e)
			mm, e := NewMonomialManager(MonomialManagerConfig{Budget: b, Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}})
			serviceMust(t, e)
			full, e = NewMultivariatePolynomialManager(MultivariatePolynomialManagerConfig{Core: MultivariatePolynomialConfig{Integer: q.Integers(), Rationals: q, Monomials: mm, Checkpoint: func() bool { return true }, Cooperation: func() error { return nil }}})
			serviceMust(t, e)
			fs, e = NewPolynomialFactors(full)
			serviceMust(t, e)
			var a unumeral.Int
			digits := make([]uint32, 80)
			digits[79] = 1
			serviceMust(t, q.Integers().SetLimbs(&a, digits))
			serviceMust(t, q.Integers().Copy(&callbackInput, &a))
			serviceMust(t, q.Integers().Copy(&callbackOut, &q.Integers().Two64))
			var coefficient unumeral.Int
			serviceMust(t, q.Integers().Copy(&coefficient, &a))
			p, e = full.core.MkConstantTake(&coefficient)
			serviceMust(t, e)
			serviceMust(t, full.core.IncRef(p))
			np, e := full.Normalize(p)
			serviceMust(t, e)
			serviceMust(t, full.core.IncRef(np))
			serviceMust(t, full.core.DecRef(np))
			pad := int64(99996) - rt.State().LocalSize - 8
			if pad <= 0 {
				t.Fatal("actual TLS frontier", rt.State())
			}
			held, e := rt.Allocate(uint64(pad))
			serviceMust(t, e)
			serviceMust(t, rt.SetMaxSize(1))
			active = true
			cause := full.toNumeralVectorSource(p, 0, selected, &output, nil)
			active = false
			if !seen || !numericPhysicalReservationRefusal(cause) {
				t.Fatal("actual SET returning transport", seen, cause)
			}
			polynomialImpPhysicalAuditNamedRaw(t, full, []Polynomial{p}, []*unumeral.Int{&a, &callbackInput, &callbackOut, &fs.constant, &fs.local}, fs.header, []umemory.Block{held})
			serviceMust(t, rt.SetMaxSize(0))
			serviceMust(t, full.cleanupFactorNumeralVector(&full.factorNumeralVector.direct, false))
			serviceMust(t, full.upm.reset(&output))
			full.upm.freeBacking(&output)
			serviceMust(t, full.cleanupFactorNumeralVector(&full.factorNumeralVector.direct, true))
			before, ok := full.factorNumeralVectorDiagnostic(&captured)
			if !ok || before.bodyCause == nil || before != full.factorNumeralVectorSnapshot() {
				t.Fatal("callback receipt did not retain FINAL source/own history", before, full.factorNumeralVectorSnapshot())
			}
			serviceMust(t, full.toNumeralVectorSource(full.core.zero, 0, selected, &output, nil))
			after, ok := full.factorNumeralVectorDiagnostic(&captured)
			if !ok || after != before {
				t.Fatal("callback-captured old history changed on later call", after, before)
			}
			used := b.Used()
			state := output
			serviceMust(t, full.cleanupFactorNumeralVector(&captured, true))
			if used != b.Used() || output != state {
				t.Fatal("old callback receipt affected later output")
			}
			serviceMust(t, full.cleanupFactorNumeralVector(&full.factorNumeralVector.direct, false))
			serviceMust(t, full.upm.reset(&output))
			full.upm.freeBacking(&output)
			serviceMust(t, full.cleanupFactorNumeralVector(&full.factorNumeralVector.direct, true))
			serviceMust(t, full.core.DecRef(p))
			serviceMust(t, fs.CloseChecked())
			serviceMust(t, q.Integers().Del(&a))
			serviceMust(t, q.Integers().Del(&callbackOut))
			serviceMust(t, q.Integers().Del(&callbackInput))
			serviceMust(t, full.CloseChecked())
			serviceMust(t, q.CloseChecked())
			serviceMust(t, mm.CloseChecked())
			serviceMust(t, rt.Deallocate(&held))
			serviceMust(t, rt.Close())
			if b.Used() != 0 {
				t.Fatal("actual owner retained", b.Used())
			}
		})
	}
}
