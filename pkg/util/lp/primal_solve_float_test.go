package lp

import (
	"bytes"
	"errors"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// Exhaust all basic vertices with independent dense Gauss-Jordan elimination.
// The fixtures are bounded standard-form maximization problems with x >= 0.
func lpVertexMaximum(a [][]float64, b, c []float64) float64 {
	rows, cols := len(b), len(c)
	best := math.Inf(-1)
	choice := make([]int, rows)
	var visit func(int, int)
	visit = func(depth, start int) {
		if depth < rows {
			for j := start; j <= cols-(rows-depth); j++ {
				choice[depth] = j
				visit(depth+1, j+1)
			}
			return
		}
		augmented := make([][]float64, rows)
		for i := 0; i < rows; i++ {
			augmented[i] = make([]float64, rows+1)
			for j := 0; j < rows; j++ {
				augmented[i][j] = a[i][choice[j]]
			}
			augmented[i][rows] = b[i]
		}
		for k := 0; k < rows; k++ {
			pivot := k
			for i := k + 1; i < rows; i++ {
				if math.Abs(augmented[i][k]) > math.Abs(augmented[pivot][k]) {
					pivot = i
				}
			}
			if math.Abs(augmented[pivot][k]) < 1e-12 {
				return
			}
			augmented[k], augmented[pivot] = augmented[pivot], augmented[k]
			v := augmented[k][k]
			for j := k; j <= rows; j++ {
				augmented[k][j] /= v
			}
			for i := 0; i < rows; i++ {
				if i != k {
					v := augmented[i][k]
					for j := k; j <= rows; j++ {
						augmented[i][j] -= float64(v * augmented[k][j])
					}
				}
			}
		}
		cost := 0.0
		for i := 0; i < rows; i++ {
			x := augmented[i][rows]
			if x < -1e-9 {
				return
			}
			cost += float64(c[choice[i]] * x)
		}
		if cost > best {
			best = cost
		}
	}
	visit(0, 0)
	return best
}

func lpSolveStandardFixture(t *testing.T, a [][]float64, b, c, x []float64, basis []uint32) *FloatPrimalCoreSolver {
	t.Helper()
	return lpSolveStandardConstructorFixture(t, a, b, c, x, basis, false, false)
}

func lpSolveStandardConstructorFixture(t *testing.T, a [][]float64, b, c, x []float64, basis []uint32, zeroLow, defaultStrategy bool) *FloatPrimalCoreSolver {
	t.Helper()
	in := lpFloatCoreInput(t, uint32(len(b)), uint32(len(c)))
	in.Settings.SimplexStrategy = StrategyLU
	if defaultStrategy {
		in.Settings.SimplexStrategy = StrategyTableauRows
	}
	in.Settings.MaxTotalIterations = 1000
	*in.B = append([]float64(nil), b...)
	*in.Costs = append([]float64(nil), c...)
	*in.X = append([]float64(nil), x...)
	*in.Basis = append([]uint32(nil), basis...)
	for i, row := range a {
		for j, v := range row {
			lpStaticMust(t, in.A.Set(uint32(i), uint32(j), v))
		}
	}
	for j := range c {
		(*in.ColumnTypes)[j] = ColumnLowBound
	}
	var m *FloatPrimalCoreSolver
	var err error
	if zeroLow {
		// Match the source fixture's empty upper vector and omitted low vector.
		*in.UpperBounds = nil
		in.LowBounds = nil
		m, err = NewFloatPrimalCoreSolverWithZeroLowBounds(in)
	} else {
		m, err = NewFloatPrimalCoreSolver(in)
	}
	lpStaticMust(t, err)
	t.Cleanup(func() { m.Close() })
	return m
}

func TestLPFloatPrimalSolveOriginalNumericalFixtures(t *testing.T) {
	fixtures := []struct {
		a       [][]float64
		b, c, x []float64
		basis   []uint32
	}{
		{[][]float64{{3, 2, 1, 2, 1, 0, 0}, {1, 1, 1, 1, 0, 1, 0}, {4, 3, 3, 4, 0, 0, 1}}, []float64{1329, 762, 2511}, []float64{19, 13, 12, 17, 0, 0, 0}, []float64{225, 117, 420, 0, 0, 0, 0}, []uint32{0, 1, 2}},
		{[][]float64{{1, 3, 1, 1, 0, 0, 0}, {-1, 0, 3, 0, 1, 0, 0}, {2, -1, 2, 0, 0, 1, 0}, {2, 3, -1, 0, 0, 0, 1}}, []float64{3, 2, 4, 2}, []float64{5, 5, 3, 0, 0, 0, 0}, []float64{0, 0, 0, 3, 2, 4, 2}, []uint32{3, 4, 5, 6}},
	}
	for i, f := range fixtures {
		for _, zeroLow := range []bool{false, true} {
			m := lpSolveStandardConstructorFixture(t, f.a, f.b, f.c, f.x, f.basis, zeroLow, false)
			iterations, err := m.Solve()
			want := lpVertexMaximum(f.a, f.b, f.c)
			cost := 0.0
			for j, v := range *m.X {
				cost += float64(f.c[j] * v)
				if v < -1e-7 {
					t.Fatal(i, "negative solution", *m.X)
				}
			}
			if err != nil || m.Status != StatusOptimal || iterations == 0 || math.Abs(cost-want) > 1e-7 {
				t.Fatal(i, iterations, m.Status, cost, want, err, *m.X)
			}
			if bad, err := m.AMultXIsOff(); bad || err != nil {
				t.Fatal(i, bad, err)
			}
		}
		original := lpSolveStandardConstructorFixture(t, f.a, f.b, f.c, f.x, f.basis, true, true)
		if original.Status != StatusFeasible || original.Factorization != nil {
			t.Fatal("original default constructor", original.Status)
		}
		wantErr := ErrBounds
		if sourceDebugEnabled {
			// Source init_reduced_costs asserts !use_tableau before the
			// release path reaches the missing factorization.
			wantErr = ErrAssertion
		}
		if _, err := original.Solve(); err != wantErr {
			t.Fatal("original floating default profile", err)
		}
	}
}

func TestLPFloatPrimalSolveIndependentVertices(t *testing.T) {
	rng := rand.New(rand.NewSource(864963))
	for world := 0; world < 100; world++ {
		a := make([][]float64, 3)
		b := make([]float64, 3)
		c := []float64{float64(1 + rng.Intn(12)), float64(1 + rng.Intn(12)), 0, 0, 0}
		x := make([]float64, 5)
		for i := 0; i < 3; i++ {
			a[i] = make([]float64, 5)
			a[i][0] = float64(1 + rng.Intn(8))
			a[i][1] = float64(1 + rng.Intn(8))
			a[i][i+2] = 1
			b[i] = float64(1 + rng.Intn(30))
			x[i+2] = b[i]
		}
		m := lpSolveStandardFixture(t, a, b, c, x, []uint32{2, 3, 4})
		if world%2 == 0 {
			m.Settings.DensityThreshold = 0
		} else {
			m.Settings.DensityThreshold = math.Inf(1)
		}
		iterations, err := m.Solve()
		want := lpVertexMaximum(a, b, c)
		cost := 0.0
		for j, v := range *m.X {
			cost += float64(c[j] * v)
		}
		if err != nil || m.Status != StatusOptimal || math.Abs(cost-want) > 1e-7 || iterations > 100 {
			t.Fatal(world, iterations, m.Status, cost, want, err)
		}
	}
}

func TestLPFloatPrimalSolveStopsAndStatuses(t *testing.T) {
	for _, only := range []bool{false, true} {
		m := lpFloatPrimalCore(t, 1, 1)
		(*m.X)[0] = -1
		(*m.B)[0] = -1
		(*m.ColumnTypes)[0] = ColumnLowBound
		m.LookForFeasibleSolutionOnly = only
		it, err := m.Solve()
		if err != nil || it != 1 || m.Status != StatusInfeasible {
			t.Fatal(only, it, m.Status, err)
		}
	}
	m := lpFloatPrimalCore(t, 0, 1)
	(*m.Costs)[0] = 1
	it, err := m.Solve()
	if err != nil || it != 2 || m.Status != StatusUnbounded {
		t.Fatal(it, m.Status, err)
	}
	n := lpFloatPrimalCore(t, 1, 1)
	n.LookForFeasibleSolutionOnly = true
	(*n.B)[0] = 1
	it, err = n.Solve()
	if err != nil || it != 0 || n.Status != StatusFeasible {
		t.Fatal(it, n.Status, err)
	}
	n.LookForFeasibleSolutionOnly = false
	it, err = n.Solve()
	if err != nil || it != 0 || n.Status != StatusFloatingPointError {
		t.Fatal(it, n.Status, err)
	}
	u := lpFloatPrimalCore(t, 0, 1)
	(*u.Costs)[0] = 1
	u.Settings.MaxTotalIterations = 0
	it, err = u.Solve()
	if err != nil || it != 1 || u.Status != StatusTentativeUnbounded {
		t.Fatal(it, u.Status, err)
	}
	v := lpFloatPrimalCore(t, 0, 1)
	limit := &lpTestResourceLimit{cancel: true}
	v.Settings.SetResourceLimit(limit)
	it, err = v.Solve()
	if err != nil || it != 1 || v.Status != StatusTimeExhausted || limit.calls != 1 {
		t.Fatal(it, v.Status, err, limit.calls)
	}
	wantErr := errors.New("cancel clock failed")
	limit = &lpTestResourceLimit{err: wantErr}
	v = lpFloatPrimalCore(t, 0, 1)
	v.Settings.SetResourceLimit(limit)
	v.Status = StatusUnknown
	it, err = v.Solve()
	if err != wantErr || it != 1 || v.Status != StatusUnknown {
		t.Fatal(it, v.Status, err)
	}
}

func TestLPFloatPrimalIterationBudgetAndStatistics(t *testing.T) {
	for _, n := range []int{0, 1, 300, 301, 1000} {
		for _, percent := range []uint32{0, 5, 100, math.MaxUint32} {
			m := lpFloatPrimalCore(t, 0, 0)
			nb := make([]uint32, n)
			m.NBasis = &nb
			m.Settings.PercentOfEnteringToCheck = percent
			m.Settings.RandomSeed(9)
			count := uint32(n)
			if count > 300 {
				count = count * percent / 100
			}
			want := uint32(0)
			if count > 0 {
				state := uint32(9)*214013 + 2531011
				want = ((state >> 16) & 32767) % count
				if want == 0 {
					want = 1
				}
			}
			if got := m.NumberOfNonBasicColumnsToTryForEnter(); got != want {
				t.Fatal(n, percent, got, want)
			}
			m.Status = StatusTentativeUnbounded
			if got := m.NumberOfNonBasicColumnsToTryForEnter(); got != uint32(n) {
				t.Fatal(got)
			}
		}
	}
	m := lpFloatPrimalCore(t, 1, 2)
	(*m.X)[0] = 2
	(*m.Costs)[0] = 3
	m.Settings.ReportFrequency = 2
	m.Settings.PrintStatistics = true
	var out bytes.Buffer
	prefix := "feas"
	for i := 0; i < 3; i++ {
		over, err := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, &out)
		if err != nil || over {
			t.Fatal(over, err)
		}
	}
	if out.String() != "feas iterations = 0, cost = 6, nonzeros = 1\nfeas iterations = 2, cost = 6, nonzeros = 1\n" || m.Settings.Stats.TotalIterations != 3 {
		t.Fatal(out.String(), m.Settings.Stats)
	}
	m.Close()
	out.Reset()
	lpStaticMust(t, m.PrintStatistics(nil, math.Inf(1), &out))
	if !strings.Contains(out.String(), "cost = inf") {
		t.Fatal(out.String())
	}
	wantErr := errors.New("statistics refused")
	limit := &lpTestResourceLimit{}
	m.Settings.SetResourceLimit(limit)
	m.TotalIterations = 0
	over, err := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, lpPresolveFailWriter{wantErr})
	if over || err != wantErr || m.TotalIterations != 1 || limit.calls != 0 {
		t.Fatal(over, err, m.TotalIterations, limit.calls)
	}
}

func TestLPFloatPrimalStatisticsGuardsAndIEEE(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	wantErr := errors.New("writer refused")
	if err := m.PrintStatistics(nil, 1, nil); err != ErrBounds {
		t.Fatal(err)
	}
	if err := m.PrintStatistics(nil, 1, lpPresolveFailWriter{wantErr}); err != wantErr {
		t.Fatal(err)
	}
	m.Factorization.Close()
	var out bytes.Buffer
	if err := m.PrintStatistics(nil, 1, &out); err != ErrLUClosed || out.String() != "iterations = 4294967295, cost = 1, nonzeros = " {
		t.Fatal(err, out.String())
	}
	m.Factorization = nil
	for _, value := range []float64{math.Inf(-1), math.NaN(), math.Copysign(math.NaN(), -1), math.Copysign(0, -1), 1.23456789} {
		out.Reset()
		lpStaticMust(t, m.PrintStatistics(nil, value, &out))
		want := "cost = -inf"
		if math.IsNaN(value) {
			want = "cost = nan"
			if math.Signbit(value) {
				want = "cost = -nan"
			}
		} else if value == 0 {
			want = "cost = -0"
		} else if !math.IsInf(value, 0) {
			want = "cost = 1.23457"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatal(value, out.String(), want)
		}
	}
	m.TotalIterations = math.MaxUint32
	m.Settings.Stats.TotalIterations = math.MaxUint32
	if old := m.IncTotalIterations(); old != math.MaxUint32 || m.TotalIterations != 0 || m.Settings.Stats.TotalIterations != 0 {
		t.Fatal(old, m.TotalIterations, m.Settings.Stats)
	}
	*m.Costs = []float64{1, 2}
	*m.X = []float64{3}
	_, err := m.GetCost()
	want := ErrBounds
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if err != want {
		t.Fatal(err)
	}
	m.Settings.ReportFrequency = 1
	m.Settings.PrintStatistics = true
	if _, err := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(nil, &out); err != want || m.TotalIterations != 1 {
		t.Fatal(err, m.TotalIterations)
	}
	m.Settings.PrintStatistics = false
	if over, err := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(nil, nil); over || err != nil {
		t.Fatal(over, err)
	}
}

func TestLPFloatPrimalSolveAndIterationReachedGuards(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	*m.X = nil
	if _, err := m.Solve(); err != ErrBounds {
		t.Fatal(err)
	}
	n := lpFloatPrimalCore(t, 1, 2)
	*n.B = nil
	if _, err := n.Solve(); err != ErrBounds {
		t.Fatal(err)
	}
	p := lpFloatPrimalCore(t, 0, 1)
	p.BasisSortCounter = 1
	p.NonBasisList = []uint32{99}
	if err := p.OneIteration(); err != ErrBounds {
		t.Fatal(err)
	}
	q := lpFloatPrimalCore(t, 0, 1)
	q.UsingInfeasCosts = true
	err := q.DecideOnStatusWhenCannotFindEntering()
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && (err != nil || q.Status != StatusOptimal) {
		t.Fatal(err, q.Status)
	}
	r := lpFloatPrimalCore(t, 1, 2)
	(*r.Costs)[1] = 1
	err = r.CostsOnNBasisAreZeros()
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	*r.Costs = nil
	err = r.CostsOnNBasisAreZeros()
	if sourceDebugEnabled && err != ErrBounds || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	(*r.Heading)[0] = -1
	err = r.CostsOnNBasisAreZeros()
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
}
