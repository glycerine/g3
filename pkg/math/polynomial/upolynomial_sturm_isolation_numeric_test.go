package polynomial

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"strconv"
	"testing"
)

func TestUPolynomialSturmIsolationBinaryReachedReservations(t *testing.T) {
	success, debt := false, false
	for after := int64(0); after < 256 && !success; after++ {
		t.Run(strconv.FormatInt(after, 10), func(t *testing.T) {
			c, m, _ := algebraicNegTLSFixture(t, nil)
			z, b := m.config.Integer, c.config.Binary
			q, e := m.beginSturmIsolation(UPolynomialView{}, b)
			serviceMust(t, e)
			q.constructCore()
			ops := b.WithAllocationContext(integerAllocationContextRef(q.context()).utility(z))
			z.Budget().FailAfter(after)
			body := ops.PowerOfTwo(128, &q.lower, &q.copy)
			if body == nil {
				body = ops.Neg(&q.lower)
			}
			if body == nil {
				body = ops.PowerOfTwo(127, &q.upper, &q.copy)
			}
			if body == nil {
				body = ops.AddSub(&q.lower, &q.upper, &q.mid, false)
			}
			if body == nil {
				body = ops.Div2(&q.mid)
			}
			z.Budget().FailAfter(-1)
			if unumeral.NumericAllocationPending(&q.numeric) {
				debt = true
			}
			if body == nil {
				success = true
				for _, row := range []struct {
					value *unumeral.BinaryRational
					text  string
				}{
					{&q.lower, "-340282366920938463463374607431768211456"},
					{&q.upper, "170141183460469231731687303715884105728"},
					{&q.mid, "-85070591730234615865843651857942052864"},
				} {
					var expected unumeral.Int
					serviceMust(t, z.SetString(&expected, row.text))
					equal, e := b.EqualInteger(row.value, &expected)
					serviceMust(t, e)
					serviceMust(t, z.Del(&expected))
					if !equal {
						t.Fatal("independent binary arithmetic result", row.text)
					}
				}
			} else if !numericPhysicalReservationRefusal(body) {
				t.Fatal(body)
			}
			_ = q.finish(body)
			source, stack, coefficient := q.sourceStage, q.sourceStack, q.sourceCoefficient
			serviceMust(t, m.CleanupSturmIsolation())
			if q.active || q.pending() || source != q.sourceStage || stack != q.sourceStack || coefficient != q.sourceCoefficient {
				t.Fatal("numeric acknowledgement changed source history")
			}
			algebraicCellSourcePhysical(t, c, nil, nil, nil)
		})
	}
	if !success || !debt {
		t.Fatal("binary reservation campaign incomplete", success, debt)
	}
}

func TestUPolynomialSturmIsolationBinaryFailureTransportRejectsContext(t *testing.T) {
	var m *UPolynomialManager
	var context integerAllocationInvocationContext
	var issued unumeral.NumericBinaryRationalOperations
	armed, probed := false, false
	transport := func(f umemory.Failure) error {
		if !armed {
			return f
		}
		armed = false
		probed = true
		z := m.config.Integer
		used, stats := z.Budget().Used(), z.RuntimeOwner().Stats()
		for _, ops := range []unumeral.NumericBinaryRationalOperations{issued, m.sturmIsolation.binary.WithAllocationContext(context.utility(z))} {
			value := unumeral.NewBinaryInt(7)
			if e := ops.Neg(&value); e == nil || value.Num.Val != 7 || value.KField != 0 {
				t.Fatal("failure transport acquired numeric negation", e, value)
			}
			if e := ops.Div2(&value); e == nil || value.Num.Val != 7 || value.KField != 0 {
				t.Fatal("failure transport published denominator", e, value)
			}
		}
		if context.sturmIsolationFrame(z) != nil || used != z.Budget().Used() || stats != z.RuntimeOwner().Stats() {
			t.Fatal("failure transport acquired isolation context")
		}
		return f
	}
	c, manager, runtime := algebraicNegTLSFixture(t, transport)
	m = manager
	q, e := m.beginSturmIsolation(UPolynomialView{}, c.config.Binary)
	serviceMust(t, e)
	q.constructCore()
	context = q.context()
	issued = q.binary.WithAllocationContext(context.utility(m.config.Integer))
	pad := algebraicCellSourceTLSBoundary(t, c)
	armed = true
	body := issued.PowerOfTwo(128, &q.lower, &q.copy)
	armed = false
	if body == nil || !probed || !unumeral.NumericAllocationPending(&q.numeric) {
		t.Fatal("actual power allocation refusal not owned", body, probed)
	}
	_ = q.finish(body)
	serviceMust(t, runtime.SetMaxSize(0))
	serviceMust(t, runtime.Deallocate(&pad))
	source := q.sourceStage
	serviceMust(t, m.CleanupSturmIsolation())
	if q.active || source != q.sourceStage {
		t.Fatal("power acknowledgement rewrote source history")
	}
	algebraicCellSourcePhysical(t, c, nil, nil, nil)
}

func TestUPolynomialSturmIsolationBinaryRejectsRetiredIssuedContext(t *testing.T) {
	c, m, _ := algebraicNegTLSFixture(t, nil)
	z, b := m.config.Integer, c.config.Binary
	q, e := m.beginSturmIsolation(UPolynomialView{}, b)
	serviceMust(t, e)
	q.constructCore()
	retired := q.context()
	issued := b.WithAllocationContext(retired.utility(z))
	serviceMust(t, q.finish(nil))
	q, e = m.beginSturmIsolation(UPolynomialView{}, b)
	serviceMust(t, e)
	q.constructCore()
	value := unumeral.NewBinaryInt(7)
	used := z.Budget().Used()
	e = issued.Div2(&value)
	if e == nil || value.Num.Val != 7 || value.KField != 0 || used != z.Budget().Used() || retired.sturmIsolationFrame(z) != nil {
		t.Error("retired issued context acquired new invocation", e, value)
	}
	serviceMust(t, q.finish(nil))
	algebraicCellSourcePhysical(t, c, nil, nil, nil)
}

func TestUPolynomialSturmNumericIssuedContextsKeepInvocationIdentity(t *testing.T) {
	for _, rootID := range []bool{false, true} {
		name := "public-sequence"
		if rootID {
			name = "root-id"
		}
		t.Run(name, func(t *testing.T) {
			c, m, _ := algebraicNegTLSFixture(t, nil)
			z, b := m.config.Integer, c.config.Binary
			sequence, e := m.NewSequence()
			serviceMust(t, e)
			var context integerAllocationInvocationContext
			if rootID {
				q, e := m.beginRootID(UPolynomialView{}, nil)
				serviceMust(t, e)
				context = q.context()
			} else {
				q, e := sequence.beginSturm()
				serviceMust(t, e)
				context = q.context()
			}
			issued := b.WithAllocationContext(context.utility(z))
			if rootID {
				serviceMust(t, m.rootID.finish(nil))
				_, e = m.beginRootID(UPolynomialView{}, nil)
				serviceMust(t, e)
			} else {
				serviceMust(t, sequence.sturm.finish(nil))
				_, e = sequence.beginSturm()
				serviceMust(t, e)
			}
			value := unumeral.NewBinaryInt(7)
			used := z.Budget().Used()
			var copy unumeral.IntegerCopyAllocationReceipt
			for _, e := range []error{issued.Div2(&value), issued.PowerOfTwo(4, &value, &copy)} {
				if e == nil {
					t.Error("retired operation acquired reused source owner")
				}
			}
			if value.Num.Val != 7 || value.KField != 0 || used != z.Budget().Used() || copy != (unumeral.IntegerCopyAllocationReceipt{}) {
				t.Error("refused retired operation changed source state")
			}
			if rootID {
				serviceMust(t, m.rootID.finish(nil))
			} else {
				serviceMust(t, sequence.sturm.finish(nil))
			}
			serviceMust(t, sequence.Close())
			algebraicCellSourcePhysical(t, c, nil, nil, nil)
		})
	}
}
