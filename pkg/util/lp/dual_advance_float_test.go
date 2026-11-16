package lp

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestLPFloatDualAdvanceIndependentInverseRows(t *testing.T) {
	oldInv := [][]float64{{0.5, -0.125, 0.03125}, {0, 0.25, -0.0625}, {0, 0, 0.125}}
	b := [][]float64{{2, 1, 0}, {0, 4, 2}, {0, 0, 8}}
	for world := 0; world < 150; world++ {
		r := world % 3
		in := lpFloatCoreInput(t, 3, 4)
		in.Settings.SimplexStrategy = StrategyLU
		u := []float64{float64(world%5-2) / 4, 0.5, -0.25}
		u[r] = -float64(1+world%3) / 2
		x := []float64{5, 7, 9}
		x[r] = -float64(1+world%11) / 4
		for i := 0; i < 3; i++ {
			column := 0.0
			rhs := 0.0
			for j := 0; j < 3; j++ {
				lpStaticMust(t, in.A.Set(uint32(i), uint32(j), b[i][j]))
				column += float64(b[i][j] * u[j])
				rhs += float64(b[i][j] * x[j])
			}
			lpStaticMust(t, in.A.Set(uint32(i), 3, column))
			(*in.B)[i] = rhs
		}
		*in.ColumnTypes = []ColumnType{ColumnLowBound, ColumnLowBound, ColumnLowBound, ColumnLowBound}
		(*in.Costs)[3] = 1
		can := []bool{true, true, true, true}
		m, err := NewFloatDualCoreSolver(in, &can)
		lpStaticMust(t, err)
		lpStaticMust(t, m.RecalculateD())
		lpStaticMust(t, m.StartWithInitialBasisAndMakeItDualFeasible())
		lpStaticMust(t, m.InitBetasPrecisely())
		for i := 0; i < 3; i++ {
			want := 0.0
			for _, v := range oldInv[i] {
				want += float64(v * v)
			}
			if m.Betas[i] != want {
				t.Fatal(world, i, m.Betas[i], want)
			}
		}
		m.P, m.R = int32(r), int32(r)
		m.Delta, err = m.GetDelta()
		lpStaticMust(t, err)
		if world%2 != 0 {
			m.Factorization.RefactorCounter = 200
		}
		ok, err := m.AdvanceOnKnownP()
		lpStaticMust(t, err)
		if !ok || m.Q != 3 || (*m.Basis)[r] != 3 || (*m.Heading)[r] >= 0 || (*m.Heading)[3] != int32(r) {
			t.Fatal(world, ok, *m.Basis, *m.Heading)
		}
		step := x[r] / u[r]
		// The source DSE uses the prepared triangular stage alone. For this
		// B, normalized U has two superdiagonal 1/2 entries; diagonal-scaling
		// tails are deliberately absent from this independently solved RHS.
		ready := []float64{0, 0, oldInv[r][2]}
		ready[1] = oldInv[r][1] - float64(0.5*ready[2])
		ready[0] = oldInv[r][0] - float64(0.5*ready[1])
		oldNorm := make([]float64, 3)
		for i := 0; i < 3; i++ {
			for _, v := range oldInv[i] {
				oldNorm[i] += float64(v * v)
			}
		}
		for i := 0; i < 3; i++ {
			want := x[i] - float64(step*u[i])
			if i == r {
				want = 0
			}
			if math.Abs((*m.X)[i]-want) > 1e-10 {
				t.Fatal(world, i, *m.X, want)
			}
			beta := 0.0
			for j := 0; j < 3; j++ {
				v := oldInv[i][j] - float64(u[i]/u[r]*oldInv[r][j])
				if i == r {
					v = oldInv[r][j] / u[r]
				}
				beta += float64(v * v)
			}
			if math.Abs(m.PivotRowB1.Data[i]-ready[i]) > 1e-10 {
				t.Fatal(world, i, m.PivotRowB1.Data, ready)
			}
			sourceBeta := oldNorm[i] + float64(u[i]*u[i]*oldNorm[r]/(u[r]*u[r])) - float64(2*u[i]/u[r]*ready[i])
			if i == r {
				sourceBeta = oldNorm[r] / (u[r] * u[r])
			}
			if sourceBeta < 0.0001 {
				sourceBeta = 0.0001
			}
			if math.Abs(m.Betas[i]-sourceBeta) > 1e-10 {
				t.Fatal(world, i, m.Betas[i], sourceBeta)
			}
			if world == 0 && i == 1 && (m.Betas[i] != 0.0517578125 || beta != 0.2666015625) {
				t.Fatal("source DSE discrepancy", m.Betas[i], beta)
			}
		}
		if math.Abs((*m.X)[3]-step) > 1e-10 {
			t.Fatal(world, *m.X, step)
		}
		for i := 0; i < 3; i++ {
			lhs, err := m.A.DotRow(uint32(i), *m.X)
			lpStaticMust(t, err)
			if math.Abs(lhs-(*m.B)[i]) > 1e-10 {
				t.Fatal(world, i, lhs, (*m.B)[i])
			}
		}
		ok, err = m.ProblemIsDualFeasible()
		lpStaticMust(t, err)
		if !ok {
			t.Fatal(world, m.D)
		}
		ok, err = m.DIsCorrect()
		lpStaticMust(t, err)
		if !ok {
			t.Fatal(world, m.D)
		}
		// Roll back through actual fresh LU and RHS/cost recomputation.
		lpStaticMust(t, m.RevertToPreviousBasis())
		if !reflect.DeepEqual(*m.Basis, []uint32{0, 1, 2}) || (*m.X)[3] != 0 {
			t.Fatal(world, *m.Basis, *m.X)
		}
		for i, want := range x {
			if math.Abs((*m.X)[i]-want) > 1e-10 {
				t.Fatal(world, i, *m.X, want)
			}
		}
		m.Close()
	}
}

func TestLPFloatDualAdvanceFlipsAndStops(t *testing.T) {
	in := lpFloatCoreInput(t, 1, 4)
	in.Settings.SimplexStrategy = StrategyLU
	for j, a := range []float64{2, -2, -4, -2} {
		lpStaticMust(t, in.A.Set(0, uint32(j), a))
	}
	*in.B = []float64{-20}
	*in.Costs = []float64{0, 1, 2, 8}
	*in.ColumnTypes = []ColumnType{ColumnLowBound, ColumnBoxed, ColumnBoxed, ColumnLowBound}
	*in.UpperBounds = []float64{0, 1, 1, 0}
	can := []bool{false, true, true, true}
	m, err := NewFloatDualCoreSolver(in, &can)
	lpStaticMust(t, err)
	defer m.Close()
	m.P = 0
	m.R = 0
	m.HarrisTolerance = 0
	lpStaticMust(t, m.RecalculateD())
	lpStaticMust(t, m.StartWithInitialBasisAndMakeItDualFeasible())
	m.Delta, err = m.GetDelta()
	lpStaticMust(t, err)
	ok, err := m.AdvanceOnKnownP()
	lpStaticMust(t, err)
	if !ok || !reflect.DeepEqual(*m.X, []float64{0, 1, 1, 7}) || !reflect.DeepEqual(*m.Basis, []uint32{3}) || !reflect.DeepEqual(m.D, []float64{8, -7, -14, 0}) || m.Betas[0] != 0.25 {
		t.Fatal(ok, *m.X, *m.Basis, m.D, m.Betas)
	}
	for _, status := range []Status{StatusOptimal, StatusUnknown} {
		m.Status = status
		m.TotalIterations = m.Settings.MaxTotalIterations + 1
		old := append([]float64(nil), *m.X...)
		ok, err = m.AdvanceOnKnownP()
		lpStaticMust(t, err)
		if !ok || !reflect.DeepEqual(*m.X, old) {
			t.Fatal(ok, *m.X)
		}
	}
	// A ratio refusal is a successful advance return, with tentative status.
	n := lpDualInitialFixture(t, 1, 2)
	n.P = 0
	n.R = 0
	(*n.ColumnTypes)[0] = ColumnLowBound
	(*n.X)[0] = -1
	n.Delta = -1
	ok, err = n.AdvanceOnKnownP()
	lpStaticMust(t, err)
	if !ok || n.Status != StatusTentativeDualUnbounded {
		t.Fatal(ok, n.Status)
	}
}

func TestLPFloatDualAdvanceActualDualRecovery(t *testing.T) {
	in := lpFloatCoreInput(t, 1, 3)
	in.Settings.SimplexStrategy = StrategyLU
	lpStaticMust(t, in.A.Set(0, 0, 2))
	lpStaticMust(t, in.A.Set(0, 1, -2))
	lpStaticMust(t, in.A.Set(0, 2, 1))
	*in.B = []float64{-6}
	*in.Costs = []float64{0, 1, 1}
	*in.ColumnTypes = []ColumnType{ColumnLowBound, ColumnLowBound, ColumnLowBound}
	can := []bool{true, true, true}
	m, err := NewFloatDualCoreSolver(in, &can)
	lpStaticMust(t, err)
	defer m.Close()
	m.P = 0
	m.R = 0
	m.Delta = -3
	lpStaticMust(t, m.RecalculateD())
	lpStaticMust(t, m.StartWithInitialBasisAndMakeItDualFeasible())
	lpStaticMust(t, m.CalculatePivotRowOfBInverse(0))
	lpStaticMust(t, m.CalculatePivotRowWhenInverseReady(0))
	ok, err := m.RatioTest()
	lpStaticMust(t, err)
	if !ok || m.Q != 1 {
		t.Fatal(ok, m.Q)
	}
	lpStaticMust(t, m.CalculateBetaRPrecisely())
	lpStaticMust(t, m.SolveBd(1))
	lpStaticMust(t, m.DSEFTran())
	// A real invalid remaining reduced cost triggers the source rollback path.
	m.D[2] = -10
	ok, err = m.BasisChangeAndUpdate()
	lpStaticMust(t, err)
	if ok || m.IterationsNoCostGrowth != 1 || !reflect.DeepEqual(*m.Basis, []uint32{0}) || !reflect.DeepEqual(*m.X, []float64{-3, 0, 0}) || !reflect.DeepEqual(m.D, []float64{0, 1, 1}) || m.Betas[0] != 0.25 {
		t.Fatal(ok, m.IterationsNoCostGrowth, *m.Basis, *m.X, m.D, m.Betas)
	}
}

func TestLPFloatDualBetaIEEEAndSourceClamp(t *testing.T) {
	m := lpDualInitialFixture(t, 2, 3)
	m.R = 0
	m.Q = 2
	for _, pivot := range []float64{0, math.Copysign(0, -1), 2, -2, math.Inf(1), math.NaN()} {
		for _, old := range []float64{0, 1, -1, math.NaN(), math.Inf(1)} {
			m.PivotRow.Data[2] = pivot
			m.Betas = []float64{old, 1}
			m.Ed.Data = []float64{0, 2}
			m.PivotRowB1.Data = []float64{0, 3}
			lpStaticMust(t, m.UpdateBetas())
			inv := 1 / pivot
			want := 0.0001
			v := (old * inv) * inv
			if v > want {
				want = v
			}
			other := 1 + float64(2*(float64(2*want)-float64(2*inv*3)))
			if other < 0.0001 {
				other = 0.0001
			}
			if m.Betas[0] != want {
				t.Fatal(pivot, old, m.Betas, want)
			}
			if !(math.IsNaN(other) && math.IsNaN(m.Betas[1])) && m.Betas[1] != other {
				t.Fatal(pivot, old, m.Betas, other)
			}
		}
	}
}

func TestLPFloatDualSnapAndRecoverSourceQuirks(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	m.Q = 1
	(*m.LowBounds)[1], (*m.UpperBounds)[1] = 0, 10
	for _, typ := range []ColumnType{ColumnLowBound, ColumnUpperBound, ColumnFixed, ColumnBoxed, ColumnFree, ColumnType(99)} {
		(*m.ColumnTypes)[1] = typ
		for _, x := range []float64{0, 1, 5, 9, 10, math.NaN()} {
			(*m.X)[1] = x
			want := x
			snapped := false
			switch typ {
			case ColumnLowBound, ColumnFixed:
				if x != 0 && !math.IsNaN(x) {
					want = 0
					snapped = true
				}
			case ColumnUpperBound:
				if x != 10 && !math.IsNaN(x) {
					want = 10
					snapped = true
				}
			case ColumnBoxed:
				if x != 0 && x != 10 && !math.IsNaN(x) {
					want = 0
					snapped = true
				}
			}
			got, err := m.SnapRunawayNonBasicColumn(1)
			lpStaticMust(t, err)
			if got != snapped || !(math.IsNaN(want) && math.IsNaN((*m.X)[1])) && (*m.X)[1] != want {
				t.Fatal(typ, x, got, *m.X, want)
			}
		}
	}
	for _, p := range []NonBasicColumnValuePosition{PositionAtLowBound, PositionAtFixed, PositionAtUpperBound, PositionFreeOfBounds, PositionNotAtBound} {
		m.EnteringBoundaryPosition = p
		(*m.X)[1] = 7
		err := m.RecoverLeaving()
		want := 0.0
		if p == PositionAtUpperBound {
			want = 10
		}
		if p == PositionNotAtBound {
			want = 7
		}
		if sourceDebugEnabled && (p == PositionFreeOfBounds || p == PositionNotAtBound) {
			if err != ErrAssertion {
				t.Fatal(p, err)
			}
		} else {
			lpStaticMust(t, err)
		}
		if (*m.X)[1] != want {
			t.Fatal(p, *m.X, want)
		}
	}
}

func TestLPFloatDualReducedCostDiagnosticThreshold(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	var out bytes.Buffer
	m.Settings.DebugOut = &out
	m.TotalIterations = 17
	for _, d := range []float64{0, math.Nextafter(0.001, 0), 0.001, -0.001, math.NaN(), math.Inf(1)} {
		m.D[1] = d
		out.Reset()
		ok, err := m.DIsCorrect()
		lpStaticMust(t, err)
		want := !(math.Abs(d) >= 0.001)
		if ok != want {
			t.Fatal(d, ok, want)
		}
		if !want && out.Len() == 0 {
			t.Fatal("missing diagnostic")
		}
	}
	m.D[1] = 0.001
	out.Reset()
	_, err := m.DIsCorrect()
	lpStaticMust(t, err)
	if out.String() != "total_iterations = 17\nd[1] = 0.001 but should be 0\n" {
		t.Fatal(out.String())
	}
	wantErr := errors.New("dual diagnostic")
	m.Settings.DebugOut = lpPresolveFailWriter{wantErr}
	if _, err = m.DIsCorrect(); err != wantErr {
		t.Fatal(err)
	}
	m.Settings.DebugOut = nil
	ok, err := m.DIsCorrect()
	lpStaticMust(t, err)
	if ok {
		t.Fatal(ok)
	}
}

func TestLPFloatDualAdvanceReachedGuards(t *testing.T) {
	type scenario struct {
		name string
		run  func(*FloatDualCoreSolver) error
	}
	cases := []scenario{
		{"dse-nil", func(m *FloatDualCoreSolver) error { m.Close(); return m.DSEFTran() }},
		{"beta-r-scratch", func(m *FloatDualCoreSolver) error { m.PivotRowB1.Data = nil; return m.CalculateBetaRPrecisely() }},
		{"beta-r-index", func(m *FloatDualCoreSolver) error { m.R = -1; return m.CalculateBetaRPrecisely() }},
		{"beta-init-row", func(m *FloatDualCoreSolver) error { return m.InitBetaPrecisely(2) }},
		{"beta-init-nil", func(m *FloatDualCoreSolver) error { m.Close(); return m.InitBetaPrecisely(0) }},
		{"beta-init-heading", func(m *FloatDualCoreSolver) error { *m.Basis = nil; return m.InitBetaPrecisely(0) }},
		{"beta-init-storage", func(m *FloatDualCoreSolver) error { m.Betas = nil; return m.InitBetaPrecisely(0) }},
		{"beta-init-loop", func(m *FloatDualCoreSolver) error { m.Betas = nil; return m.InitBetasPrecisely() }},
		{"beta-update-q", func(m *FloatDualCoreSolver) error { m.Q = -1; return m.UpdateBetas() }},
		{"beta-update-r", func(m *FloatDualCoreSolver) error { m.R = -1; return m.UpdateBetas() }},
		{"beta-update-ed", func(m *FloatDualCoreSolver) error { m.Ed.Data = nil; return m.UpdateBetas() }},
		{"beta-update-row", func(m *FloatDualCoreSolver) error { m.PivotRowB1.Data = nil; return m.UpdateBetas() }},
		{"d-correct-y", func(m *FloatDualCoreSolver) error { m.Close(); _, e := m.DIsCorrect(); return e }},
		{"d-correct-cost", func(m *FloatDualCoreSolver) error { *m.Costs = (*m.Costs)[:2]; _, e := m.DIsCorrect(); return e }},
		{"d-correct-d", func(m *FloatDualCoreSolver) error { m.D = nil; _, e := m.DIsCorrect(); return e }},
		{"snap-type", func(m *FloatDualCoreSolver) error { _, e := m.SnapRunawayNonBasicColumn(3); return e }},
		{"snap-low", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[2] = ColumnLowBound
			*m.LowBounds = nil
			_, e := m.SnapRunawayNonBasicColumn(2)
			return e
		}},
		{"snap-box-low", func(m *FloatDualCoreSolver) error {
			*m.LowBounds = nil
			_, e := m.SnapRunawayNonBasicColumn(2)
			return e
		}},
		{"snap-box-upper", func(m *FloatDualCoreSolver) error {
			*m.UpperBounds = nil
			_, e := m.SnapRunawayNonBasicColumn(2)
			return e
		}},
		{"snap-upper", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[2] = ColumnUpperBound
			*m.UpperBounds = nil
			_, e := m.SnapRunawayNonBasicColumn(2)
			return e
		}},
		{"dual-feasible-metadata", func(m *FloatDualCoreSolver) error { *m.ColumnTypes = nil; _, e := m.ProblemIsDualFeasible(); return e }},
		{"recover-bounds", func(m *FloatDualCoreSolver) error { *m.LowBounds = nil; return m.RecoverLeaving() }},
		{"recover-free", func(m *FloatDualCoreSolver) error {
			m.EnteringBoundaryPosition = PositionFreeOfBounds
			*m.X = nil
			return m.RecoverLeaving()
		}},
		{"revert-heading", func(m *FloatDualCoreSolver) error { *m.Heading = nil; return m.RevertToPreviousBasis() }},
		{"basis-beta", func(m *FloatDualCoreSolver) error { m.Q = -1; _, e := m.BasisChangeAndUpdate(); return e }},
		{"basis-d", func(m *FloatDualCoreSolver) error { m.D = nil; _, e := m.BasisChangeAndUpdate(); return e }},
		{"basis-flips", func(m *FloatDualCoreSolver) error {
			m.FlippedBoxed = map[uint32]bool{2: true}
			(*m.X)[2] = 0
			*m.UpperBounds = nil
			_, e := m.BasisChangeAndUpdate()
			return e
		}},
		{"advance-row", func(m *FloatDualCoreSolver) error { m.R = -1; _, e := m.AdvanceOnKnownP(); return e }},
		{"advance-pivot-metadata", func(m *FloatDualCoreSolver) error { *m.Heading = nil; _, e := m.AdvanceOnKnownP(); return e }},
		{"advance-ratio", func(m *FloatDualCoreSolver) error { *m.ColumnTypes = nil; _, e := m.AdvanceOnKnownP(); return e }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := lpDualInitialFixture(t, 2, 3)
			m.P = 0
			m.R = 0
			m.Q = 2
			m.Delta = -1
			(*m.ColumnTypes)[0], (*m.ColumnTypes)[2] = ColumnLowBound, ColumnBoxed
			(*m.X)[0] = -1
			m.PivotRow.Data[2] = -1
			m.PivotRowB1.Data = []float64{1, 0}
			m.Ed.Data = []float64{-1, 0}
			if err := c.run(m); err != ErrBounds {
				t.Fatal(err)
			}
		})
	}
}

func TestLPFloatDualAdvancePublicationAndClosedChildren(t *testing.T) {
	m := lpDualInitialFixture(t, 2, 3)
	m.R = 0
	m.Q = 2
	m.Betas = []float64{4, 9}
	m.PivotRow.Data[2] = 2
	m.Ed.Data = nil
	if err := m.UpdateBetas(); err != ErrBounds || m.Betas[0] != 1 || m.Betas[1] != 9 {
		t.Fatal(err, m.Betas)
	}
	m.Factorization.Close()
	if err := m.DSEFTran(); !errors.Is(err, ErrLUClosed) {
		t.Fatal(err)
	}
	m.Betas = []float64{99, 88}
	if err := m.InitBetasPrecisely(); !errors.Is(err, ErrLUClosed) || !reflect.DeepEqual(m.Betas, []float64{99, 88}) {
		t.Fatal(err, m.Betas)
	}
	n := lpDualInitialFixture(t, 1, 2)
	n.P = 0
	n.Q = 1
	wantErr := errors.New("revert output")
	n.Settings.DebugOut = lpPresolveFailWriter{wantErr}
	old := append([]uint32(nil), *n.Basis...)
	if err := n.RevertToPreviousBasis(); err != wantErr || !reflect.DeepEqual(*n.Basis, old) {
		t.Fatal(err, *n.Basis)
	}
}

func TestLPFloatDualAdvanceActualNumericalRefusals(t *testing.T) {
	newPivot := func() *FloatDualCoreSolver {
		m := lpDualInitialFixture(t, 1, 2)
		m.P = 0
		m.R = 0
		m.Q = 1
		m.Delta = -1
		(*m.ColumnTypes)[0], (*m.ColumnTypes)[1] = ColumnLowBound, ColumnBoxed
		(*m.UpperBounds)[1] = 10
		(*m.B)[0] = -1
		(*m.Costs)[1] = 1
		lpStaticMust(t, m.A.Set(0, 1, -1))
		lpStaticMust(t, m.RecalculateD())
		lpStaticMust(t, m.StartWithInitialBasisAndMakeItDualFeasible())
		return m
	}
	// A strict negative residual tolerance makes actual shared numerical
	// reconstruction fail. Dual still recomputes beta through the restored LU.
	m := newPivot()
	lpStaticMust(t, m.CalculatePivotRowOfBInverse(0))
	lpStaticMust(t, m.CalculatePivotRowWhenInverseReady(0))
	ok, err := m.RatioTest()
	lpStaticMust(t, err)
	if !ok {
		t.Fatal(ok)
	}
	lpStaticMust(t, m.SolveBd(1))
	lpStaticMust(t, m.DSEFTran())
	m.Betas[0] = 7
	m.Settings.RefactorTolerance = -1
	ok, err = m.BasisChangeAndUpdate()
	lpStaticMust(t, err)
	if ok || m.Status != StatusFloatingPointError || (*m.Basis)[0] != 0 || m.Betas[0] != 1 || m.IterationsNoCostGrowth != 1 {
		t.Fatal(ok, m.Status, *m.Basis, m.Betas, m.IterationsNoCostGrowth)
	}
	// Source pivot-consistency comparison uses the mutable Harris threshold.
	// Negative threshold forces comparison=1 and a real full refactor.
	n := newPivot()
	old := n.Factorization
	n.Settings.HarrisFeasibilityTolerance = -1
	ok, err = n.AdvanceOnKnownP()
	lpStaticMust(t, err)
	if !ok || !old.closed || n.Factorization == old || (*n.Basis)[0] != 1 {
		t.Fatal(ok, old.closed, *n.Basis)
	}
	for _, singular := range []bool{false, true} {
		p := newPivot()
		lpStaticMust(t, p.ChangeBasisUnconditionally(1, 0))
		lpStaticMust(t, p.InitLU())
		p.EnteringBoundaryPosition = PositionAtLowBound
		if singular {
			lpStaticMust(t, p.A.MultiplyColumn(0, 0))
		} else {
			p.Settings.RefactorTolerance = -1
		}
		lpStaticMust(t, p.RevertToPreviousBasis())
		if p.Status != StatusFloatingPointError || (*p.Basis)[0] != 0 {
			t.Fatal(singular, p.Status, *p.Basis)
		}
	}
}
