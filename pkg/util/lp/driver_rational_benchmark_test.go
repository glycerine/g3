package lp

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func lpScalarBenchmarkOracle(seed, iterations uint32) ([]*big.Rat, *big.Rat, uint32) {
	state := seed
	next := func() uint32 { state = state*214013 + 2531011; return (state >> 16) & 32767 }
	inputs := make([]*big.Rat, 20)
	for i := range inputs {
		den := 1 + next()%100 // GCC draws mpq(n, d)'s denominator first
		n := next() % 100
		inputs[i] = big.NewRat(int64(n), int64(den))
	}
	result := new(big.Rat)
	for i := 0; i < 10; i++ {
		result.Add(result, new(big.Rat).Mul(inputs[i], inputs[10+i]))
	}
	result.Mul(result, new(big.Rat).SetInt64(int64(iterations)))
	return inputs, result, state
}

// Independent exact arithmetic is converted only for the source's report.
func lpBenchmarkNumberOracle(v *big.Rat) string {
	number, _ := v.Float64()
	return strconv.FormatFloat(number, 'g', 6, 64)
}

func lpBenchmarkClock() func() (uint64, error) {
	n := uint64(0)
	return func() (uint64, error) { n++; return n * 2e9, nil }
}

func TestLPRationalBenchmarkIndependentArithmeticAndInputs(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, plus := range []bool{false, true} {
		for seed := uint32(0); seed < 20; seed++ {
			t.Run(fmt.Sprintf("plus=%v/seed=%d", plus, seed), func(t *testing.T) {
				h := newLPPairHarness(t)
				iterations := []uint32{0, 1, 3, 7}[seed%4]
				inputs, want, state := lpScalarBenchmarkOracle(seed, iterations)
				var d *RationalBenchmarkDriver
				inspected := false
				w := &lpMPSDriverActionWriter{action: func(s string) error {
					if s == "operation with rationals no pairs " {
						for i, v := range inputs {
							lpSparseRationalCheck(t, h, &d.work.Data[i], v)
						}
						lpSparseRationalCheck(t, h, &d.work.Data[20], want)
						inspected = true
					}
					return nil
				}}
				d = NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: w, Clock: lpBenchmarkClock()})
				r := z3rand.New(seed)
				h.must(d.runIterations(r, iterations))
				report := "operation with rationals no pairs 2\n" + lpBenchmarkNumberOracle(want) + "\n"
				if !inspected || w.buffer.String() != report || d.ResultText != lpBenchmarkNumberOracle(want) || d.CompletedIterations != iterations || d.Coefficients != 10 || d.Values != 10 || d.work != nil || !d.closed || d.calls.pending() {
					t.Fatal(d, w.buffer.String(), report)
				}
				state = state*214013 + 2531011
				if r.Next() != (state>>16)&32767 {
					t.Fatal("shared RNG order")
				}
				if d.runIterations(r, 1) != ErrExactVectorPending {
					t.Fatal("repeat")
				}
				h.must(d.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

func TestLPRationalBenchmarkReportFailureAndFlushPrefixes(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	_, want, _ := lpScalarBenchmarkOracle(17, 1)
	report := "operation with rationals no pairs 2\n" + lpBenchmarkNumberOracle(want) + "\n"
	for _, plus := range []bool{false, true} {
		for _, cause := range []error{nil, lpErrResidualWrite} {
			for n := 0; n < len(report); n++ {
				h := newLPPairHarness(t)
				w := &larConstraintPrefixWriter{remaining: n, err: cause}
				d := NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: w, Clock: lpBenchmarkClock()})
				err := d.runIterations(z3rand.New(17), 1)
				expected := cause
				if expected == nil {
					expected = io.ErrShortWrite
				}
				if !errors.Is(err, expected) || w.output.String() != report[:n] || d.CompletedIterations != 1 || d.closed || d.work == nil || d.watch.Running() {
					t.Fatal(plus, n, err, w.output.String())
				}
				hasResult := n >= len("operation with rationals no pairs 2\n")
				if (d.ResultText != "") != hasResult {
					t.Fatal("result conversion publication", n, d.ResultText)
				}
				h.must(d.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
		for flush := 1; flush <= 2; flush++ {
			h := newLPPairHarness(t)
			w := &lpDriverFlushWriter{failAt: flush}
			d := NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: w, Clock: lpBenchmarkClock()})
			err := d.runIterations(z3rand.New(17), 1)
			prefix := strings.IndexByte(report, '\n') + 1
			if flush == 2 {
				prefix = len(report)
			}
			if !errors.Is(err, lpErrResidualWrite) || w.String() != report[:prefix] || w.flushes != flush || d.closed {
				t.Fatal(plus, flush, err, w.String())
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPRationalBenchmarkClockAndOwnerBoundaries(t *testing.T) {
	failure := errors.New("benchmark clock")
	for _, plus := range []bool{false, true} {
		for failAt := 1; failAt <= 2; failAt++ {
			h := newLPPairHarness(t)
			calls := 0
			clock := func() (uint64, error) {
				calls++
				if calls == failAt {
					return 0, failure
				}
				return uint64(calls) * 1e9, nil
			}
			w := &lpDriverFlushWriter{}
			d := NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: w, Clock: clock})
			err := d.runIterations(z3rand.New(23), 3)
			if !errors.Is(err, failure) || calls != failAt || w.Len() != 0 || d.Coefficients != 10 || d.Values != 10 || d.closed || d.CompletedIterations != uint32((failAt-1)*3) || d.watch.Running() != (failAt == 2) {
				t.Fatal(plus, failAt, d, err)
			}
			h.must(d.Close())
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
	d := NewRationalBenchmarkDriver(nil, false, MPSDriverConfig{Output: io.Discard})
	if d.Run(z3rand.New(0)) != ErrBounds {
		t.Fatal("nil manager")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	h := newLPPairHarness(t)
	d = NewRationalBenchmarkDriver(h.m, false, MPSDriverConfig{Output: io.Discard})
	if d.Run(nil) != ErrBounds || d.work != nil {
		t.Fatal("nil random")
	}
	h.must(d.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalBenchmarkPhysicalRefusalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, direct := range []bool{false, true} {
		for _, plus := range []bool{false, true} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 1500 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct=%v/plus=%v/refusal=%d", direct, plus, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					d := NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: io.Discard, Clock: lpBenchmarkClock()})
					h.budget.FailAfter(ordinal)
					err := d.runIterations(z3rand.New(19), 1)
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
						_, want, _ := lpScalarBenchmarkOracle(19, 1)
						if d.ResultText != lpBenchmarkNumberOracle(want) || !d.closed {
							t.Fatal("completed result", d.ResultText)
						}
					} else {
						if !lpExactPriorityPhysicalRefusal(err) || d.closed {
							t.Fatal(err)
						}
						failures++
						if d.runIterations(z3rand.New(19), 1) != ErrExactVectorPending {
							t.Fatal("failed owner reentered")
						}
					}
					h.must(d.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("missing physical campaign completion", direct, plus, done, failures)
			}
		}
	}
}

func TestLPRationalBenchmarkPreservesOlderComparison(t *testing.T) {
	for _, plus := range []bool{false, true} {
		found := false
		for ordinal := int64(0); ordinal < 40 && !found; ordinal++ {
			h := newLPPairHarness(t)
			v := h.pairExactLarge(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
			var caller exactEtaCalls
			before, err := caller.begin(h.m)
			h.must(err)
			h.budget.FailAfter(ordinal)
			_, err = h.m.Compare(&v.X, &v.Y)
			h.budget.FailAfter(-1)
			err = caller.after(h.m, before, err)
			older := h.m.ComparisonInvocationState()
			if older.Active {
				found = true
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				d := NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: io.Discard, Clock: lpBenchmarkClock()})
				// This actual stopped comparison leaves shared manager scratch
				// unavailable to the benchmark's normalization. Transport that
				// refusal without acquiring the older caller's receipt.
				if err := d.runIterations(z3rand.New(19), 3); !errors.Is(err, arena.ErrInvalidSlot) {
					t.Fatal("stopped-manager transport", err)
				}
				if d.calls.comparison != 0 || h.m.ComparisonInvocationState() != older {
					t.Fatal("adopted or consumed older caller comparison")
				}
				h.must(d.Close())
				if h.m.ComparisonInvocationState() != older {
					t.Fatal("cleanup consumed older caller comparison")
				}
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !found {
			t.Fatal(plus, "no actual older caller comparison")
		}
	}
}

// Run explicitly with -run '^$' -bench '^BenchmarkLPRationalSource500000$'
// -benchtime=1x. Each sample executes the complete registered source count.
func BenchmarkLPRationalSource500000(b *testing.B) {
	for _, plus := range []bool{false, true} {
		name := "compound"
		if plus {
			name = "binary_plus"
		}
		b.Run(name, func(b *testing.B) {
			budget := arena.NewBudget(8 << 20)
			m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
				Cells: arena.Config{PageSize: 4, Budget: budget},
				Words: arena.VectorConfig{PageSize: 32, Descriptors: arena.Config{PageSize: 4, Budget: budget}},
			}, Synchronized: true})
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := m.CloseChecked(); err != nil {
					b.Error(err)
				}
				if budget.Used() != 0 {
					b.Error("retained bytes", budget.Used())
				}
			})
			_, want, _ := lpScalarBenchmarkOracle(19, 500000)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var d *RationalBenchmarkDriver
				checked := false
				out := &lpMPSDriverActionWriter{action: func(text string) error {
					if text == "operation with rationals no pairs " {
						actual, e := m.String(&d.work.Data[20])
						if e != nil {
							b.Fatal(e)
						}
						if actual != want.RatString() {
							b.Fatal("independent exact accumulator", actual, want)
						}
						checked = true
					}
					return nil
				}}
				d = NewRationalBenchmarkDriver(m, plus, MPSDriverConfig{Output: out})
				b.Cleanup(func() {
					if err := d.Close(); err != nil {
						b.Error(err)
					}
				})
				if err := d.Run(z3rand.New(19)); err != nil {
					b.Fatal(err)
				}
				if !checked || d.CompletedIterations != 500000 || d.ResultText != lpBenchmarkNumberOracle(want) || !d.closed {
					b.Fatal("registered count/result/cleanup", d.CompletedIterations, d.ResultText)
				}
			}
		})
	}
}

func TestLPNumericBenchmarkLiteralApproximateReports(t *testing.T) {
	// Literals independently calculated from the source LCG and exact fractions,
	// drawing each mpq denominator first as GCC does (AUDIT.md util_lp_02-F1).
	for _, tc := range []struct {
		seed, iterations uint32
		want             string
	}{{0, 0, "0"}, {1, 1, "14.467"}, {19, 3, "157.242"}} {
		for _, plus := range []bool{false, true} {
			h := newLPPairHarness(t)
			out := &lpDriverFlushWriter{}
			d := NewRationalBenchmarkDriver(h.m, plus, MPSDriverConfig{Output: out, Clock: lpBenchmarkClock()})
			h.must(d.runIterations(z3rand.New(tc.seed), tc.iterations))
			if d.ResultText != tc.want || out.String() != "operation with rationals no pairs 2\n"+tc.want+"\n" {
				t.Fatal(tc, plus, d.ResultText, out.String())
			}
			lpExactVectorNamedCensus(t, h, nil)
		}
		h := newLPPairHarness(t)
		out := &lpDriverFlushWriter{}
		d := NewPairBenchmarkDriver(h.m, MPSDriverConfig{Output: out, Clock: lpBenchmarkClock()})
		h.must(d.runIterations(z3rand.New(tc.seed), tc.iterations))
		if d.ResultText != tc.want || !strings.HasSuffix(out.String(), "operation with rationals 2\n"+tc.want+"\n") {
			t.Fatal(tc, d.ResultText, out.String())
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

// AUDIT.md util_lp_02-F1: GCC evaluates mpq(my_random()%100,
// 1 + my_random()%100)'s arguments right to left, so each pair draws its
// denominator first. The literals come from build/test-z3 lp --test_mpq,
// --test_mpq_np and --test_mpq_np_plus (seed 0), not from an oracle.
func TestLPBenchmarkInputsMatchGCCArgumentOrder(t *testing.T) {
	h := newLPPairHarness(t)
	var d *RationalBenchmarkDriver
	inspected := false
	w := &lpMPSDriverActionWriter{action: func(s string) error {
		if s == "operation with rationals no pairs " {
			lpSparseRationalCheck(t, h, &d.work.Data[0], big.NewRat(19, 39))
			lpSparseRationalCheck(t, h, &d.work.Data[5], new(big.Rat))
			lpSparseRationalCheck(t, h, &d.work.Data[10], big.NewRat(13, 11))
			lpSparseRationalCheck(t, h, &d.work.Data[19], big.NewRat(67, 57))
			inspected = true
		}
		return nil
	}}
	d = NewRationalBenchmarkDriver(h.m, false, MPSDriverConfig{Output: w, Clock: lpBenchmarkClock()})
	h.must(d.runIterations(z3rand.New(0), 1))
	if !inspected {
		t.Fatal("inputs not inspected")
	}
	h.must(d.Close())

	pw := &lpMPSDriverActionWriter{action: func(string) error { return nil }}
	p := NewPairBenchmarkDriver(h.m, MPSDriverConfig{Output: pw, Clock: lpBenchmarkClock()})
	h.must(p.runIterations(z3rand.New(0), 1))
	if prefix := "x = (13/11, 0) (3/28, 0) (23/58, 0) (20/41, 0) (3, 0) (90/47, 0) (87/41, 0) (37/8, 0) (17/12, 0) (67/57, 0) \n"; !strings.HasPrefix(pw.buffer.String(), prefix) {
		t.Fatalf("pair report %q", pw.buffer.String())
	}
	h.must(p.Close())

	// The registered counts' result lines, from the native driver.
	_, np, _ := lpScalarBenchmarkOracle(0, 500000)
	_, pairs, _ := lpScalarBenchmarkOracle(0, 1000000)
	if got := lpBenchmarkNumberOracle(np); got != "6.41439e+06" {
		t.Fatal("--test_mpq_np", got)
	}
	if got := lpBenchmarkNumberOracle(pairs); got != "1.28288e+07" {
		t.Fatal("--test_mpq", got)
	}
	lpExactVectorNamedCensus(t, h, nil)
}
