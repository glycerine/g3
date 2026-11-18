package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type lpMPSDriverClock struct {
	calls, failAt int
}

var lpErrMPSDriverClock = errors.New("MPS driver clock read refused")

func (c *lpMPSDriverClock) read() (uint64, error) {
	c.calls++
	if c.calls == c.failAt {
		return 0, lpErrMPSDriverClock
	}
	return uint64(2*c.calls-1) * 1000000000, nil
}

func lpMPSDriverFile(t *testing.T, text string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "model.mps")
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func lpMPSDriverModel(cx, cy, u, v, rhs int64) string {
	return "NAME DRIVER\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "COST", fmt.Sprint(cx)) + lpExactMPSPair("y", "COST", fmt.Sprint(cy)) + lpExactMPSPair("x", "R", "1") + lpExactMPSPair("y", "R", "1") + "RHS\n" + lpExactMPSPair("RHS1", "R", fmt.Sprint(rhs)) + "BOUNDS\n" + lpMPSBoundLine("UP", fmt.Sprint("x ", u)) + lpMPSBoundLine("UP", fmt.Sprint("y ", v)) + "ENDATA\n"
}

func lpMPSDriverArguments(t *testing.T, exact, dual, minimum, compare bool) *DriverArguments {
	t.Helper()
	args := []string{"lp-test", "--rep_frq", "0", "--max_iters", "1000"}
	for _, flag := range []struct {
		used bool
		name string
	}{{exact, "--mpq"}, {dual, "--dual"}, {minimum, "--min"}, {compare, "--compare_with_primal"}} {
		if flag.used {
			args = append(args, flag.name)
		}
	}
	return lpDriverArgs(t, args...)
}

func TestLPMPSDriverIndependentFileMinMax(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for world := 0; world < 40; world++ {
		cx, cy, u, v, rhs := int64(1+world%7), int64(1+world%9), int64(3+world%5), int64(5+world%4), int64(3+world%8)
		// Independent continuous knapsack oracle, with both box bounds.
		x, y := int64(0), int64(0)
		if cx >= cy {
			x = min(u, rhs)
			y = min(v, rhs-x)
		} else {
			y = min(v, rhs)
			x = min(u, rhs-y)
		}
		maximum := cx*x + cy*y
		name := lpMPSDriverFile(t, lpMPSDriverModel(cx, cy, u, v, rhs))
		for _, mode := range []string{"exact", "float-primal", "float-dual"} {
			for _, minimum := range []bool{false, true} {
				h := newLPPairHarness(t)
				clock := &lpMPSDriverClock{}
				var out bytes.Buffer
				exact, dual := mode == "exact", mode == "float-dual"
				d := NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, dual, minimum, true), h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
				h.must(d.Run())
				want := maximum
				if minimum {
					want = 0
				}
				costText := fmt.Sprint(want)
				if minimum && !exact && !dual {
					costText = "-0" // source negates the floating zero objective.
				}
				if d.Status != StatusOptimal || !d.closed || d.rationalReader != nil || d.floatReader != nil || d.work != nil || !strings.Contains(out.String(), "Status: OPTIMAL\n") || !strings.Contains(out.String(), "cost = "+costText+"\n") || !strings.Contains(out.String(), "processed in 0.002 seconds, running for ") {
					t.Fatal(world, mode, minimum, d.Status, out.String())
				}
				if exact {
					if clock.calls != 3 || !d.watch.Running() || strings.Contains(out.String(), "x=") || strings.Contains(out.String(), "primal cost") || !strings.HasPrefix(out.String(), "solving "+name+" in rationals \n") {
						t.Fatal("exact report profile", world, minimum, clock.calls, out.String())
					}
				} else {
					comparisonText := fmt.Sprint(want)
					if minimum {
						comparisonText = "-0"
					}
					if clock.calls != 2 || d.watch.Running() || !strings.Contains(out.String(), "x=") || !strings.Contains(out.String(), "primal cost = "+comparisonText+"\n") || !strings.Contains(out.String(), "  one iter for ") || strings.Contains(out.String(), "solving for dual") != dual {
						t.Fatal("floating report profile", world, minimum, clock.calls, out.String())
					}
					var gotX, gotY float64
					found := false
					for _, line := range strings.Split(out.String(), "\n") {
						// print_x walks m_columns: libstdc++ order is y, x
						// (AUDIT.md util_lp_06-F1).
						if strings.HasPrefix(line, "y=") {
							if _, err := fmt.Sscanf(line, "y=%f x=%f", &gotY, &gotX); err != nil {
								t.Fatal(err, line)
							}
							found = true
						}
					}
					if !found || gotX < 0 || gotX > float64(u) || gotY < 0 || gotY > float64(v) || gotX+gotY > float64(rhs) || float64(gotX*float64(cx))+float64(gotY*float64(cy)) != float64(want) {
						t.Fatal("independent reported model", world, minimum, gotX, gotY)
					}
				}
				if d.Run() != ErrExactVectorPending {
					t.Fatal("driver repeated a completed source call")
				}
				h.must(d.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPMPSDriverPercentSourceDomains(t *testing.T) {
	for _, tc := range []struct {
		a, b float64
		want bool
	}{{0, 0, true}, {0, 0.000000999, true}, {0, 0.000001, false}, {99, 100, true}, {98.999, 100, false}, {-99, -100, true}, {math.NaN(), 0, false}, {0, math.NaN(), true}, {1, math.NaN(), false}, {math.Inf(1), 7, true}, {math.Inf(1), math.Inf(1), false}, {math.Inf(1), math.Inf(-1), true}, {math.MaxFloat64, -math.MaxFloat64, false}} {
		if got := DriverValuesOnePercentClose(tc.a, tc.b); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestLPMPSDriverClockFailurePrefixes(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	for _, exact := range []bool{false, true} {
		last := 2
		if exact {
			last = 3
		}
		for failAt := 1; failAt <= last; failAt++ {
			h := newLPPairHarness(t)
			clock := &lpMPSDriverClock{failAt: failAt}
			var out bytes.Buffer
			d := NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, false, false, false), h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
			if err := d.Run(); err != lpErrMPSDriverClock || clock.calls != failAt || d.Run() != ErrExactVectorPending {
				t.Fatal(exact, failAt, err, clock.calls)
			}
			if exact && failAt > 1 {
				if !strings.HasSuffix(out.String(), "cost = 9\nprocessed in ") || d.Status != StatusOptimal || d.work != nil {
					t.Fatal("objective dies before clock query", failAt, out.String())
				}
			} else if strings.Contains(out.String(), "Status:") || d.Status != StatusUnknown {
				t.Fatal("clock failure before status publication", exact, failAt, out.String())
			}
			h.must(d.Close())
			if clock.calls != failAt {
				t.Fatal("cleanup performed extra clock reads")
			}
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPMPSDriverEntryAndSourceLimits(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	for _, profile := range []string{"nil-args", "bad-limit", "closed", "nil-manager", "missing-float", "missing-exact", "dual-exact", "iteration-limit", "fresh-tableau"} {
		h := newLPPairHarness(t)
		clock := &lpMPSDriverClock{}
		var out bytes.Buffer
		a := lpMPSDriverArguments(t, profile != "missing-float", profile == "dual-exact", false, false)
		if profile == "nil-args" {
			a = nil
		} else if profile == "bad-limit" {
			a.setValue("--max_iters", "bad")
		} else if profile == "iteration-limit" {
			a.setValue("--max_iters", "1")
		} else if profile == "fresh-tableau" {
			a.setValue("--simplex_strategy", "0")
		}
		file := name
		if strings.HasPrefix(profile, "missing-") {
			file = filepath.Join(t.TempDir(), "absent.mps")
		}
		d := NewMPSFileDriver(file, a, h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
		if profile == "closed" {
			h.must(d.Close())
		} else if profile == "nil-manager" {
			d.manager = nil
		}
		err := d.Run()
		switch profile {
		case "nil-args":
			if err != ErrBounds || out.Len() != 0 {
				t.Fatal(profile, err, out.String())
			}
		case "closed":
			if err != ErrExactVectorPending || out.Len() != 0 {
				t.Fatal(profile, err)
			}
		case "bad-limit":
			if err == nil || out.Len() != 0 || clock.calls != 0 {
				t.Fatal(profile, err, out.String())
			}
		case "nil-manager":
			if err != ErrExactVectorOwner || !strings.HasSuffix(out.String(), " in rationals \n") {
				t.Fatal(profile, err, out.String())
			}
		case "missing-float", "missing-exact":
			h.must(err)
			if !strings.Contains(out.String(), "cannot process "+file+"\n") || clock.calls != 0 || d.Status != StatusUnknown {
				t.Fatal(profile, out.String())
			}
		case "dual-exact":
			if sourceDebugEnabled {
				if err != ErrAssertion || d.rationalDual == nil || d.rationalDual.CoreSolver != nil {
					t.Fatal(profile, err)
				}
			} else if err != ErrBounds || d.rationalDual == nil || d.rationalDual.CoreSolver == nil || d.rationalDual.CoreSolver.pivotWork == nil {
				t.Fatal(profile, err)
			}
			if strings.Contains(out.String(), "Status:") || clock.calls != 1 {
				t.Fatal("dual source boundary precedes reports", out.String())
			}
		case "iteration-limit":
			h.must(err)
			if d.Status != StatusFeasible || d.TotalIterations != 2 || strings.Contains(out.String(), "cost = ") || !strings.Contains(out.String(), "Status: FEASIBLE\n") {
				t.Fatal(profile, d.Status, out.String())
			}
		case "fresh-tableau":
			h.must(err)
			if d.Status != StatusOptimal || !strings.Contains(out.String(), "cost = 0\n") {
				t.Fatal("retained uninitialized tableau accepts initial objective", err, out.String())
			}
		}
		h.must(d.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	// Constructor's default output is source stdout, without writing to it here.
	d := NewMPSFileDriver(name, nil, nil, MPSDriverConfig{})
	if d.out != os.Stdout {
		t.Fatal("native default report stream")
	}
}

func TestLPMPSDriverVariableReportThreshold(t *testing.T) {
	for _, count := range []int{19, 20} {
		input := "NAME MANY\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + lpExactMPSPair("x", "R", "1")
		for j := 1; j < count; j++ {
			input += lpExactMPSPair(fmt.Sprint("f", j), "COST", "1") + lpExactMPSPair(fmt.Sprint("f", j), "R", "1")
		}
		input += "RHS\n" + lpExactMPSPair("RHS1", "R", fmt.Sprint(count+2)) + "BOUNDS\n"
		for j := 1; j < count; j++ {
			input += lpMPSBoundLine("FX", fmt.Sprint("f", j, " 1"))
		}
		name := lpMPSDriverFile(t, input+"ENDATA\n")
		var out bytes.Buffer
		clock := &lpMPSDriverClock{}
		d := NewMPSFileDriver(name, lpMPSDriverArguments(t, false, false, false, false), nil, MPSDriverConfig{Output: &out, Clock: clock.read})
		lpStaticMust(t, d.Run())
		if d.Status != StatusOptimal || strings.Contains(out.String(), "x=") != (count < 20) || !strings.Contains(out.String(), fmt.Sprintf("cost = %d\n", count+2)) {
			t.Fatal(count, d.Status, out.String())
		}
	}
}

// Named buffer member prevents io.WriteString from bypassing fallible Write.
type lpMPSDriverActionWriter struct {
	buffer             bytes.Buffer
	action             func(string) error
	flushes, failFlush int
}

func (w *lpMPSDriverActionWriter) Write(p []byte) (int, error) {
	if w.action != nil {
		if err := w.action(string(p)); err != nil {
			return 0, err
		}
	}
	return w.buffer.Write(p)
}
func (w *lpMPSDriverActionWriter) Flush() error {
	w.flushes++
	if w.flushes == w.failFlush {
		return lpErrResidualWrite
	}
	return nil
}

func TestLPMPSDriverRationalCostLifetime(t *testing.T) {
	h := newLPPairHarness(t)
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	clock := &lpMPSDriverClock{}
	w := &lpMPSDriverActionWriter{}
	var d *MPSFileDriver
	changed, clockAfterDestruction := false, false
	w.action = func(text string) error {
		if text == "cost = " {
			if d.work == nil {
				t.Fatal("lexical objective absent before conversion")
			}
			lpSparseRationalCheck(t, h, &d.work.Data[0], big.NewRat(9, 1))
			c, _ := d.rationalPrimal.Columns.Get2(0)
			if err := c.SetCost(lpScalarBoundValue(h, "97")); err != nil {
				return err
			}
			changed = true
		}
		return nil
	}
	d = NewMPSFileDriver(name, lpMPSDriverArguments(t, true, false, false, false), h.m, MPSDriverConfig{Output: w, Clock: func() (uint64, error) {
		if clock.calls == 1 {
			clockAfterDestruction = d.work == nil
		}
		return clock.read()
	}})
	h.must(d.Run())
	if !changed || !clockAfterDestruction || !strings.Contains(w.buffer.String(), "cost = 9\nprocessed in ") {
		t.Fatal(changed, clockAfterDestruction, w.buffer.String())
	}
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPMPSDriverReportByteAndFlushFailures(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	for _, exact := range []bool{false, true} {
		h := newLPPairHarness(t)
		clock := &lpMPSDriverClock{}
		baseline := &lpMPSDriverActionWriter{}
		d := NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, !exact, false, !exact), h.m, MPSDriverConfig{Output: baseline, Clock: clock.read})
		h.must(d.Run())
		want, flushes := baseline.buffer.String(), baseline.flushes
		lpExactVectorNamedCensus(t, h, nil)
		for accepted := 0; accepted <= len(want); accepted++ {
			h := newLPPairHarness(t)
			clock := &lpMPSDriverClock{}
			w := &lpResidualFailWriter{remaining: accepted}
			d := NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, !exact, false, !exact), h.m, MPSDriverConfig{Output: lpResidualWriterOnly{writer: w}, Clock: clock.read})
			err := d.Run()
			if accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil || w.String() != want[:accepted] {
				t.Fatal("byte report refusal", exact, accepted, err, strconv.Quote(w.String()), strconv.Quote(want[:accepted]))
			}
			if d.Run() != ErrExactVectorPending {
				t.Fatal("failed driver retry")
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
		for flush := 1; flush <= flushes; flush++ {
			h := newLPPairHarness(t)
			clock := &lpMPSDriverClock{}
			w := &lpMPSDriverActionWriter{failFlush: flush}
			d := NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, !exact, false, !exact), h.m, MPSDriverConfig{Output: w, Clock: clock.read})
			if err := d.Run(); err != lpErrResidualWrite || w.flushes != flush || !strings.HasPrefix(want, w.buffer.String()) {
				t.Fatal("endl report refusal", exact, flush, err, w.buffer.String())
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

// Ensure the concrete driver's reporting API accepts standard writer surfaces.
var _ io.Writer = (*lpMPSDriverActionWriter)(nil)

func lpMPSDriverComparisonFixture(t *testing.T, profile string, out io.Writer) *MPSFileDriver {
	t.Helper()
	text := lpMPSDriverModel(3, 2, 6, 6, 3)
	if profile == "both-infeasible" {
		text = strings.Replace(text, "ENDATA", lpMPSBoundLine("LO", "x 7")+"ENDATA", 1)
	}
	d := NewMPSFileDriver("", lpMPSDriverArguments(t, false, false, false, false), nil, MPSDriverConfig{Output: out})
	d.floatReader = lpMPSRead(t, text)
	var err error
	d.floatPrimal, err = d.floatReader.CreatePrimalSolver()
	lpStaticMust(t, err)
	m := d.floatPrimal
	if profile == "status" {
		m.GetOrCreateColumnInfo(0).SetLowBound(7)
	} else if profile == "cost" {
		// Two actual optima for explicitly differing input objectives. The
		// original reader is unchanged, so its comparison owner maximizes 3x+2y.
		m.GetOrCreateColumnInfo(0).Cost = 1
	}
	lpStaticMust(t, m.SetupFromDriver(1000, 0, false, d.Arguments, nil))
	m.Settings.MessageOut, m.Settings.DebugOut = nil, nil
	lpStaticMust(t, m.FindMaximalSolution())
	d.maxIterations = 1000
	return d
}

func TestLPMPSDriverPreparedComparisonDiagnostics(t *testing.T) {
	for _, profile := range []string{"status", "cost", "both-infeasible"} {
		var out bytes.Buffer
		d := lpMPSDriverComparisonFixture(t, profile, &out)
		err := d.compareWithPrimal()
		if profile == "cost" && sourceDebugEnabled {
			if err != ErrAssertion {
				t.Fatal(profile, err)
			}
		} else {
			lpStaticMust(t, err)
		}
		switch profile {
		case "status":
			if d.floatPrimal.Status != StatusInfeasible || d.floatComparison.Status != StatusOptimal || !strings.HasSuffix(out.String(), "statuses are different: dual INFEASIBLE primal = OPTIMAL\n") {
				t.Fatal(profile, out.String())
			}
		case "cost":
			if !strings.HasSuffix(out.String(), "primal cost = 9\ndifferent values for y:0 and 3\ndifferent values for x:3 and 0\ny=0 x=3 \ndual cost is 6, but primal cost is 9\n") {
				t.Fatal(profile, out.String())
			}
		case "both-infeasible":
			if d.floatPrimal.Status != StatusInfeasible || d.floatComparison.Status != StatusInfeasible || strings.Contains(out.String(), "primal cost") || strings.Contains(out.String(), "different") {
				t.Fatal(profile, out.String())
			}
		}
		lpStaticMust(t, d.Close())
	}
}

func TestLPMPSDriverComparisonReportFailures(t *testing.T) {
	for _, profile := range []string{"cost", "status"} {
		baseline := &lpMPSDriverActionWriter{}
		d := lpMPSDriverComparisonFixture(t, profile, baseline)
		err := d.compareWithPrimal()
		terminal := error(nil)
		if profile == "cost" && sourceDebugEnabled {
			terminal = ErrAssertion
		}
		if err != terminal {
			t.Fatal(err)
		}
		want, flushes := baseline.buffer.String(), baseline.flushes
		lpStaticMust(t, d.Close())
		for accepted := 0; accepted <= len(want); accepted++ {
			w := &lpResidualFailWriter{remaining: accepted}
			d := lpMPSDriverComparisonFixture(t, profile, lpResidualWriterOnly{writer: w})
			err := d.compareWithPrimal()
			if accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != terminal || w.String() != want[:accepted] {
				t.Fatal(profile, accepted, err, w.String())
			}
			lpStaticMust(t, d.Close())
		}
		for flush := 1; flush <= flushes; flush++ {
			w := &lpMPSDriverActionWriter{failFlush: flush}
			d := lpMPSDriverComparisonFixture(t, profile, w)
			if err := d.compareWithPrimal(); err != lpErrResidualWrite || w.flushes != flush || !strings.HasPrefix(want, w.buffer.String()) {
				t.Fatal(profile, flush, err, w.buffer.String())
			}
			lpStaticMust(t, d.Close())
		}
	}
}

func TestLPMPSDriverNamedAndComparisonGuards(t *testing.T) {
	for _, profile := range []string{"missing-primary", "missing-comparison", "bad-row", "setup"} {
		var out bytes.Buffer
		d := lpMPSDriverComparisonFixture(t, "cost", &out)
		switch profile {
		case "missing-primary":
			d.floatPrimal.NamesToColumns.Del("x")
			if err := d.printFloatX(false); err == nil || err.Error() != "get_column_value_by_name x" || out.String() != "y=3 x=" { // m_columns order y, x (util_lp_06-F1)
				t.Fatal(profile, err, out.String())
			}
		case "missing-comparison":
			var err error
			d.floatComparison, err = d.floatReader.CreatePrimalSolver()
			lpStaticMust(t, err)
			d.floatComparison.NamesToColumns.Del("x")
			if err := d.compareFloatValues(); err == nil || err.Error() != "get_column_value_by_name x" || out.String() != "different values for y:0 and 3\n" { // y first (util_lp_06-F1)
				t.Fatal(profile, err, out.String())
			}
		case "bad-row":
			row, _ := d.floatReader.rows.Get2("R")
			row.typ = mpsCost
			if err := d.compareWithPrimal(); err != ErrMPSUnhandled || d.floatComparison == nil {
				t.Fatal(profile, err)
			}
		case "setup":
			d.Arguments.setValue("--density", "bad")
			if err := d.compareWithPrimal(); err == nil || d.floatComparison == nil || d.floatComparison.CoreSolver != nil {
				t.Fatal(profile, err)
			}
		}
		lpStaticMust(t, d.Close())
	}
}

func lpMPSDriverFixedObjective(t *testing.T, h *lpPairHarness, dual, minimum bool, out io.Writer) *MPSFileDriver {
	t.Helper()
	d := NewMPSFileDriver("", lpMPSDriverArguments(t, true, dual, minimum, false), h.m, MPSDriverConfig{Output: out})
	d.minimum, d.dual = minimum, dual
	var base *RationalLPSolver
	var err error
	if dual {
		d.rationalDual, err = NewRationalDualSimplex(h.m)
		if err == nil {
			base = d.rationalDual.RationalLPSolver
		}
	} else {
		d.rationalPrimal, err = NewRationalPrimalSimplex(h.m)
		if err == nil {
			base = d.rationalPrimal.RationalLPSolver
		}
	}
	h.must(err)
	h.must(base.SetFixedValue(0, lpScalarBoundValue(h, strings.Repeat("1234567890", 100)+"7/13")))
	cost := "3"
	if dual {
		cost = "-3" // the actual dual cost accessor returns the negated sum.
	}
	h.must(base.SetCostForColumn(0, lpScalarBoundValue(h, cost)))
	return d
}

func TestLPMPSDriverObjectivePhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, dual := range []bool{false, true} {
		for _, minimum := range []bool{false, true} {
			for _, direct := range []bool{false, true} {
				success, failures := false, 0
				for ordinal := int64(0); ordinal < 3000 && !success; ordinal++ {
					if !t.Run(fmt.Sprintf("dual%t/min%t/direct%t/%d", dual, minimum, direct, ordinal), func(t *testing.T) {
						var h *lpPairHarness
						if direct {
							h = lpNewDirectSparseHarness(t)
						} else {
							h = newLPPairHarness(t)
						}
						var out bytes.Buffer
						d := lpMPSDriverFixedObjective(t, h, dual, minimum, &out)
						h.budget.FailAfter(ordinal)
						err := d.reportRationalCost()
						h.budget.FailAfter(-1)
						if err == nil {
							success = true
							want := "cost = inf\n"
							if minimum {
								want = "cost = -inf\n"
							}
							if out.String() != want || d.work != nil {
								t.Fatal("actual objective conversion", out.String())
							}
						} else {
							failures++
							if !lpExactPriorityPhysicalRefusal(err) || d.work == nil {
								t.Fatal("objective refusal", err)
							}
						}
						h.must(d.Close())
						lpExactVectorNamedCensus(t, h, nil)
					}) {
						t.FailNow()
					}
				}
				if !success || direct && failures == 0 {
					t.Fatal("objective physical campaign", dual, minimum, direct, success, failures)
				}
			}
		}
	}
}

func TestLPMPSDriverFullRunPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	// This magnitude must overflow the driver's float64 cost display to inf.
	large := strings.Repeat("1234567890", 100) + "7/13"
	input := strings.Replace(lpMPSDriverModel(3, 2, 4, 5, 3), lpExactMPSPair("x", "COST", "3"), lpExactMPSPair("x", "COST", large), 1)
	name := lpMPSDriverFile(t, input)
	for _, direct := range []bool{false, true} {
		success, failures := false, 0
		for ordinal := int64(0); ordinal < 6000 && !success; ordinal++ {
			if !t.Run(fmt.Sprintf("direct%t/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				clock := &lpMPSDriverClock{}
				var out bytes.Buffer
				d := NewMPSFileDriver(name, lpMPSDriverArguments(t, true, false, false, false), h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
				h.budget.FailAfter(ordinal)
				err := d.Run()
				h.budget.FailAfter(-1)
				if err == nil {
					success = true
					if d.Status != StatusOptimal || !d.closed || !strings.Contains(out.String(), "cost = inf\n") {
						t.Fatal("full exact driver completion", d.Status, out.String())
					}
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) || d.Run() != ErrExactVectorPending {
						t.Fatal("full driver refusal", err)
					}
				}
				h.must(d.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}) {
				t.FailNow()
			}
		}
		if !success || direct && failures == 0 {
			t.Fatal("full driver physical campaign", direct, success, failures)
		}
	}
}

func TestLPMPSDriverPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	for _, phase := range []string{"run", "objective"} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			clock := &lpMPSDriverClock{}
			d := NewMPSFileDriver(name, lpMPSDriverArguments(t, true, false, true, false), h.m, MPSDriverConfig{Output: io.Discard, Clock: clock.read})
			if phase == "objective" {
				d = lpMPSDriverFixedObjective(t, h, false, true, io.Discard)
			}
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
				matched = true
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				if phase == "run" {
					_ = d.Run()
				} else {
					_ = d.reportRationalCost()
				}
				if d.calls.comparison != 0 || d.rationalPrimal != nil && (d.rationalPrimal.outerCalls.comparison != 0 || d.rationalPrimal.calls.comparison != 0) || d.rationalReader != nil && d.rationalReader.calls.comparison != 0 {
					t.Fatal("driver/child adopted caller comparison")
				}
			}
			h.must(d.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("driver cleanup consumed caller comparison")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("no actual older comparison", phase)
		}
	}
}

func TestLPMPSDriverEmptyAndInfeasibleFiles(t *testing.T) {
	for _, empty := range []bool{false, true} {
		input := "NAME EMPTY\nROWS\n N COST\nCOLUMNS\nRHS\nENDATA\n"
		if !empty {
			input = strings.Replace(lpMPSDriverModel(3, 2, 4, 5, 3), "ENDATA", lpMPSBoundLine("LO", "x 7")+"ENDATA", 1)
		}
		name := lpMPSDriverFile(t, input)
		for _, exact := range []bool{false, true} {
			for _, dual := range []bool{false, true} {
				h := newLPPairHarness(t)
				clock := &lpMPSDriverClock{}
				var out bytes.Buffer
				d := NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, dual, false, true), h.m, MPSDriverConfig{Output: &out, Clock: clock.read})
				h.must(d.Run())
				status, text, reads := StatusInfeasible, "INFEASIBLE", 2
				if empty {
					status, text = StatusEmpty, "EMPTY"
				}
				if exact {
					reads = 3
				}
				if d.Status != status || clock.calls != reads || strings.Contains(out.String(), "cost = ") || !strings.Contains(out.String(), "Status: "+text+"\n") {
					t.Fatal(empty, exact, dual, d.Status, out.String())
				}
				if empty && !exact && (d.TotalIterations != 0 || !strings.Contains(out.String(), "one iter for inf ms\n")) {
					t.Fatal("retained zero-iteration IEEE timing", d.TotalIterations, out.String())
				}
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}

func TestLPMPSDriverFileCloseFailure(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	for _, exact := range []bool{false, true} {
		h := newLPPairHarness(t)
		clock := &lpMPSDriverClock{}
		w := &lpMPSDriverActionWriter{}
		var d *MPSFileDriver
		closed := false
		w.action = func(text string) error {
			if strings.HasPrefix(text, "processed in ") && !closed {
				closed = true
				if exact {
					return d.rationalReader.file.Close()
				}
				return d.floatReader.file.Close()
			}
			return nil
		}
		d = NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, false, false, false), h.m, MPSDriverConfig{Output: w, Clock: clock.read})
		if err := d.Run(); !errors.Is(err, os.ErrClosed) || !closed || d.closed || !strings.Contains(w.buffer.String(), "processed in 0.002 seconds") {
			t.Fatal(exact, err, w.buffer.String())
		}
		h.must(d.Close())
		if d.rationalReader != nil || d.floatReader != nil {
			t.Fatal("file close retry retained owner")
		}
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPMPSDriverLiveReportEvaluation(t *testing.T) {
	name := lpMPSDriverFile(t, lpMPSDriverModel(3, 2, 4, 5, 3))
	for _, exact := range []bool{false, true} {
		for _, profile := range []string{"status", "iterations"} {
			h := newLPPairHarness(t)
			clock := &lpMPSDriverClock{}
			w := &lpMPSDriverActionWriter{}
			var d *MPSFileDriver
			changed := 0
			w.action = func(text string) error {
				if profile == "status" && text == "Status: " {
					if exact {
						d.rationalPrimal.Status = StatusFeasible
					} else {
						d.floatPrimal.Status = StatusFeasible
					}
					changed++
				} else if profile == "iterations" && strings.HasSuffix(text, "seconds, running for ") {
					if exact {
						d.rationalPrimal.TotalIterations = 4
					} else {
						d.floatPrimal.TotalIterations = 4
					}
					changed++
				} else if profile == "iterations" && !exact && strings.HasSuffix(text, "iterations  one iter for ") {
					d.floatPrimal.TotalIterations = 8
					changed++
				}
				return nil
			}
			d = NewMPSFileDriver(name, lpMPSDriverArguments(t, exact, false, false, false), h.m, MPSDriverConfig{Output: w, Clock: clock.read})
			h.must(d.Run())
			if profile == "status" {
				if changed != 1 || d.Status != StatusFeasible || !strings.Contains(w.buffer.String(), "Status: FEASIBLE\n") || strings.Contains(w.buffer.String(), "cost = ") {
					t.Fatal("status getter evaluated before output prefix", exact, w.buffer.String())
				}
			} else if exact {
				if changed != 1 || !strings.HasSuffix(w.buffer.String(), "processed in 0.002 seconds, running for 4 iterations\n") {
					t.Fatal("exact counter getter evaluated early", w.buffer.String())
				}
			} else if changed != 2 || !strings.HasSuffix(w.buffer.String(), "processed in 0.002 seconds, running for 4 iterations  one iter for 0.25 ms\n") {
				t.Fatal("floating live counter reads collapsed", w.buffer.String())
			}
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}
