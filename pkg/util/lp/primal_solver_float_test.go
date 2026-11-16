package lp

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestLPFloatPrimalOuterIndependentVertices(t *testing.T) {
	rng := rand.New(rand.NewSource(169271))
	for world := 0; world < 120; world++ {
		m := lpFloatPrimal(t)
		m.Settings.SimplexStrategy = StrategyLU
		m.Settings.MaxTotalIterations = 1000
		m.Settings.UseScaling = world%2 == 0
		t.Cleanup(func() { m.Close() })
		sign := []float64{1, 1}
		shift := []float64{-2, -3}
		cost := []float64{float64(1 + rng.Intn(9)), float64(1 + rng.Intn(9))}
		boxed := []bool{world%3 == 0, world%5 == 0}
		for j := 0; j < 2; j++ {
			c := m.GetOrCreateColumnInfo(uint32(10 + j))
			if boxed[j] {
				c.SetLowBound(shift[j])
				c.SetUpperBound(shift[j] + 6)
			} else {
				c.SetLowBound(shift[j])
			}
			c.Cost = cost[j] * sign[j]
		}
		fixed := m.GetOrCreateColumnInfo(99)
		fixed.SetLowBound(5)
		fixed.SetUpperBound(5)
		fixed.SetFixedValue(5)
		fixed.Cost = 2
		var inequalities [][]float64
		var rhs []float64
		originalA := make([][]float64, 3)
		originalB := make([]float64, 3)
		for i := 0; i < 3; i++ {
			a, b := float64(1+rng.Intn(7)), float64(1+rng.Intn(7))
			bound := float64(3 + rng.Intn(25))
			inequalities = append(inequalities, []float64{a, b})
			rhs = append(rhs, bound)
			originalA[i] = []float64{a * sign[0], b * sign[1], float64(i + 1)}
			originalB[i] = bound + float64(originalA[i][0]*shift[0]) + float64(originalA[i][1]*shift[1]) + float64(5*originalA[i][2])
			for j, external := range []uint32{10, 11, 99} {
				m.SetRowColumnCoefficient(uint32(100+i), external, originalA[i][j])
			}
			lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, originalB[i], uint32(100+i)))
		}
		for j := 0; j < 2; j++ {
			if boxed[j] {
				v := []float64{0, 0}
				v[j] = 1
				inequalities = append(inequalities, v)
				rhs = append(rhs, 6)
			}
		}
		// Independent standard-form model represents external boxes explicitly.
		a := make([][]float64, len(rhs))
		c := make([]float64, 2+len(rhs))
		c[0], c[1] = cost[0], cost[1]
		for i, v := range inequalities {
			a[i] = make([]float64, len(c))
			copy(a[i], v)
			a[i][2+i] = 1
		}
		want := lpVertexMaximum(a, rhs, c) + float64(cost[0]*sign[0]*shift[0]) + float64(cost[1]*sign[1]*shift[1]) + 10
		lpStaticMust(t, m.FindMaximalSolution())
		got, err := m.GetCurrentCost()
		if err != nil || m.Status != StatusOptimal || math.Abs(got-want) > 1e-7 || m.CoreSolver == nil || m.TotalIterations != m.CoreSolver.TotalIterations {
			t.Fatal(world, m.Status, got, want, err, m.TotalIterations, "raw", originalA, originalB, "core", m.X, m.Costs, m.CoreSolver.D, m.Basis, m.ColumnTypes, m.UpperBounds)
		}
		values := make([]float64, 3)
		for j, external := range []uint32{10, 11, 99} {
			values[j], err = m.GetColumnValue(external)
			lpStaticMust(t, err)
		}
		solution := make(map[string]float64)
		for j, external := range []uint32{10, 11, 99} {
			name := string(rune('a' + j))
			m.Columns[external].Name = name
			solution[name] = values[j]
		}
		if ok, err := m.SolutionIsFeasible(solution); err != nil || !ok {
			t.Fatal(world, ok, err, solution)
		}
		for i, v := range originalA {
			sum := 0.0
			for j, x := range values {
				sum += float64(v[j] * x)
			}
			if sum > originalB[i]+1e-7 {
				t.Fatal(world, i, sum, originalB[i], values)
			}
		}
		for j := 0; j < 2; j++ {
			y := (values[j] - shift[j]) * sign[j]
			if y < -1e-7 || boxed[j] && y > 6+1e-7 {
				t.Fatal(world, j, values)
			}
		}
		core := m.CoreSolver
		lu := core.Factorization
		lpStaticMust(t, m.Close())
		lpStaticMust(t, m.Close())
		if m.CoreSolver != nil || !lu.closed {
			t.Fatal("core lifetime")
		}
	}
}

func TestLPFloatPrimalOuterStatusesAndPublication(t *testing.T) {
	empty := lpFloatPrimal(t)
	empty.TotalIterations = 8
	lpStaticMust(t, empty.FindMaximalSolution())
	if empty.Status != StatusEmpty || empty.CoreSolver != nil || empty.TotalIterations != 8 {
		t.Fatal(empty.Status, empty.TotalIterations)
	}
	empty.A = NewFloatStaticMatrix(0, 0)
	lpStaticMust(t, empty.SolveWithTotalInf())
	if empty.Status != StatusOptimal || empty.CoreSolver != nil {
		t.Fatal(empty.Status)
	}
	// Two opposite multicolumn inequalities remain after presolve and prove
	// actual infeasibility through the core rather than a single-variable scan.
	m := lpFloatPrimal(t)
	m.Settings.SimplexStrategy = StrategyLU
	m.Settings.UseScaling = false
	for j := uint32(0); j < 2; j++ {
		m.GetOrCreateColumnInfo(j).SetLowBound(0)
		m.SetRowColumnCoefficient(0, j, 1)
		m.SetRowColumnCoefficient(1, j, 1)
	}
	lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, 1, 0))
	lpStaticMust(t, m.AddConstraint(RelationGreaterOrEqual, 2, 1))
	lpStaticMust(t, m.FindMaximalSolution())
	defer m.Close()
	if m.Status != StatusInfeasible || m.CoreSolver == nil {
		t.Fatal(m.Status)
	}
	// Before any live core is available, fixed/unmapped/missing external values
	// still follow the source's early routes.
	n := lpFloatPrimal(t)
	fixed := n.GetOrCreateColumnInfo(2)
	fixed.SetLowBound(4)
	fixed.SetUpperBound(4)
	fixed.SetFixedValue(4)
	fixed.Cost = 3
	if v, err := n.GetColumnValue(2); err != nil || v != 4 {
		t.Fatal(v, err)
	}
	if v, err := n.GetColumnValue(99); err != nil || v != 0 {
		t.Fatal(v, err)
	}
	if cost, err := n.GetCurrentCost(); err != nil || cost != 12 {
		t.Fatal(cost, err)
	}
	n.GetOrCreateColumnInfo(1).Index = 0
	if _, err := n.GetColumnValue(1); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := n.GetCurrentCost(); err != ErrBounds {
		t.Fatal(err)
	}
	n.Columns[0] = nil
	if cost, err := n.GetCurrentCost(); err != ErrBounds || cost != 0 {
		t.Fatal(cost, err)
	}
}

func TestLPFloatPrimalOuterErrorsKeepSourcePrefixes(t *testing.T) {
	m := lpFloatPrimal(t)
	if err := m.SolveWithTotalInf(); err != ErrBounds {
		t.Fatal(err)
	}
	for _, failAtCore := range []bool{false, true} {
		n := lpFloatPrimal(t)
		n.Settings.SimplexStrategy = StrategyLU
		n.Settings.UseScaling = false
		for j := uint32(0); j < 2; j++ {
			n.GetOrCreateColumnInfo(j).SetLowBound(0)
			n.SetRowColumnCoefficient(0, j, 1)
		}
		lpStaticMust(t, n.AddConstraint(RelationLessOrEqual, 2, 0))
		want := errors.New("outer boundary refused")
		if failAtCore {
			n.Settings.SetResourceLimit(&lpTestResourceLimit{err: want})
		} else {
			n.Settings.MessageOut = lpPresolveFailWriter{want}
		}
		err := n.FindMaximalSolution()
		if err != want || n.A == nil || n.A.ColumnCount() != 3 {
			t.Fatal(failAtCore, err, n.A)
		}
		if failAtCore {
			if n.CoreSolver == nil || n.TotalIterations != 0 || n.CoreSolver.TotalIterations != 1 {
				t.Fatal(n.CoreSolver, n.TotalIterations)
			}
			lpStaticMust(t, n.Close())
		} else if n.CoreSolver != nil {
			t.Fatal(n.CoreSolver)
		}
	}
}

// Source defect: lp_solver flips an upper-only coordinate, but
// lp_primal_simplex::set_core_solver_bounds retains upper_bound with zero
// upper value. This is a fidelity regression, not an optimum success claim.
func TestLPFloatPrimalOuterSourceUpperOnlyInconsistency(t *testing.T) {
	m := lpFloatPrimal(t)
	m.Settings.SimplexStrategy = StrategyLU
	m.Settings.UseScaling = false
	x := m.GetOrCreateColumnInfo(0)
	x.SetUpperBound(3)
	x.Cost = -1
	m.GetOrCreateColumnInfo(1).SetLowBound(0)
	m.SetRowColumnCoefficient(0, 0, -1)
	m.SetRowColumnCoefficient(0, 1, 1)
	lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, 2, 0))
	lpStaticMust(t, m.FindMaximalSolution())
	defer m.Close()
	value, err := m.GetColumnValue(0)
	lpStaticMust(t, err)
	cost, err := m.GetCurrentCost()
	lpStaticMust(t, err)
	// Maximize -x subject to x<=3, z>=0, -x+z<=2. The true optimum is
	// x=-2,z=0,cost=2. Source instead reports optimal at x=3,cost=-3.
	if m.Status != StatusOptimal || value != 3 || cost != -3 || m.ColumnTypes[x.Index] != ColumnUpperBound || m.UpperBounds[x.Index] != 0 {
		t.Fatal(m.Status, value, cost, m.ColumnTypes, m.UpperBounds)
	}
	optimum := lpVertexMaximum([][]float64{{1, 1, 1}}, []float64{5}, []float64{1, 0, 0}) - 3
	if optimum != 2 || cost >= optimum {
		t.Fatal(cost, optimum)
	}
}

func TestLPFloatPrimalOuterInitialStatusAndPresolveErrors(t *testing.T) {
	m := lpFloatPrimal(t)
	m.Settings.SimplexStrategy = StrategyLU
	for j := uint32(0); j < 2; j++ {
		m.GetOrCreateColumnInfo(j).SetLowBound(0)
		m.SetRowColumnCoefficient(0, j, 1)
	}
	lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, 3, 0))
	m.Status = StatusInfeasible
	m.TotalIterations = 7
	lpStaticMust(t, m.FindMaximalSolution())
	if m.A == nil || m.CoreSolver != nil || len(m.X) != 0 || m.TotalIterations != 7 {
		t.Fatal(m.A, m.CoreSolver, m.X, m.TotalIterations)
	}
	n := lpFloatPrimal(t)
	n.SetRowColumnCoefficient(0, 0, 1)
	n.Columns[0] = nil
	lpStaticMust(t, n.AddConstraint(RelationLessOrEqual, 2, 0))
	if err := n.FindMaximalSolution(); err != ErrBounds || n.A != nil {
		t.Fatal(err, n.A)
	}
	// A genuine preparation failure cannot publish a core.
	p := lpPrimalRowFixture(t, RelationLessOrEqual)
	p.ColumnTypes = nil
	if err := p.SolveWithTotalInf(); err != ErrBounds || p.CoreSolver != nil || p.A.ColumnCount() != 1 {
		t.Fatal(err, p.CoreSolver, p.A.ColumnCount())
	}
}
