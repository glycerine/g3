package lp

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
)

type lpFeasibleNames struct{}

func (lpFeasibleNames) ColumnName(uint32) string { return "v" }

func lpCaptureFeasibleStdout(t *testing.T, call func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	lpStaticMust(t, err)
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old; r.Close(); w.Close() }()
	call()
	lpStaticMust(t, w.Close())
	text, err := io.ReadAll(r)
	lpStaticMust(t, err)
	return string(text)
}

func TestLPFloatPrimalFindFeasibleIndependentSystems(t *testing.T) {
	rng := rand.New(rand.NewSource(89112))
	feasible, driftStops := 0, 0
	for world := 0; world < 160; world++ {
		// -a*x+s=b, with x,s>=0 and b<0. Initial slack basis is
		// infeasible; x=max(-b/a) independently proves feasibility.
		a := [][]float64{{-float64(1 + rng.Intn(8)), 1, 0}, {-float64(1 + rng.Intn(8)), 0, 1}}
		b := []float64{-float64(1 + rng.Intn(16)), -float64(1 + rng.Intn(16))}
		witness := math.Max(b[0]/a[0][0], b[1]/a[1][0])
		for i := range b {
			if b[i]-float64(a[i][0]*witness) < -1e-12 {
				t.Fatal("invalid independent witness")
			}
		}
		m := lpSolveStandardFixture(t, a, b, []float64{7, 2, 3}, []float64{0, b[0], b[1]}, []uint32{1, 2})
		if world%2 == 0 {
			m.Settings.DensityThreshold = 0
		} else {
			m.Settings.DensityThreshold = math.Inf(1)
		}
		m.Status = StatusUnbounded
		lpStaticMust(t, m.FindFeasibleSolution())
		// The source updates basic infeasibility costs after the first pivot,
		// then updates reduced costs using the old entering reduced cost.
		// For different crossings the leaving reduced cost changes from
		// (a0+a1)/aLeaving to aOther/aLeaving. Its drift exceeds half
		// the old value exactly when aLeaving>aOther; feasibility-only
		// advance reports FLOATING_POINT_ERROR instead of refreshing.
		q0, q1 := b[0]/a[0][0], b[1]/a[1][0]
		leaving, other := 0, 1
		if q1 < q0 {
			leaving, other = 1, 0
		}
		wantDriftStop := q0 != q1 && -a[leaving][0] > -a[other][0]
		if wantDriftStop {
			driftStops++
			if m.Status != StatusFloatingPointError || m.TotalIterations != 2 || m.CurrentXIsFeasible() {
				t.Fatal("source drift stop", world, m.Status, m.TotalIterations, *m.X)
			}
		} else {
			feasible++
			if !m.CurrentXIsFeasible() || m.Status != StatusFeasible {
				t.Fatal("independent feasible system", world, m.Status, *m.X)
			}
		}
		if !m.LookForFeasibleSolutionOnly || m.TotalIterations == 0 {
			t.Fatal(world, m.Status, m.TotalIterations, a, b, *m.X, *m.Basis, *m.Costs, m.D)
		}
		for j, v := range *m.X {
			if !wantDriftStop && v < -1e-8 || math.IsNaN(v) {
				t.Fatal(world, j, v)
			}
		}
		for i, row := range a {
			sum := 0.0
			for j, v := range row {
				sum += float64(v * (*m.X)[j])
			}
			if math.Abs(sum-b[i]) > 1e-8 {
				t.Fatal(world, i, sum, b[i])
			}
		}
	}
	if feasible == 0 || driftStops == 0 {
		t.Fatal(feasible, driftStops)
	}
	t.Logf("%d independently feasible exits, %d independently predicted source drift stops", feasible, driftStops)
	m := lpFloatPrimalCore(t, 1, 1)
	(*m.X)[0], (*m.B)[0], (*m.ColumnTypes)[0] = -1, -1, ColumnLowBound
	lpStaticMust(t, m.FindFeasibleSolution())
	if m.Status != StatusInfeasible {
		t.Fatal(m.Status)
	}
	n := lpFloatPrimalCore(t, 0, 1)
	n.Status = StatusUnbounded
	lpStaticMust(t, n.FindFeasibleSolution())
	if n.Status != StatusFeasible || n.TotalIterations != 0 {
		t.Fatal(n.Status)
	}
}

func TestLPFloatPrimalFeasibleCheckAndDiagnostic(t *testing.T) {
	m := lpFloatPrimalCore(t, 0, 1)
	m.Names = lpFeasibleNames{}
	(*m.LowBounds)[0], (*m.UpperBounds)[0] = -2, 3
	for _, typ := range []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed} {
		(*m.ColumnTypes)[0], (*m.X)[0] = typ, 1
		ok, err := m.NonBasicColumnsAreSetCorrectly()
		if err != nil || !ok {
			t.Fatal(typ, ok, err)
		}
		var out bytes.Buffer
		stdout := lpCaptureFeasibleStdout(t, func() { lpStaticMust(t, m.PrintColumnInfo(0, &out)) })
		typeName, _ := ColumnTypeString(typ)
		want := "column_index = 0, name = v type = " + typeName + "\n"
		switch typ {
		case ColumnLowBound:
			want += "-2\n"
		case ColumnUpperBound:
			want += "3\n"
		case ColumnBoxed, ColumnFixed:
			want += "(-2, 3)\n"
		}
		if out.String() != want || stdout != "basis heading = -1\nx = 1\n" {
			t.Fatal(typ, out.String(), stdout)
		}
	}
	(*m.ColumnTypes)[0], (*m.X)[0] = ColumnLowBound, -3
	stdout := lpCaptureFeasibleStdout(t, func() {
		ok, err := m.NonBasicColumnsAreSetCorrectly()
		if ok || err != nil {
			t.Fatal(ok, err)
		}
	})
	if stdout != "column_index = 0, name = v type = low_bound\n-2\nbasis heading = -1\nx = -3\n" {
		t.Fatal(stdout)
	}
	m.Names = nil
	if ok, err := m.NonBasicColumnsAreSetCorrectly(); ok || err != ErrBounds {
		t.Fatal(ok, err)
	}
	m.Names = lpFeasibleNames{}
	// The debug check happens after the mode change, before status reset/Solve.
	if sourceDebugEnabled {
		m.Status = StatusOptimal
		lpCaptureFeasibleStdout(t, func() {
			if err := m.FindFeasibleSolution(); err != ErrAssertion {
				t.Fatal(err)
			}
		})
		if !m.LookForFeasibleSolutionOnly || m.Status != StatusOptimal || m.CostsBackup != nil {
			t.Fatal("failed precheck publication")
		}
		*m.NBasis = []uint32{9}
		if err := m.FindFeasibleSolution(); err != ErrBounds {
			t.Fatal(err)
		}
	}
	// In release the SASSERT operand is not evaluated at all.
	if !sourceDebugEnabled {
		n := lpFloatPrimalCore(t, 0, 1)
		(*n.ColumnTypes)[0], (*n.X)[0] = ColumnLowBound, -3
		lpStaticMust(t, n.FindFeasibleSolution())
		if n.Status != StatusFeasible {
			t.Fatal(n.Status)
		}
	}
	// Propagate a real InitRun failure after resetting the previous status.
	n := lpFloatPrimalCore(t, 1, 1)
	n.Status = StatusOptimal
	*n.Heading = nil
	if err := n.FindFeasibleSolution(); err != ErrBounds {
		t.Fatal(err)
	}
	if n.Status != StatusUnknown || !n.LookForFeasibleSolutionOnly {
		t.Fatal("Solve failure publication")
	}
}

func TestLPFloatPrimalFeasibleDiagnosticFailurePrefixes(t *testing.T) {
	m := lpFloatPrimalCore(t, 0, 1)
	m.Names = lpFeasibleNames{}
	(*m.ColumnTypes)[0] = ColumnBoxed
	wantErr := errors.New("diagnostic refused")
	for _, allowed := range []int{0, 1} {
		w := &lpValidationWriter{allowed: allowed, err: wantErr}
		if err := m.PrintColumnInfo(0, w); err != wantErr {
			t.Fatal(allowed, err)
		}
	}
	var out bytes.Buffer
	*m.LowBounds = nil
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds || !strings.HasSuffix(out.String(), "type = boxed\n") {
		t.Fatal(out.String(), err)
	}
	*m.LowBounds = []float64{0}
	*m.UpperBounds = nil
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnUpperBound
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnLowBound
	*m.LowBounds = nil
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnFree
	*m.Heading = nil
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
		t.Fatal(err)
	}
	*m.Heading = []int32{-1}
	*m.X = nil
	stdout := lpCaptureFeasibleStdout(t, func() {
		if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
			t.Fatal(err)
		}
	})
	if stdout != "basis heading = -1\n" {
		t.Fatal(stdout)
	}
	*m.X = []float64{0}
	(*m.ColumnTypes)[0] = ColumnType(99)
	stdout = lpCaptureFeasibleStdout(t, func() {
		err := m.PrintColumnInfo(0, &out)
		if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
			t.Fatal(err)
		}
	})
	if sourceDebugEnabled && stdout != "" {
		t.Fatal(stdout)
	}
	if err := m.PrintColumnInfo(0, nil); err != ErrBounds {
		t.Fatal(err)
	}
	m.Names = nil
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
		t.Fatal(err)
	}
	m.Names = lpFeasibleNames{}
	*m.ColumnTypes = nil
	if err := m.PrintColumnInfo(0, &out); err != ErrBounds {
		t.Fatal(err)
	}
	*m.NBasis = []uint32{9}
	if ok, err := m.NonBasicColumnsAreSetCorrectly(); ok || err != ErrBounds {
		t.Fatal(ok, err)
	}
	// A closed real stdout file exercises the native output error boundary.
	*m.ColumnTypes = []ColumnType{ColumnFree}
	r, w, err := os.Pipe()
	lpStaticMust(t, err)
	r.Close()
	w.Close()
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	if err := m.PrintColumnInfo(0, &out); err == nil {
		t.Fatal("closed stdout accepted")
	}
}
