package lp

import (
	"bytes"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatDualOuterIndependentVertices(t *testing.T) {
	rng := rand.New(rand.NewSource(366352))
	for world := 0; world < 120; world++ {
		m := lpFloatDual(t)
		m.Settings.SimplexStrategy = StrategyLU
		m.Settings.MaxTotalIterations = 1000
		m.Settings.UseScaling = world%2 == 0
		t.Cleanup(func() { m.Close() })
		shift := []float64{-2, -3}
		cost := []float64{float64(1 + rng.Intn(8)), float64(1 + rng.Intn(8))}
		if world%7 == 0 {
			cost[0] = -cost[0]
		}
		boxed := []bool{world%3 == 0, world%5 == 0}
		for j := 0; j < 2; j++ {
			c := m.GetOrCreateColumnInfo(uint32(10 + j))
			c.SetLowBound(shift[j])
			if boxed[j] {
				c.SetUpperBound(shift[j] + 6)
			}
			c.Cost = cost[j]
		}
		fixed := m.GetOrCreateColumnInfo(99)
		fixed.SetLowBound(5)
		fixed.SetUpperBound(5)
		fixed.SetFixedValue(5)
		fixed.Cost = 2
		inequalities := [][]float64{}
		rhs := []float64{}
		for i := 0; i < 3; i++ {
			a, b := float64(1+rng.Intn(7)), float64(1+rng.Intn(7))
			bound := float64(3 + rng.Intn(25))
			inequalities = append(inequalities, []float64{a, b})
			rhs = append(rhs, bound)
			row := uint32(100 + i)
			m.SetRowColumnCoefficient(row, 10, a)
			m.SetRowColumnCoefficient(row, 11, b)
			m.SetRowColumnCoefficient(row, 99, float64(i+1))
			lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, bound+float64(a*shift[0])+float64(b*shift[1])+float64(5*float64(i+1)), row))
		}
		for j := 0; j < 2; j++ {
			if boxed[j] {
				row := []float64{0, 0}
				row[j] = 1
				inequalities = append(inequalities, row)
				rhs = append(rhs, 6)
			}
		}
		a := make([][]float64, len(rhs))
		c := make([]float64, 2+len(rhs))
		copy(c, cost)
		for i, row := range inequalities {
			a[i] = make([]float64, len(c))
			copy(a[i], row)
			a[i][2+i] = 1
		}
		want := lpVertexMaximum(a, rhs, c) + float64(cost[0]*shift[0]) + float64(cost[1]*shift[1]) + 10
		lpStaticMust(t, m.FindMaximalSolution())
		got, err := m.GetCurrentCost()
		lpStaticMust(t, err)
		if m.Status != StatusOptimal || math.Abs(got-want) > 1e-7 || m.TotalIterations != m.FirstStageIterations+m.SecondStageIterations {
			t.Fatal(world, m.Status, got, want, m.X, m.B, m.CoreSolver.D)
		}
		values := make([]float64, 2)
		for j := 0; j < 2; j++ {
			values[j], err = m.GetColumnValue(uint32(10 + j))
			lpStaticMust(t, err)
			if values[j] < shift[j]-1e-7 || boxed[j] && values[j] > shift[j]+6+1e-7 {
				t.Fatal(world, j, values)
			}
		}
		for i, row := range inequalities {
			lhs := float64(row[0]*(values[0]-shift[0])) + float64(row[1]*(values[1]-shift[1]))
			if lhs > rhs[i]+1e-7 {
				t.Fatal(world, i, lhs, rhs[i])
			}
		}
		if value, err := m.GetColumnValue(99); err != nil || value != 5 {
			t.Fatal(value, err)
		}
		core := m.CoreSolver
		lpStaticMust(t, m.Close())
		lpStaticMust(t, m.Close())
		if m.CoreSolver != nil || core.Factorization != nil {
			t.Fatal("close")
		}
	}
}

func TestLPFloatDualOuterStageStatusAndTolerance(t *testing.T) {
	m := lpFloatDual(t)
	in := lpFloatCoreInput(t, 0, 1)
	core, err := NewFloatCoreSolverBase(in)
	lpStaticMust(t, err)
	m.CoreSolver = &FloatDualCoreSolver{FloatCoreSolverBase: core}
	*m.CoreSolver.Costs = []float64{1}
	for _, cost := range []float64{0, math.Nextafter(m.Settings.ToleranceForArtificials, 0), m.Settings.ToleranceForArtificials, -m.Settings.ToleranceForArtificials, math.NaN(), math.Inf(1)} {
		*m.CoreSolver.X = []float64{cost}
		m.CoreSolver.Status = StatusOptimal
		lpStaticMust(t, m.DecideOnStatusAfterStage1())
		want := StatusUnbounded
		if math.Abs(cost) < m.Settings.ToleranceForArtificials {
			want = StatusFeasible
		}
		if m.Status != want {
			t.Fatal(cost, m.Status, want)
		}
	}
	for _, status := range []Status{StatusDualUnbounded, StatusIterationsExhausted, StatusTimeExhausted, StatusFloatingPointError, StatusUnknown} {
		m.Status = StatusEmpty
		m.CoreSolver.Status = status
		err := m.DecideOnStatusAfterStage1()
		if sourceDebugEnabled && (status == StatusDualUnbounded || status == StatusUnknown) {
			if err != ErrAssertion || m.Status != StatusEmpty {
				t.Fatal(status, err, m.Status)
			}
			continue
		}
		lpStaticMust(t, err)
		want := status
		if status == StatusDualUnbounded {
			want = StatusIterationsExhausted
		}
		if status == StatusUnknown {
			want = StatusEmpty
		}
		if m.Status != want {
			t.Fatal(status, m.Status, want)
		}
	}
}

func TestLPFloatDualOuterEmptyFailuresAndPublication(t *testing.T) {
	m := lpFloatDual(t)
	lpStaticMust(t, m.FindMaximalSolution())
	if m.Status != StatusEmpty || m.CoreSolver != nil {
		t.Fatal(m.Status)
	}
	if err := m.Stage1(); err != ErrBounds {
		t.Fatal(err)
	}
	if err := m.SolveForStage2(); err != ErrBounds {
		t.Fatal(err)
	}
	if err := m.DecideOnStatusAfterStage1(); err != ErrBounds {
		t.Fatal(err)
	}
	if err := m.PrintStatisticsOnA(&bytes.Buffer{}); err != ErrBounds {
		t.Fatal(err)
	}
	n := lpDualSingle(t)
	n.Settings.SimplexStrategy = StrategyLU
	lpStaticMust(t, n.FillFirstStageSolverFields())
	n.CopyBAsideAndSetToZeros()
	n.X = []float64{8, 9, 10}
	wantErr := errors.New("dual matrix stats")
	n.Settings.MessageOut = lpPresolveFailWriter{wantErr}
	if err := n.Stage1(); err != wantErr || n.CoreSolver != nil || !reflect.DeepEqual(n.X, []float64{8, 9}) {
		t.Fatal(err, n.CoreSolver, n.X)
	}
	var out bytes.Buffer
	n.Settings.MessageOut = &out
	// Default tableau constructor has no LU: partial owner stays unpublished.
	n.Settings.SimplexStrategy = StrategyTableauRows
	if err := n.Stage1(); err != ErrBounds || n.CoreSolver != nil || out.String() != "extended A[1,2]\n" {
		t.Fatal(err, n.CoreSolver, out.String())
	}
	n.Settings.SimplexStrategy = StrategyLU
	n.Settings.MessageOut = nil
	lpStaticMust(t, n.Stage1())
	defer n.Close()
	if n.Status != StatusFeasible || n.FirstStageIterations != 0 {
		t.Fatal(n.Status, n.FirstStageIterations)
	}
	if sourceDebugEnabled {
		old := n.CoreSolver
		if err := n.Stage1(); err != ErrAssertion || n.CoreSolver != old {
			t.Fatal(err)
		}
	}
	// Source stage2 default omits IterationsExhausted status dispatch.
	n.Settings.MaxTotalIterations = 0
	n.Status = StatusFeasible
	n.FirstStageIterations = math.MaxUint32
	n.SecondStageIterations = 77
	n.TotalIterations = 88
	n.BCopy = []float64{-2}
	n.CoreColumnTypes[0] = ColumnLowBound
	lpStaticMust(t, n.A.Set(0, 0, 1))
	err := n.Stage2()
	if sourceDebugEnabled {
		if err != ErrAssertion || n.SecondStageIterations != 77 || n.TotalIterations != 88 {
			t.Fatal(err, n.SecondStageIterations, n.TotalIterations)
		}
	} else if err != nil || n.Status != StatusFeasible || n.SecondStageIterations != 1 || n.TotalIterations != 0 {
		t.Fatal(err, n.Status, n.SecondStageIterations, n.TotalIterations)
	}
}

func TestLPFloatDualOuterRelationsFreeAndStatuses(t *testing.T) {
	type fixture struct {
		name      string
		rows      [][]float64
		rhs       []float64
		relations []LPRelation
		cost      []float64
		free      bool
		status    Status
		optimum   float64
	}
	for _, f := range []fixture{
		{"equality", [][]float64{{1, 1}, {1, 2}}, []float64{3, 5}, []LPRelation{RelationEqual, RelationLessOrEqual}, []float64{1, 2}, false, StatusOptimal, 5},
		{"free", [][]float64{{1, 1}, {-1, 1}}, []float64{3, 2}, []LPRelation{RelationLessOrEqual, RelationLessOrEqual}, []float64{1, 2}, true, StatusOptimal, 5.5},
		{"infeasible", [][]float64{{1, 1}, {1, 1}}, []float64{1, 3}, []LPRelation{RelationLessOrEqual, RelationGreaterOrEqual}, []float64{1, 1}, false, StatusInfeasible, 0},
		{"unbounded", [][]float64{{-1, -1}}, []float64{-1}, []LPRelation{RelationLessOrEqual}, []float64{1, 1}, false, StatusUnbounded, 0},
	} {
		t.Run(f.name, func(t *testing.T) {
			m := lpFloatDual(t)
			m.Settings.SimplexStrategy = StrategyLU
			m.Settings.MaxTotalIterations = 1000
			t.Cleanup(func() { m.Close() })
			for j, cost := range f.cost {
				c := m.GetOrCreateColumnInfo(uint32(j))
				if j != 0 || !f.free {
					c.SetLowBound(0)
				}
				c.Cost = cost
			}
			for i, row := range f.rows {
				for j, v := range row {
					m.SetRowColumnCoefficient(uint32(i), uint32(j), v)
				}
				lpStaticMust(t, m.AddConstraint(f.relations[i], f.rhs[i], uint32(i)))
			}
			lpStaticMust(t, m.FindMaximalSolution())
			if m.Status != f.status {
				t.Fatal(m.Status, f.status, m.X, m.Costs)
			}
			if f.status == StatusOptimal {
				value, err := m.GetCurrentCost()
				lpStaticMust(t, err)
				if math.Abs(value-f.optimum) > 1e-7 {
					t.Fatal(value, f.optimum)
				}
			}
		})
	}
}

func TestLPFloatDualOuterActualStopsAndMetadata(t *testing.T) {
	for _, phase := range []int{1, 2} {
		m := lpFloatDual(t)
		m.Settings.SimplexStrategy = StrategyLU
		m.Settings.TimeLimit = -1
		for j := uint32(0); j < 2; j++ {
			c := m.GetOrCreateColumnInfo(j)
			c.SetLowBound(0)
			c.Cost = 1
			if phase == 2 {
				c.SetUpperBound(5)
			}
			m.SetRowColumnCoefficient(0, j, 1)
		}
		lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, 3, 0))
		lpStaticMust(t, m.FindMaximalSolution())
		defer m.Close()
		if m.Status != StatusTimeExhausted {
			t.Fatal(phase, m.Status)
		}
		if phase == 1 {
			if m.FirstStageIterations != 1 || m.SecondStageIterations != 0 {
				t.Fatal(phase, m.FirstStageIterations, m.SecondStageIterations)
			}
		} else if m.FirstStageIterations != 0 || m.SecondStageIterations != 1 || m.TotalIterations != 1 {
			t.Fatal(phase, m.FirstStageIterations, m.SecondStageIterations, m.TotalIterations)
		}
	}
	m := lpFloatDual(t)
	m.Settings.SimplexStrategy = StrategyLU
	for j := uint32(0); j < 2; j++ {
		c := m.GetOrCreateColumnInfo(j)
		c.SetLowBound(0)
		c.Cost = 1
		m.SetRowColumnCoefficient(0, j, 1)
	}
	lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, -1, 0))
	lpStaticMust(t, m.FindMaximalSolution())
	if m.Status != StatusInfeasible || m.CoreSolver != nil || m.Columns[0].Cost != -1 {
		t.Fatal(m.Status, m.CoreSolver, m.Columns[0].Cost)
	}
	n := lpFloatDual(t)
	if value, err := n.GetColumnValue(99); err != nil || value != 0 {
		t.Fatal(value, err)
	}
	if cost, err := n.GetCurrentCost(); err != nil || cost != 0 || !math.Signbit(cost) {
		t.Fatal(cost, err)
	}
	n.Columns[1] = nil
	if _, err := n.GetCurrentCost(); err != ErrBounds {
		t.Fatal(err)
	}
	n.Columns[1] = NewFloatColumnInfo(0)
	if _, err := n.GetColumnValue(1); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := n.GetCurrentCost(); err != ErrBounds {
		t.Fatal(err)
	}
	n.A = NewFloatStaticMatrix(0, 0)
	if err := n.PrintStatisticsOnA(nil); err != ErrBounds {
		t.Fatal(err)
	}
}

func TestLPFloatDualOuterReachedChildFailures(t *testing.T) {
	prepared := func() *FloatDualSimplex {
		m := lpDualSingle(t)
		m.Settings.SimplexStrategy = StrategyLU
		lpStaticMust(t, m.FillFirstStageSolverFields())
		m.CopyBAsideAndSetToZeros()
		t.Cleanup(func() { m.Close() })
		return m
	}
	m := prepared()
	m.LowBounds = nil
	if err := m.Stage1(); err != ErrBounds || (m.CoreSolver == nil) != sourceDebugEnabled {
		t.Fatal(err, m.CoreSolver)
	}
	n := prepared()
	n.Costs = append(n.Costs, 7)
	err := n.Stage1()
	want := error(ErrBounds)
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if err != want || n.CoreSolver == nil {
		t.Fatal(err, want, n.CoreSolver)
	}
	p := prepared()
	p.Costs[0] = -1
	p.Settings.PrintStatistics = true
	p.Settings.ReportFrequency = 1
	wantErr := errors.New("stage one solve stats")
	p.Settings.MessageOut = &lpValidationWriter{allowed: 1, err: wantErr}
	p.FirstStageIterations = 77
	if err := p.Stage1(); err != wantErr || p.CoreSolver == nil || p.CoreSolver.TotalIterations != 1 || p.FirstStageIterations != 77 {
		t.Fatal(err, p.CoreSolver, p.FirstStageIterations)
	}
	for _, which := range []string{"heading", "closed", "bounds", "solve"} {
		q := prepared()
		lpStaticMust(t, q.Stage1())
		q.SecondStageIterations = 77
		want := error(ErrBounds)
		switch which {
		case "heading":
			q.Heading = nil
		case "closed":
			q.CoreSolver.Factorization.Close()
			want = ErrLUClosed
		case "bounds":
			q.LowBounds = nil
		case "solve":
			q.Settings.PrintStatistics = true
			q.Settings.ReportFrequency = 1
			q.Settings.MessageOut = lpPresolveFailWriter{wantErr}
			want = wantErr
		}
		if err := q.SolveForStage2(); err != want || q.SecondStageIterations != 77 {
			t.Fatal(which, err, want, q.SecondStageIterations)
		}
	}
	r := prepared()
	lpStaticMust(t, r.Stage1())
	r.CoreColumnTypes = nil
	if err := r.Stage2(); err != ErrBounds {
		t.Fatal(err)
	}
	s := prepared()
	lpStaticMust(t, s.Stage1())
	s.BCopy = nil
	if err := s.Stage2(); err != ErrBounds {
		t.Fatal(err)
	}
	v := prepared()
	lpStaticMust(t, v.A.Set(0, 0, -1))
	lpStaticMust(t, v.Stage1())
	v.BCopy = []float64{-2}
	v.Settings.RefactorTolerance = -1
	lpStaticMust(t, v.Stage2())
	if v.Status != StatusFloatingPointError || v.SecondStageIterations != 1 {
		t.Fatal(v.Status, v.SecondStageIterations)
	}
	// Status cost reads retain the source profile's vector extent checks.
	w := prepared()
	lpStaticMust(t, w.Stage1())
	w.X = nil
	w.CoreSolver.Status = StatusOptimal
	err = w.DecideOnStatusAfterStage1()
	if err != want {
		t.Fatal(err, want)
	}
}
