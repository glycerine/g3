package lp

import (
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func lpDualStandardFixture(t *testing.T, a [][]float64, b, c []float64, basis []uint32) *FloatDualCoreSolver {
	t.Helper()
	in := lpFloatCoreInput(t, uint32(len(b)), uint32(len(c)))
	in.Settings.SimplexStrategy = StrategyLU
	in.Settings.MaxTotalIterations = 200
	*in.B = append([]float64(nil), b...)
	*in.Costs = append([]float64(nil), c...)
	*in.Basis = append([]uint32(nil), basis...)
	can := make([]bool, len(c))
	for j := range c {
		can[j] = true
		(*in.ColumnTypes)[j] = ColumnLowBound
	}
	for i, row := range a {
		for j, v := range row {
			lpStaticMust(t, in.A.Set(uint32(i), uint32(j), v))
		}
	}
	m, err := NewFloatDualCoreSolver(in, &can)
	lpStaticMust(t, err)
	t.Cleanup(func() { m.Close() })
	lpStaticMust(t, m.RecalculateD())
	lpStaticMust(t, m.StartWithInitialBasisAndMakeItDualFeasible())
	return m
}

func TestLPFloatDualSolveIndependentVertices(t *testing.T) {
	rng := rand.New(rand.NewSource(745122))
	for world := 0; world < 120; world++ {
		rows := 1 + rng.Intn(3)
		structurals := 2 + rng.Intn(3)
		cols := structurals + rows
		a := make([][]float64, rows)
		b := make([]float64, rows)
		c := make([]float64, cols)
		negativeCost := make([]float64, cols)
		basis := make([]uint32, rows)
		for j := 0; j < structurals; j++ {
			c[j] = float64(1+rng.Intn(8)) / 4
			negativeCost[j] = -c[j]
		}
		for i := 0; i < rows; i++ {
			a[i] = make([]float64, cols)
			b[i] = -float64(1+rng.Intn(20)) / 2
			for j := 0; j < structurals; j++ {
				a[i][j] = -float64(1+rng.Intn(8)) / 4
			}
			a[i][structurals+i] = 1
			basis[i] = uint32(structurals + i)
		}
		want := -lpVertexMaximum(a, b, negativeCost)
		m := lpDualStandardFixture(t, a, b, c, basis)
		m.Settings.RandomSeed(uint32(world + 1))
		iterations, err := m.Solve()
		lpStaticMust(t, err)
		got, err := m.GetCost()
		lpStaticMust(t, err)
		if m.Status != StatusOptimal || iterations == 0 || math.Abs(got-want) > 1e-7 {
			t.Fatal(world, m.Status, iterations, got, want, *m.X)
		}
		for j, x := range *m.X {
			if x < -1e-7 {
				t.Fatal(world, j, x)
			}
		}
		for i, row := range a {
			lhs := 0.0
			for j, v := range row {
				lhs += float64(v * (*m.X)[j])
			}
			if math.Abs(lhs-b[i]) > 1e-7 {
				t.Fatal(world, i, lhs, b[i])
			}
		}
		ok, err := m.ProblemIsDualFeasible()
		lpStaticMust(t, err)
		if !ok {
			t.Fatal(world, m.D)
		}
	}
}

func TestLPFloatDualPricingRecordBudget(t *testing.T) {
	rng := rand.New(rand.NewSource(7211))
	m := lpDualInitialFixture(t, 9, 9)
	for world := 0; world < 500; world++ {
		m.ForbiddenRows = map[uint32]bool{}
		scores := make([]int, 9)
		for i := 0; i < 9; i++ {
			(*m.ColumnTypes)[i] = ColumnLowBound
			m.Betas[i] = 1
			scores[i] = rng.Intn(7)
			(*m.X)[i] = -float64(scores[i])
			if rng.Intn(3) == 0 {
				m.ForbiddenRows[uint32(i)] = true
			}
		}
		offset := rng.Intn(9)
		budget := rng.Intn(12)
		chosen := -1
		best := 0
		records := 0
		for n := 0; n < 9; n++ {
			i := (offset + n) % 9
			if !m.ForbiddenRows[uint32(i)] && scores[i] > best {
				chosen = i
				best = scores[i]
				records++
			}
			if budget == 0 || records == budget {
				break
			}
		}
		m.TotalIterations = m.Settings.MaxTotalIterations + 1
		m.Status = StatusUnknown
		lpStaticMust(t, m.PricingLoop(uint32(budget), uint32(offset)))
		if int(m.R) != chosen {
			t.Fatal(world, budget, offset, m.R, chosen, scores)
		}
		if chosen >= 0 {
			if m.P != int32(chosen) || m.Delta != -float64(scores[chosen]) || len(m.ForbiddenRows) != 0 || m.Status != StatusIterationsExhausted {
				t.Fatal(world, m.P, m.Delta, m.ForbiddenRows, m.Status)
			}
		} else if m.Status != StatusOptimal {
			t.Fatal(world, m.Status)
		}
	}
	m.ForbiddenRows = map[uint32]bool{}
	(*m.X)[0] = -1
	for i := uint32(0); i < 9; i++ {
		m.ForbiddenRows[i] = true
	}
	m.Status = StatusUnstable
	lpStaticMust(t, m.PricingLoop(9, 0))
	if m.R != -1 || m.Status != StatusUnstable || len(m.ForbiddenRows) != 9 {
		t.Fatal(m.R, m.Status, m.ForbiddenRows)
	}
	// OneIteration resets unstable to feasible before pricing. Source can
	// declare optimal when every infeasible row remains forbidden.
	lpStaticMust(t, m.OneIteration())
	if m.Status != StatusOptimal {
		t.Fatal(m.Status)
	}
	if (*m.X)[0] != -1 {
		t.Fatal("source forbidden-row result", *m.X)
	}
}

func TestLPFloatDualPricingActualNumericalFailures(t *testing.T) {
	for _, floating := range []bool{false, true} {
		m := lpDualStandardFixture(t, [][]float64{{1, -1}}, []float64{-1}, []float64{0, 1}, []uint32{0})
		m.ForbiddenRows = map[uint32]bool{9: true}
		if floating {
			m.Settings.RefactorTolerance = -1
		} else {
			// Real stale LU plus dropping tiny reconstruction values produces
			// a false advance with restored original X, rather than a fake
			// numerical callback. The original X satisfies the changed basis.
			lpStaticMust(t, m.A.MultiplyColumn(0, 1.5))
			(*m.X)[0] = -5e-6
			(*m.B)[0] = 1.5 * (*m.X)[0]
			m.Settings.DropTolerance = 0.001
			m.Settings.RefactorTolerance = 1e-12
		}
		lpStaticMust(t, m.PricingLoop(1, 0))
		if floating {
			if m.Status != StatusFloatingPointError || m.ForbiddenRows[0] || !m.ForbiddenRows[9] {
				t.Fatal(m.Status, m.ForbiddenRows)
			}
		} else if m.Status != StatusUnstable || !m.ForbiddenRows[0] || !m.ForbiddenRows[9] || (*m.X)[0] != -5e-6 || (*m.X)[1] != 0 {
			t.Fatal(m.Status, m.ForbiddenRows, *m.X)
		}
	}
}

func TestLPFloatDualLeavingBudgetRNGAndDomains(t *testing.T) {
	for _, rows := range []uint32{0, 1, 300, 301, 1000} {
		for _, percent := range []uint32{0, 1, 5, 100, math.MaxUint32} {
			in := lpFloatCoreInput(t, rows, 0)
			in.Settings.PercentOfEnteringToCheck = percent
			in.Settings.RandomSeed(17)
			m := &FloatDualCoreSolver{FloatCoreSolverBase: &FloatCoreSolverBase{FloatCoreInputs: in}}
			state := uint32(17)*214013 + 2531011
			random := (state >> 16) & 32767
			span := float64(rows)
			if rows > 300 {
				span = (span / 100) * float64(percent)
			}
			got, err := m.GetNumberOfRowsToTryForLeaving()
			if span >= float64(math.MaxUint32)+1 {
				if err != ErrBounds || in.Settings.RandomNext() != random {
					t.Fatal(rows, percent, got, err)
				}
				continue
			}
			n := uint32(span)
			if n == 0 {
				if err != ErrBounds {
					t.Fatal(rows, percent, err)
				}
			} else if err != nil || got != random%n+1 {
				t.Fatal(rows, percent, got, err)
			}
			state = state*214013 + 2531011
			if in.Settings.RandomNext() != (state>>16)&32767 {
				t.Fatal(rows, percent, "random publication")
			}
		}
	}
}

func TestLPFloatDualSolveLimitsTimeAndUnbounded(t *testing.T) {
	for _, limit := range []uint32{0, 1, 2, 20} {
		m := lpDualStandardFixture(t, [][]float64{{1, -1}}, []float64{-3}, []float64{0, 2}, []uint32{0})
		m.TotalIterations = 99
		m.IterationsNoCostGrowth = 88
		m.Settings.MaxTotalIterations = limit
		n, err := m.Solve()
		lpStaticMust(t, err)
		if limit == 0 {
			if n != 1 || m.Status != StatusIterationsExhausted || (*m.X)[0] != -3 {
				t.Fatal(limit, n, m.Status, *m.X)
			}
		} else if n != 2 || m.Status != StatusOptimal || (*m.X)[1] != 3 {
			t.Fatal(limit, n, m.Status, *m.X)
		}
	}
	m := lpDualStandardFixture(t, [][]float64{{1, 0}}, []float64{-3}, []float64{0, 2}, []uint32{0})
	n, err := m.Solve()
	lpStaticMust(t, err)
	if n != 2 || m.Status != StatusDualUnbounded {
		t.Fatal(n, m.Status)
	}
	p := lpDualStandardFixture(t, [][]float64{{1, -1}}, []float64{-3}, []float64{0, 2}, []uint32{0})
	p.TotalIterations = 99
	p.IterationsNoCostGrowth = 77
	p.Settings.TimeLimit = -1
	n, err = p.Solve()
	lpStaticMust(t, err)
	if n != 1 || p.Status != StatusTimeExhausted || p.IterationsNoCostGrowth != 0 || (*p.X)[0] != -3 {
		t.Fatal(n, p.Status, p.IterationsNoCostGrowth, *p.X)
	}
	want := errors.New("stats output")
	q := lpDualStandardFixture(t, [][]float64{{1, -1}}, []float64{-3}, []float64{0, 2}, []uint32{0})
	q.Settings.PrintStatistics = true
	q.Settings.ReportFrequency = 1
	q.Settings.MessageOut = lpPresolveFailWriter{want}
	n, err = q.Solve()
	if n != 1 || err != want || (*q.X)[0] != -3 {
		t.Fatal(n, err, *q.X)
	}
}

func TestLPFloatDualSolveDebugPreconditionsAndGuards(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	m.TotalIterations = 99
	m.IterationsNoCostGrowth = 77
	m.D[1] = 1
	n, err := m.Solve()
	if sourceDebugEnabled {
		if err != ErrAssertion || n != 99 || m.IterationsNoCostGrowth != 77 {
			t.Fatal(n, err)
		}
	} else if err != nil || n != 1 || m.Status != StatusOptimal {
		t.Fatal(n, err, m.Status)
	}
	if err = m.PricingLoop(1, 1); err != ErrBounds || m.R != -1 {
		t.Fatal(err, m.R)
	}
	*m.Basis = nil
	if err = m.PricingLoop(1, 0); err != ErrBounds {
		t.Fatal(err)
	}
	z := lpDualInitialFixture(t, 0, 0)
	if err = z.PricingLoop(0, 0); err != ErrBounds {
		t.Fatal(err)
	}
	if err = z.OneIteration(); err != ErrBounds {
		t.Fatal(err)
	}
	_, err = z.Solve()
	if err != ErrBounds {
		t.Fatal(err)
	}
	// Row score failure must retain the old P/delta; R was reset first.
	p := lpDualInitialFixture(t, 1, 1)
	p.P = 8
	p.Delta = 9
	*p.ColumnTypes = nil
	if err = p.PricingLoop(1, 0); err != ErrBounds || p.P != 8 || p.Delta != 9 {
		t.Fatal(err, p.P, p.Delta)
	}
	// Actual dual guard runs after OneIteration's RNG and pricing effects.
	q := lpDualInitialFixture(t, 1, 2)
	(*q.ColumnTypes)[1] = ColumnLowBound
	q.D[1] = -1
	q.Status = StatusUnstable
	err = q.OneIteration()
	if sourceDebugEnabled {
		if err != ErrAssertion || q.Status != StatusOptimal {
			t.Fatal(err, q.Status)
		}
	} else {
		lpStaticMust(t, err)
	}
	if !reflect.DeepEqual(*q.X, []float64{0, 0}) {
		t.Fatal(*q.X)
	}
}

func TestLPFloatDualSolvePreconditionPublication(t *testing.T) {
	for _, which := range []string{"dual-false", "dual-metadata", "heading-false", "heading-metadata", "closed"} {
		t.Run(which, func(t *testing.T) {
			m := lpDualInitialFixture(t, 1, 2)
			m.TotalIterations = 99
			m.IterationsNoCostGrowth = 77
			want := error(ErrAssertion)
			switch which {
			case "dual-false":
				(*m.ColumnTypes)[1] = ColumnLowBound
				(*m.Costs)[1] = -1
				m.D[1] = -1
			case "dual-metadata":
				*m.ColumnTypes = (*m.ColumnTypes)[:1]
				want = ErrBounds
			case "heading-false":
				(*m.Heading)[0] = 1
			case "heading-metadata":
				*m.Heading = nil
			case "closed":
				m.Factorization.Close()
				want = ErrLUClosed
			}
			if sourceDebugEnabled {
				n, err := m.Solve()
				if err != want || n != 99 || m.IterationsNoCostGrowth != 77 {
					t.Fatal(n, err, want, m.IterationsNoCostGrowth)
				}
			}
		})
	}
	// Source failures before/after the selected-row publication are distinct.
	m := lpDualInitialFixture(t, 1, 1)
	(*m.ColumnTypes)[0] = ColumnLowBound
	(*m.X)[0] = -2
	m.Close()
	if err := m.PricingLoop(1, 0); err != ErrBounds || m.R != 0 || m.P != 0 || m.Delta != -2 {
		t.Fatal(err, m.R, m.P, m.Delta)
	}
	p := lpDualInitialFixture(t, 1, 1)
	*p.ColumnTypes = nil
	p.Status = StatusUnstable
	if err := p.OneIteration(); err != ErrBounds || p.Status != StatusFeasible {
		t.Fatal(err, p.Status)
	}
	q := lpDualInitialFixture(t, 1, 2)
	*q.ColumnTypes = (*q.ColumnTypes)[:1]
	err := q.OneIteration()
	if sourceDebugEnabled {
		if err != ErrBounds || q.Status != StatusOptimal {
			t.Fatal(err, q.Status)
		}
	} else {
		lpStaticMust(t, err)
	}
}

func TestLPFloatDualSolveActualNoImprovementStop(t *testing.T) {
	for _, limit := range []uint32{0, 1} {
		m := lpDualStandardFixture(t, [][]float64{{1, -1}}, []float64{-1}, []float64{0, 1}, []uint32{0})
		lpStaticMust(t, m.A.MultiplyColumn(0, 1.5))
		(*m.X)[0] = -5e-6
		(*m.B)[0] = 1.5 * (*m.X)[0]
		m.Settings.DropTolerance = 0.001
		m.Settings.RefactorTolerance = 1e-12
		m.Settings.MaxIterationsWithoutImprovements = limit
		n, err := m.Solve()
		lpStaticMust(t, err)
		if m.IterationsNoCostGrowth != 1 || !m.ForbiddenRows[0] || (*m.X)[0] != -5e-6 {
			t.Fatal(limit, n, m.Status, m.IterationsNoCostGrowth, m.ForbiddenRows, *m.X)
		}
		if limit == 0 {
			if n != 1 || m.Status != StatusUnstable {
				t.Fatal(limit, n, m.Status)
			}
		} else if n != 2 || m.Status != StatusOptimal {
			t.Fatal(limit, n, m.Status)
		}
	}
}
