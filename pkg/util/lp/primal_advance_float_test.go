package lp

import (
	"bytes"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatPrimalAdvanceIndependentRayWorlds(t *testing.T) {
	rng := rand.New(rand.NewSource(784832))
	for world := 0; world < 180; world++ {
		rows := uint32(2 + rng.Intn(6))
		in := lpFloatCoreInput(t, rows, rows+1)
		in.Settings.SimplexStrategy = StrategyLU
		in.Settings.BackupCosts = false
		if world%2 == 0 {
			in.Settings.DensityThreshold = 0
		} else {
			in.Settings.DensityThreshold = math.Inf(1)
		}
		sign := 1.0
		if world%3 == 0 {
			sign = -1
		}
		ed := make([]float64, rows)
		before := make([]float64, rows)
		order := rng.Perm(int(rows))
		theta := math.Inf(1)
		leave := uint32(0)
		for i := uint32(0); i < rows; i++ {
			ed[i] = sign * float64(1+rng.Intn(9)) / 4
			before[i] = math.Abs(ed[i]) * float64(1+order[i])
			ratio := before[i] / math.Abs(ed[i])
			if ratio < theta {
				theta = ratio
				leave = i
			}
			(*in.X)[i] = before[i]
			(*in.ColumnTypes)[i] = ColumnLowBound
		}
		(*in.ColumnTypes)[rows] = ColumnBoxed
		(*in.LowBounds)[rows] = 0
		(*in.UpperBounds)[rows] = 100
		if sign < 0 {
			(*in.X)[rows] = 100
		}
		for i := uint32(0); i < rows; i++ {
			incoming := 0.0
			rhs := 0.0
			for j := uint32(0); j < rows; j++ {
				v := 0.0
				if i == j {
					v = 8
				}
				if j == i+1 {
					v = 1
				}
				lpStaticMust(t, in.A.Set(i, j, v))
				incoming += float64(v * ed[j])
				rhs += float64(v * before[j])
			}
			lpStaticMust(t, in.A.Set(i, rows, incoming))
			(*in.B)[i] = rhs + float64(incoming*(*in.X)[rows])
		}
		(*in.Costs)[rows] = 2 * sign
		m, err := NewFloatPrimalCoreSolver(in)
		lpStaticMust(t, err)
		lpStaticMust(t, m.InitRun())
		// Avoid Harris near-ties in this algebraic oracle; separate tolerance
		// tests cover admissible largest-pivot selection within a relaxed step.
		m.Settings.HarrisFeasibilityTolerance = 0
		m.ZeroHarrisEps()
		entering, err := m.ChooseEnteringColumn(1)
		if err != nil || entering != int32(rows) {
			t.Fatal(world, entering, err)
		}
		// Equal exact ratios are resolved by the source maximum-pivot circular
		// scan, so derive the leaving choice from the strongest admissible pivot.
		best := 0.0
		for i := uint32(0); i < rows; i++ {
			if before[i]/math.Abs(ed[i]) == theta && math.Abs(ed[i]) > best {
				best = math.Abs(ed[i])
				leave = i
			}
		}
		lpStaticMust(t, m.AdvanceOnEntering(entering))
		if m.Status != StatusFeasible || (*in.Basis)[leave] != rows || m.NonBasisList[0] != leave {
			t.Fatal(world, m.Status, *in.Basis, m.NonBasisList)
		}
		for i := uint32(0); i < rows; i++ {
			want := before[i] - float64(theta*math.Abs(ed[i]))
			if math.Abs((*in.X)[i]-want) > 1e-9 {
				t.Fatal(world, i, (*in.X)[i], want)
			}
		}
		initial := 0.0
		if sign < 0 {
			initial = 100
		}
		if math.Abs((*in.X)[rows]-(initial+float64(theta*sign))) > 1e-9 {
			t.Fatal(world, *in.X)
		}
		if math.Abs(m.D[leave]+2/math.Abs(ed[leave])) > 1e-9 || m.D[rows] != 0 {
			t.Fatal(world, m.D)
		}
		if bad, err := m.AMultXIsOff(); bad || err != nil {
			t.Fatal(world, bad, err)
		}
		lpStaticMust(t, m.Close())
	}
}

func TestLPFloatPrimalAdvanceBoundFlipAndUnbounded(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, m.A.Set(0, 1, 1))
	(*m.B)[0] = 10
	(*m.X)[0] = 10
	(*m.ColumnTypes)[0] = ColumnLowBound
	(*m.ColumnTypes)[1] = ColumnBoxed
	(*m.UpperBounds)[1] = 2
	(*m.Costs)[1] = 1
	m.Settings.BackupCosts = false
	lpStaticMust(t, m.InitRun())
	enter, err := m.ChooseEnteringColumn(1)
	lpStaticMust(t, err)
	m.IterationsNoCostGrowth = 9
	lpStaticMust(t, m.AdvanceOnEntering(enter))
	if !reflect.DeepEqual(*m.X, []float64{8, 2}) || !reflect.DeepEqual(*m.Basis, []uint32{0}) || m.IterationsNoCostGrowth != 0 || m.NonBasisList[0] != 1 {
		t.Fatal(*m.X, *m.Basis, m.IterationsNoCostGrowth, m.NonBasisList)
	}
	// Feasible-only bound flip returns before clearing the existing counter.
	m.LookForFeasibleSolutionOnly = true
	m.IterationsNoCostGrowth = 7
	m.SignOfEnteringDelta = -1
	lpStaticMust(t, m.SolveBd(1))
	lpStaticMust(t, m.AdvanceOnEnteringEqualLeaving(1, 1))
	if m.IterationsNoCostGrowth != 7 || !reflect.DeepEqual(*m.X, []float64{9, 1}) {
		t.Fatal(*m.X, m.IterationsNoCostGrowth)
	}
	u := lpFloatPrimalCore(t, 0, 1)
	(*u.ColumnTypes)[0] = ColumnFree
	(*u.Costs)[0] = 1
	u.Settings.BackupCosts = false
	lpStaticMust(t, u.InitRun())
	entering, err := u.ChooseEnteringColumn(1)
	lpStaticMust(t, err)
	lpStaticMust(t, u.AdvanceOnEntering(entering))
	if u.Status != StatusTentativeUnbounded {
		t.Fatal(u.Status)
	}
	lpStaticMust(t, u.AdvanceOnEntering(entering))
	if u.Status != StatusUnbounded {
		t.Fatal(u.Status)
	}
	// The infeasible no-leaving route escalates only after a repeated failure.
	v := lpFloatPrimalCore(t, 1, 2)
	(*v.ColumnTypes)[0] = ColumnLowBound
	(*v.X)[0] = -2
	(*v.B)[0] = -2
	(*v.Costs)[1] = 1
	lpStaticMust(t, v.InfSet.Insert(0))
	v.D[1] = 1
	v.SignOfEnteringDelta = 1
	lpStaticMust(t, v.AdvanceOnEntering(1))
	if v.Status != StatusUnstable {
		t.Fatal(v.Status)
	}
	v.D[1] = 1
	(*v.Costs)[1] = 1
	lpStaticMust(t, v.AdvanceOnEntering(1))
	if v.Status != StatusFloatingPointError {
		t.Fatal(v.Status)
	}
}

func TestLPFloatPrimalPivotComparisonThresholds(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	m.Settings.HarrisFeasibilityTolerance = 0.01
	for _, tc := range []struct {
		column, row float64
		want        int
	}{{0, 1, 1}, {1, 0, 1}, {-1, 1, 2}, {1, -1, 2}, {1, 1, 0}, {-1, -1, 0}, {1.19, 1, 0}, {1.21, 1, 1}, {math.NaN(), 1, 1}, {math.Inf(1), math.Inf(1), 1}} {
		m.Ed.Data[0] = tc.column
		m.PivotRow.Data[1] = tc.row
		got, err := m.PivotsInColumnAndRowAreDifferent(1, 0)
		if err != nil || got != tc.want {
			t.Fatal(tc, got, err)
		}
	}
	if _, err := m.PivotsInColumnAndRowAreDifferent(-1, 0); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := m.PivotsInColumnAndRowAreDifferent(1, -1); err != ErrBounds {
		t.Fatal(err)
	}
}

func TestLPFloatPrimalAdvanceRefreshRecovery(t *testing.T) {
	for _, feasibleOnly := range []bool{false, true} {
		for _, old := range []float64{4, -4} {
			m := lpFloatPrimalCore(t, 0, 1)
			(*m.ColumnTypes)[0] = ColumnFree
			(*m.Costs)[0] = 1
			m.D[0] = old
			m.LookForFeasibleSolutionOnly = feasibleOnly
			m.SignOfEnteringDelta = 1
			m.NonBasisList = []uint32{0}
			lpStaticMust(t, m.AdvanceOnEntering(0))
			want := StatusTentativeUnbounded
			if old < 0 {
				want = StatusUnknown
			}
			if feasibleOnly {
				want = StatusFloatingPointError
			}
			if m.Status != want || m.D[0] != 1 {
				t.Fatal(feasibleOnly, old, m.Status, m.D)
			}
			if !feasibleOnly && old < 0 && m.IterationsNoCostGrowth != 1 {
				t.Fatal(m.IterationsNoCostGrowth)
			}
		}
	}
	// A mismatched entering column sign is rejected before any basis/X change.
	m := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, m.A.Set(0, 1, 1))
	(*m.X)[0] = 4
	(*m.B)[0] = 4
	m.Ed.Index = []uint32{0}
	m.Ed.Data[0] = -1
	m.NonBasisList = []uint32{1}
	m.SignOfEnteringDelta = 1
	step := 1.0
	lpStaticMust(t, m.AdvanceOnEnteringAndLeaving(1, 0, &step))
	if m.Status != StatusUnstable || m.IterationsNoCostGrowth != 1 || (*m.X)[0] != 4 || (*m.Basis)[0] != 0 {
		t.Fatal(m.Status, *m.X, *m.Basis)
	}
	// Caller-provided debug output is borrowed; ordinary successful flips do
	// not emit diagnostics or change its contents.
	var out bytes.Buffer
	m.Settings.DebugOut = &out
	lpStaticMust(t, m.SolveBd(1))
	lpStaticMust(t, m.AdvanceOnEnteringEqualLeaving(1, 1))
	if out.Len() != 0 {
		t.Fatal(out.String())
	}
}

func TestLPFloatPrimalAdvanceActualResidualRecovery(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, m.A.Set(0, 1, 1))
	(*m.X)[0] = 4
	(*m.B)[0] = 8
	m.SignOfEnteringDelta = 1
	lpStaticMust(t, m.SolveBd(1))
	lpStaticMust(t, m.A.MultiplyColumn(0, 2))
	old := m.Factorization
	lpStaticMust(t, m.AdvanceOnEnteringEqualLeaving(1, 1))
	if !old.closed || (*m.X)[0] != 3.5 || (*m.X)[1] != 1 {
		t.Fatal(*m.X, old.closed)
	}
	for _, output := range []int{0, 1, 2} {
		n := lpFloatPrimalCore(t, 1, 2)
		lpStaticMust(t, n.A.Set(0, 1, 1))
		lpStaticMust(t, n.SolveBd(1))
		lpStaticMust(t, n.A.MultiplyColumn(0, 2))
		(*n.B)[0] = 1e-5
		(*n.X)[0] = 5e-6
		n.Settings.DropTolerance = .001
		n.Settings.RefactorTolerance = 1e-12
		n.SignOfEnteringDelta = 1
		n.IterationsNoCostGrowth = 9
		var out bytes.Buffer
		wantErr := errors.New("output refused")
		n.Settings.DebugOut = &out
		if output == 1 {
			n.Settings.DebugOut = nil
		}
		if output == 2 {
			n.Settings.DebugOut = lpPresolveFailWriter{wantErr}
		}
		err := n.AdvanceOnEnteringEqualLeaving(1, 2e-5)
		if output == 2 {
			if !errors.Is(err, wantErr) {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*n.X, []float64{5e-6, 0}) || n.IterationsNoCostGrowth != 10 {
			t.Fatal(*n.X, n.IterationsNoCostGrowth)
		}
		if output == 0 && out.String() != "failing in advance_on_entering_equal_leaving for entering = 1\n" {
			t.Fatal(out.String())
		}
	}
}

func TestLPFloatPrimalAdvanceInfeasibilityTransitions(t *testing.T) {
	for _, only := range []bool{false, true} {
		m := lpFloatPrimalCore(t, 1, 2)
		lpStaticMust(t, m.A.Set(0, 1, -1))
		(*m.ColumnTypes)[0] = ColumnLowBound
		(*m.X)[0] = -1
		(*m.B)[0] = -1
		(*m.Costs)[1] = 1
		m.LookForFeasibleSolutionOnly = only
		lpStaticMust(t, m.InitRun())
		enter, err := m.ChooseEnteringColumn(1)
		lpStaticMust(t, err)
		lpStaticMust(t, m.AdvanceOnEntering(enter))
		if m.Status != StatusFeasible || !m.CurrentXIsFeasible() || !reflect.DeepEqual(*m.X, []float64{0, 1}) || (*m.Basis)[0] != 1 {
			t.Fatal(m.Status, *m.X, *m.Basis)
		}
		if only {
			if m.NonBasisList[0] != 1 || !m.UsingInfeasCosts {
				t.Fatal(m.NonBasisList, m.UsingInfeasCosts)
			}
		} else if m.NonBasisList[0] != 0 || m.UsingInfeasCosts {
			t.Fatal(m.NonBasisList, m.UsingInfeasCosts)
		}
		// Same feasibility repair through a bound flip updates only changed basics.
		n := lpFloatPrimalCore(t, 1, 2)
		lpStaticMust(t, n.A.Set(0, 1, 1))
		(*n.ColumnTypes)[0] = ColumnLowBound
		(*n.X)[0] = -1
		(*n.B)[0] = -1
		n.LookForFeasibleSolutionOnly = only
		lpStaticMust(t, n.InitRun())
		lpStaticMust(t, n.SolveBd(1))
		n.SignOfEnteringDelta = -1
		lpStaticMust(t, n.AdvanceOnEnteringEqualLeaving(1, 1))
		if !n.CurrentXIsFeasible() || !reflect.DeepEqual(*n.X, []float64{0, -1}) || n.UsingInfeasCosts != only {
			t.Fatal(*n.X, n.UsingInfeasCosts)
		}
	}
	// Degenerate numerical step preserves the no-growth counter but still
	// performs the real basis exchange and replaces the priced column.
	m := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, m.A.Set(0, 1, 1))
	m.NonBasisList = []uint32{1}
	m.SignOfEnteringDelta = 1
	lpStaticMust(t, m.SolveBd(1))
	m.IterationsNoCostGrowth = 9
	step := 0.0
	lpStaticMust(t, m.AdvanceOnEnteringAndLeaving(1, 0, &step))
	if m.IterationsNoCostGrowth != 9 || (*m.Basis)[0] != 1 || m.NonBasisList[0] != 0 {
		t.Fatal(m.IterationsNoCostGrowth, *m.Basis, m.NonBasisList)
	}
}

func TestLPFloatPrimalAdvanceActualNumericalRefusals(t *testing.T) {
	for _, only := range []bool{false, true} {
		m := lpFloatPrimalCore(t, 2, 3)
		lpStaticMust(t, m.A.Set(1, 2, 1))
		lpStaticMust(t, m.SolveBd(2))
		m.NonBasisList = []uint32{2}
		m.LookForFeasibleSolutionOnly = only
		step := 0.0
		lpStaticMust(t, m.AdvanceOnEnteringAndLeaving(2, 0, &step))
		if only {
			if m.Status != StatusFloatingPointError || (*m.Basis)[0] != 2 || m.Factorization != nil {
				t.Fatal(m.Status, *m.Basis)
			}
		} else if m.Status != StatusUnstable || (*m.Basis)[0] != 0 || m.IterationsNoCostGrowth != 1 {
			t.Fatal(m.Status, *m.Basis, m.IterationsNoCostGrowth)
		}
		if m.NonBasisList[0] != 2 {
			t.Fatal("early return retains price list", m.NonBasisList)
		}
	}
	m := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, m.A.Set(0, 1, 1))
	lpStaticMust(t, m.SolveBd(1))
	m.NonBasisList = []uint32{1}
	m.Settings.RefactorTolerance = -1
	m.SignOfEnteringDelta = 1
	step := 1.0
	lpStaticMust(t, m.AdvanceOnEnteringAndLeaving(1, 0, &step))
	if m.Status != StatusFloatingPointError {
		t.Fatal(m.Status)
	}
	n := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, n.A.Set(0, 1, 1))
	lpStaticMust(t, n.SolveBd(1))
	lpStaticMust(t, n.A.MultiplyColumn(0, 0))
	n.Ed.Data[0] = 4
	n.NonBasisList = []uint32{1}
	step = 0
	lpStaticMust(t, n.AdvanceOnEnteringAndLeaving(1, 0, &step))
	if n.Status != StatusUnstable || n.IterationsNoCostGrowth != 1 || n.Factorization.GetStatus() == LUOK {
		t.Fatal(n.Status, n.IterationsNoCostGrowth)
	}
}

func TestLPFloatPrimalAdvanceReachedGuards(t *testing.T) {
	for stage := 0; stage < 6; stage++ {
		m := lpFloatPrimalCore(t, 1, 2)
		lpStaticMust(t, m.A.Set(0, 1, 1))
		lpStaticMust(t, m.SolveBd(1))
		m.NonBasisList = []uint32{1}
		m.SignOfEnteringDelta = 1
		step := 0.0
		switch stage {
		case 0:
			*m.Heading = (*m.Heading)[:1]
		case 1:
			m.PivotRowB1.Index = []uint32{99}
		case 2:
			m.PivotRow.Index = []uint32{99}
		case 3:
			m.Beta.Index = []uint32{99}
		case 4:
			m.ColumnNorms = nil
		case 5:
			m.D = nil
		}
		err := m.AdvanceOnEnteringAndLeaving(1, 0, &step)
		if err != ErrBounds {
			t.Fatal(stage, err)
		}
		if stage >= 4 && ((*m.Basis)[0] != 1 || m.NonBasisList[0] != 1) {
			t.Fatal("post-basis error prefix", stage, *m.Basis, m.NonBasisList)
		}
	}
	for stage := 0; stage < 4; stage++ {
		m := lpFloatPrimalCore(t, 1, 2)
		m.NonBasisList = []uint32{1}
		m.D[1] = 1
		(*m.Costs)[1] = 1
		switch stage {
		case 0:
			m.Close()
		case 1:
			m.D = nil
		case 2:
			*m.Basis = nil
		case 3:
			*m.ColumnTypes = nil
		}
		err := m.AdvanceOnEntering(1)
		if err != ErrBounds {
			t.Fatal(stage, err)
		}
	}
	m := lpFloatPrimalCore(t, 1, 2)
	m.NonBasisList = []uint32{1}
	step := 1.0
	if sourceDebugEnabled {
		if err := m.AdvanceOnEntering(-1); err != ErrAssertion {
			t.Fatal(err)
		}
		if err := m.AdvanceOnEnteringAndLeaving(-1, 0, &step); err != ErrAssertion {
			t.Fatal(err)
		}
		m.NonBasisList = nil
		if err := m.AdvanceOnEnteringAndLeaving(1, 0, &step); err != ErrBounds {
			t.Fatal(err)
		}
		m.NonBasisList = []uint32{0}
		if err := m.AdvanceOnEnteringAndLeaving(1, 0, &step); err != ErrAssertion {
			t.Fatal(err)
		}
		(*m.B)[0] = 1
		if err := m.AdvanceOnEnteringEqualLeaving(1, 1); err != ErrAssertion {
			t.Fatal(err)
		}
	}
	*m.B = nil
	m.Ed.Index = []uint32{0}
	if err := m.AdvanceOnEnteringEqualLeaving(1, 1); err != ErrBounds {
		t.Fatal(err)
	}
}

func TestLPFloatPrimalAdvanceBoundFlipReachedErrors(t *testing.T) {
	m := lpFloatPrimalCore(t, 0, 1)
	if err := m.AdvanceOnEnteringEqualLeaving(99, 1); err != ErrBounds {
		t.Fatal(err)
	}
	for _, missingCost := range []bool{false, true} {
		n := lpFloatPrimalCore(t, 1, 2)
		n.UsingInfeasCosts = true
		n.SignOfEnteringDelta = 1
		n.Ed.Index = []uint32{0}
		if missingCost {
			*n.Costs = nil
		} else {
			*n.ColumnTypes = nil
		}
		if err := n.AdvanceOnEnteringEqualLeaving(1, 1); err != ErrBounds {
			t.Fatal(missingCost, err)
		}
		if (*n.X)[1] != 1 {
			t.Fatal("entering output prefix", *n.X)
		}
	}
	n := lpFloatPrimalCore(t, 0, 1)
	n.UsingInfeasCosts = true
	(*n.Costs)[0] = 1
	n.CostsBackup = []float64{1}
	err := n.AdvanceOnEnteringEqualLeaving(0, 1)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	// A missing source pricing-list tail is guarded at the actual access:
	// before any changes in debug, after the completed pivot in release.
	p := lpFloatPrimalCore(t, 1, 2)
	lpStaticMust(t, p.A.Set(0, 1, 1))
	lpStaticMust(t, p.SolveBd(1))
	step := 0.0
	err = p.AdvanceOnEnteringAndLeaving(1, 0, &step)
	if err != ErrBounds || sourceDebugEnabled && (*p.Basis)[0] != 0 || !sourceDebugEnabled && (*p.Basis)[0] != 1 {
		t.Fatal(err, *p.Basis)
	}
}
