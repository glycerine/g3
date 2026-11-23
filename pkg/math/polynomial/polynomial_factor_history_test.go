package polynomial

import (
	"strconv"
	"testing"
)

func factorHistory(size int) (*MultivariatePolynomialManager, *PolynomialFactors) {
	m := &MultivariatePolynomialManager{}
	m.self = m
	f := &PolynomialFactors{manager: m}
	f.self = f
	for i := 0; i < size; i++ {
		// A nonzero initial offset exercises both ends of the receipt directory.
		frame := &polynomialFactorCoreFrame{manager: m, target: f, invocation: uint64(39 + i)}
		frame.self = frame
		m.factorFrames = append(m.factorFrames, frame)
	}
	return m, f
}

func TestPolynomialFactorReceiptDirectory(t *testing.T) {
	m, _ := factorHistory(3)
	for _, frame := range m.factorFrames {
		r := polynomialFactorCoreInvocationReceipt{m, frame, frame.invocation}
		if !m.matchesFactorCore(&r) {
			t.Fatal("lost the owning frame", r.invocation)
		}
		copy := *frame
		copy.self = &copy
		r.frame = &copy
		if m.matchesFactorCore(&r) {
			t.Fatal("accepted a different frame with the same invocation", r.invocation)
		}
	}
	for _, n := range []uint64{0, 1, 38, 40, 42, ^uint64(0)} {
		r := polynomialFactorCoreInvocationReceipt{m, m.factorFrames[0], n}
		if m.matchesFactorCore(&r) {
			t.Fatal("accepted invalid invocation", n)
		}
	}
}

func BenchmarkPolynomialFactorHistory(b *testing.B) {
	for _, n := range []int{128, 1024, 8192} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			m, f := factorHistory(n)
			frame := m.factorFrames[n-1]
			r := polynomialFactorCoreInvocationReceipt{m, frame, frame.invocation}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !m.matchesFactorCore(&r) || m.factorCoreExecuting() || f.factorCallerExecuting() {
					b.Fatal("idle directory or live receipt lost")
				}
			}
		})
	}
}

func TestPolynomialFactorWorkTracksNestedTargets(t *testing.T) {
	m, first := factorHistory(2)
	second := &PolynomialFactors{manager: m}
	second.self = second
	outer, inner := m.factorFrames[0], m.factorFrames[1]
	inner.target = second
	outer.enterWork()
	inner.enterWork()
	if !m.factorCoreExecuting() || !first.factorCallerExecuting() || !second.factorCallerExecuting() {
		t.Fatal("nested work missing")
	}
	inner.leaveWork()
	if !m.factorCoreExecuting() || !first.factorCallerExecuting() || second.factorCallerExecuting() {
		t.Fatal("inner completion changed the wrong owner")
	}
	inner.target = first
	inner.enterWork()
	inner.leaveWork()
	if !first.factorCallerExecuting() {
		t.Fatal("nested completion cleared an executing parent on the same target")
	}
	outer.leaveWork()
	if m.factorCoreExecuting() || first.factorCallerExecuting() {
		t.Fatal("completed work retained a busy owner")
	}
}
