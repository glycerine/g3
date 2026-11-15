package polynomial

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"strconv"
	"testing"
)

func TestUPolynomialSequenceSturmDerivativeReachedReservations(t *testing.T) {
	success, failed, owned := false, false, false
	for after := int64(0); after < 256 && !success; after++ {
		t.Run(strconv.FormatInt(after, 10), func(t *testing.T) {
			c, m, _ := algebraicNegTLSFixture(t, nil)
			z := m.config.Integer
			p := upolySourceNew(t, m, nil)
			var input [4]unumeral.Int
			for i, text := range []string{"17", "18446744073709551617", "18446744073709551618", "18446744073709551619"} {
				serviceMust(t, z.SetString(&input[i], text))
			}
			serviceMust(t, m.SetInts(p, input[:]))
			for i := range input {
				serviceMust(t, z.Del(&input[i]))
			}
			v, e := m.wholeView(p)
			serviceMust(t, e)
			s, e := m.NewSequence()
			serviceMust(t, e)
			q, e := s.beginSturm()
			serviceMust(t, e)
			serviceMust(t, q.constructPrime())
			serviceMust(t, q.pushCopy(v))
			z.Budget().FailAfter(after)
			body := m.derivativeStateWithSourceContext(v, &q.prime, q.context())
			z.Budget().FailAfter(-1)
			if unumeral.NumericAllocationPending(&q.numeric) {
				owned = true
			}
			if body == nil {
				success = true
				if q.prime.size != 3 {
					t.Fatal("derivative size", q.prime.size)
				}
				for i, text := range []string{"18446744073709551617", "36893488147419103236", "55340232221128654857"} {
					var expected unumeral.Int
					serviceMust(t, z.SetString(&expected, text))
					equal, e := z.Equal(&m.values(&q.prime)[i], &expected)
					serviceMust(t, e)
					serviceMust(t, z.Del(&expected))
					if !equal {
						t.Fatal("independent derivative coefficient", i, text)
					}
				}
			} else {
				failed = true
				if !numericPhysicalReservationRefusal(body) {
					t.Fatal(body)
				}
			}
			_ = q.finish(body)
			source := q.sourceCursor
			serviceMust(t, s.CleanupSturm())
			if q.active || q.pending() || q.sourceCursor != source {
				t.Fatal("derivative cleanup lost exact ownership/source history")
			}
			serviceMust(t, s.Close())
			algebraicCellSourcePhysical(t, c, []UPolynomial{p}, nil, nil)
		})
	}
	if !success || !failed || !owned {
		t.Fatal("derivative allocation campaign incomplete", success, failed, owned)
	}
}

func TestUPolynomialSequenceSturmDerivativeFailureTransportRejectsContext(t *testing.T) {
	var m *UPolynomialManager
	var s *UPolynomialSequence
	var v UPolynomialView
	armed, probed := false, false
	transport := func(f umemory.Failure) error {
		if !armed {
			return f
		}
		armed = false
		probed = true
		z, q := m.config.Integer, &s.sturm
		used, stats := z.Budget().Used(), z.RuntimeOwner().Stats()
		context := q.context()
		probe := unumeral.NewInt(0)
		for _, ctx := range []unumeral.NumericAllocationContext{context.utility(z), unumeral.NumericAllocationContext{}.WithCaller(&q.hooks)} {
			if e := z.WithAllocationContext(ctx).SetUint32(&probe, 7); e == nil || probe.Val != 0 || probe.Cell != 0 {
				t.Error("failure transport acquired derivative numeric policy", e, probe)
			}
		}
		var untouched upolyVectorState
		if e := m.derivativeStateWithSourceContext(v, &untouched, context); !errors.Is(e, ErrUPolynomialOwner) || untouched != (upolyVectorState{}) {
			t.Error("failure transport reserved derivative storage", e)
		}
		if used != z.Budget().Used() || stats != z.RuntimeOwner().Stats() {
			t.Error("refused derivative callback changed physical resources")
		}
		return f
	}
	c, owner, runtime := algebraicNegTLSFixture(t, transport)
	m = owner
	p := upolySourceNew(t, m, nil)
	z := m.config.Integer
	var input [2]unumeral.Int
	serviceMust(t, z.SetString(&input[1], "18446744073709551617"))
	serviceMust(t, m.SetInts(p, input[:]))
	serviceMust(t, z.Del(&input[1]))
	var e error
	v, e = m.wholeView(p)
	serviceMust(t, e)
	s, e = m.NewSequence()
	serviceMust(t, e)
	q, e := s.beginSturm()
	serviceMust(t, e)
	serviceMust(t, q.constructPrime())
	serviceMust(t, q.pushCopy(v))
	pad := algebraicCellSourceTLSBoundary(t, c)
	armed = true
	body := m.derivativeStateWithSourceContext(v, &q.prime, q.context())
	if body == nil || !probed {
		t.Fatal("actual derivative failure callback not reached", body)
	}
	_ = q.finish(body)
	serviceMust(t, runtime.SetMaxSize(0))
	serviceMust(t, runtime.Deallocate(&pad))
	serviceMust(t, s.CleanupSturm())
	serviceMust(t, s.Close())
	algebraicCellSourcePhysical(t, c, []UPolynomial{p}, nil, nil)
}

func TestUPolynomialSequenceSturmDerivativePublicReachedReservations(t *testing.T) {
	for _, fourier := range []bool{false, true} {
		name := "sturm"
		if fourier {
			name = "fourier"
		}
		t.Run(name, func(t *testing.T) {
			success, derivativeDebt := false, false
			for after := int64(0); after < 1024 && !success; after++ {
				t.Run(strconv.FormatInt(after, 10), func(t *testing.T) {
					c, m, _ := algebraicNegTLSFixture(t, nil)
					z := m.config.Integer
					p := upolySourceNew(t, m, nil)
					var input [4]unumeral.Int
					for i, text := range []string{"17", "18446744073709551617", "18446744073709551618", "18446744073709551619"} {
						serviceMust(t, z.SetString(&input[i], text))
					}
					serviceMust(t, m.SetInts(p, input[:]))
					for i := range input {
						serviceMust(t, z.Del(&input[i]))
					}
					v, e := m.wholeView(p)
					serviceMust(t, e)
					s, e := m.NewSequence()
					serviceMust(t, e)
					z.Budget().FailAfter(after)
					var body error
					if fourier {
						body = m.FourierSequence(v, s)
					} else {
						body = m.SturmSequence(v, s)
					}
					z.Budget().FailAfter(-1)
					q := &s.sturm
					// Actual first derivative, before publication of row1 or any normalize/
					// remainder constructor. Only a real retained numerical root qualifies.
					if len(s.state.sizes) == 1 && q.normalization.invocation == 0 && q.scalar.invocation == 0 && unumeral.NumericAllocationPending(&q.numeric) {
						derivativeDebt = true
					}
					if body == nil {
						success = true
						row, e := s.RowView(1)
						serviceMust(t, e)
						if row.count != 3 {
							t.Fatal("first derivative row size", row.count)
						}
						for i, text := range []string{"18446744073709551617", "36893488147419103236", "55340232221128654857"} {
							coefficient, e := m.read(row, uint32(i))
							serviceMust(t, e)
							var expected unumeral.Int
							serviceMust(t, z.SetString(&expected, text))
							equal, e := z.Equal(coefficient, &expected)
							serviceMust(t, e)
							serviceMust(t, z.Del(&expected))
							if !equal {
								t.Fatal("independent derivative row", i, text)
							}
						}
					} else if !numericPhysicalReservationRefusal(body) {
						t.Fatal(body)
					}
					source := q.sourceCursor
					serviceMust(t, s.CleanupSturm())
					if q.active || q.pending() || source != q.sourceCursor || m.sturmPending != nil {
						t.Fatal("public derivative acknowledgement lost scope ownership")
					}
					serviceMust(t, s.Close())
					algebraicCellSourcePhysical(t, c, []UPolynomial{p}, nil, nil)
				})
			}
			if !success || !derivativeDebt {
				t.Fatal("public derivative campaign incomplete", success, derivativeDebt)
			}
		})
	}
}

// Isolate the source RAW numeral producer/retirement at the uint32 BIG
// boundary. This does not claim to execute a degree-2^31 derivative or forge
// an array extent; both numerals are acquired by real source SetUint32 calls.
func TestUPolynomialSequenceSturmDerivativeNativeMultiplierLifetime(t *testing.T) {
	for _, refuseRetirement := range []bool{false, true} {
		name := "two-raw-values"
		if refuseRetirement {
			name = "retirement-refusal"
		}
		t.Run(name, func(t *testing.T) {
			c, m, _ := algebraicNegTLSFixture(t, nil)
			z := m.config.Integer
			s, e := m.NewSequence()
			serviceMust(t, e)
			q, e := s.beginSturm()
			serviceMust(t, e)
			serviceMust(t, m.modular.WithAllocationContext(integerAllocationContextRef(q.context()).utility(z)).SetUint32(&q.derivative, uint32(1)<<31))
			first := q.derivative.Cell
			if !unumeral.ValidNumericRef(z.Cells, first) || z.Cell(first).Released {
				t.Fatal("RAW UINT producer did not allocate actual BIG numeral")
			}
			source, used, stats := q.sourceCursor, z.Budget().Used(), z.RuntimeOwner().Stats()
			var body error
			if refuseRetirement {
				z.Budget().FailAfter(0)
				body = q.retireDerivativeMultiplier()
				z.Budget().FailAfter(-1)
				if !errors.Is(body, arena.ErrInjectedFailure) || q.derivative.Cell != first || q.derivativeRetired != nil || used != z.Budget().Used() || stats != z.RuntimeOwner().Stats() {
					t.Fatal("retirement refusal lost actual RAW owner", body)
				}
			} else {
				serviceMust(t, q.retireDerivativeMultiplier())
				if q.derivative.Cell != 0 || q.derivativeRetired == nil || q.derivativeRetired.value.Cell != first || z.Cell(first).Released || stats != z.RuntimeOwner().Stats() {
					t.Fatal("retirement performed an invented source DEL")
				}
				serviceMust(t, m.modular.WithAllocationContext(integerAllocationContextRef(q.context()).utility(z)).SetUint32(&q.derivative, (uint32(1)<<31)+1))
				if q.derivative.Cell == first || !unumeral.ValidNumericRef(z.Cells, q.derivative.Cell) {
					t.Fatal("next source declaration reused owning numeral")
				}
				value, e := z.Uint64(&q.derivativeRetired.value)
				serviceMust(t, e)
				if value != uint64(1)<<31 {
					t.Fatal("retired RAW value changed", value)
				}
			}
			_ = q.finish(body)
			if !q.active || !unumeral.ValidNumericRef(z.Cells, first) || z.Cell(first).Released {
				t.Fatal("plain RAW received a source destructor")
			}
			serviceMust(t, s.CleanupSturm())
			if q.active || q.derivative.Cell != 0 || q.derivativeRetired != nil || source != q.sourceCursor || m.sturmPending != nil {
				t.Fatal("RAW owning cleanup changed source history")
			}
			if unumeral.ValidNumericRef(z.Cells, first) && !z.Cell(first).Released {
				t.Fatal("retired actual RAW backing leaked")
			}
			serviceMust(t, s.Close())
			algebraicCellSourcePhysical(t, c, nil, nil, nil)
		})
	}
}
