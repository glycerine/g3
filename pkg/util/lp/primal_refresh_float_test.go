package lp

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatPrimalRefreshIndependentBasisWorlds(t *testing.T) {
	rng := rand.New(rand.NewSource(5741010))
	for world := 0; world < 160; world++ {
		const rows = 4
		const cols = 11
		in := lpFloatCoreInput(t, rows, cols)
		in.Settings.SimplexStrategy = StrategyLU
		if world%2 == 0 {
			in.Settings.DensityThreshold = 0
		} else {
			in.Settings.DensityThreshold = math.Inf(1)
		}
		var b [rows][rows]float64
		for i := 0; i < rows; i++ {
			b[i][i] = float64(2 + i)
			for k := i + 1; k < rows; k++ {
				b[i][k] = float64(rng.Intn(5) - 2)
			}
		}
		coord := make([][]float64, cols)
		for j := 0; j < cols; j++ {
			coord[j] = make([]float64, rows)
			for i := 0; i < rows; i++ {
				if j < rows {
					if i == j {
						coord[j][i] = 1
					}
				} else {
					coord[j][i] = float64(rng.Intn(13)-6) / 2
				}
			}
			for i := 0; i < rows; i++ {
				v := 0.0
				for k := 0; k < rows; k++ {
					v += float64(b[i][k] * coord[j][k])
				}
				lpStaticMust(t, in.A.Set(uint32(i), uint32(j), v))
			}
			(*in.Costs)[j] = float64(rng.Intn(17) - 8)
		}
		enter := uint32(4 + world%7)
		row := world % rows
		if coord[enter][row] == 0 {
			row = -1
			for i, v := range coord[enter] {
				if v != 0 {
					row = i
					break
				}
			}
			if row < 0 {
				t.Fatal("all zero entering fixture")
			}
		}
		leave := uint32(row)
		m, err := NewFloatPrimalCoreSolver(in)
		lpStaticMust(t, err)
		lpStaticMust(t, m.InitReducedCostsForOneIteration())
		lpStaticMust(t, m.SolveBd(enter))
		lpStaticMust(t, m.CalculatePivotRowOfBInverse(uint32(row)))
		lpStaticMust(t, m.CalculatePivotRowWhenInverseReady(uint32(row)))
		lpStaticMust(t, m.CalcWorkingVectorBetaForColumnNorms())
		// Independent forward substitution for B^T beta = B^-1 A_enter.
		beta := make([]float64, rows)
		for i := 0; i < rows; i++ {
			v := coord[enter][i]
			for k := 0; k < i; k++ {
				v -= float64(b[k][i] * beta[k])
			}
			beta[i] = v / b[i][i]
			if math.Abs(m.Beta.Data[i]-beta[i]) > 1e-10 {
				t.Fatal(world, "beta", i, m.Beta.Data[i], beta[i])
			}
		}
		for j := 0; j < cols; j++ {
			m.ColumnNorms[j] = 1
			for _, v := range coord[j] {
				m.ColumnNorms[j] += float64(v * v)
			}
		}
		lpStaticMust(t, m.ChangeBasis(enter, leave))
		lpStaticMust(t, m.UpdateColumnNorms(enter, leave))
		lpStaticMust(t, m.UpdateReducedCostsFromPivotRow(enter, leave))
		// Independent explicit inverse of the single replaced coordinate column.
		for j := 0; j < cols; j++ {
			if (*m.Heading)[j] >= 0 {
				if math.Abs(m.D[j]) > 1e-10 {
					t.Fatal("basic cost", j, m.D[j])
				}
				continue
			}
			transformed := append([]float64(nil), coord[j]...)
			transformed[row] = coord[j][row] / coord[enter][row]
			for i := 0; i < rows; i++ {
				if i != row {
					transformed[i] -= float64(coord[enter][i] * transformed[row])
				}
			}
			wantNorm := 1.0
			wantCost := (*in.Costs)[j]
			for i, v := range transformed {
				wantNorm += float64(v * v)
				wantCost -= float64((*in.Costs)[(*in.Basis)[i]] * v)
			}
			if math.Abs(m.ColumnNorms[j]-wantNorm) > 1e-9 || math.Abs(m.D[j]-wantCost) > 1e-9 {
				t.Fatalf("world %d j %d norm %g want %g cost %g want %g", world, j, m.ColumnNorms[j], wantNorm, m.D[j], wantCost)
			}
		}
		// Refresh uses the new actual factorization and direct basis-cost dot.
		lpStaticMust(t, m.InitLU())
		target := (*m.NBasis)[0]
		lpStaticMust(t, m.SolveBd(target))
		original := m.D[target]
		if math.Abs(original) > m.EpsilonOfReducedCost {
			code, err := m.RefreshReducedCostAtEnteringAndCheckThatItIsOff(target)
			if err != nil || code != 0 || math.Abs(m.D[target]-original) > 1e-9 {
				t.Fatal(world, code, err, original, m.D[target])
			}
		}
		lpStaticMust(t, m.Close())
	}
}

func TestLPFloatPrimalRefreshClassificationAndNormLifecycle(t *testing.T) {
	m := lpFloatPrimalCore(t, 0, 1)
	m.EpsilonOfReducedCost = 1
	for _, old := range []float64{-10, -2, 2, 10} {
		for _, fresh := range []float64{-15, -10, -5, -1, 0, 1, 5, 10, 15} {
			m.D[0] = old
			(*m.Costs)[0] = fresh
			want := 0
			if math.Abs(old-fresh) > math.Abs(old)/2 {
				want = 1
			}
			if old > 1 && fresh <= 0 || old <= 1 && fresh > -1 {
				want = 2
			}
			got, err := m.RefreshReducedCostAtEnteringAndCheckThatItIsOff(0)
			if err != nil || got != want || m.D[0] != fresh {
				t.Fatal(old, fresh, got, want, err)
			}
		}
	}
	for _, feasible := range []bool{false, true} {
		if feasible {
			lpStaticMust(t, m.InfSet.Clear())
		} else {
			lpStaticMust(t, m.InfSet.Insert(0))
		}
		for _, using := range []bool{false, true} {
			m.UsingInfeasCosts = using
			for _, strategy := range []SimplexStrategy{StrategyLU, StrategyTableauRows, StrategyTableauCosts} {
				m.Settings.SimplexStrategy = strategy
				if m.NeedToSwitchCosts() != (strategy != StrategyTableauRows && feasible == using) {
					t.Fatal(feasible, using, strategy)
				}
			}
		}
	}
	m.Settings.ColumnNormsUpdateFrequency = 0
	m.ColumnNormUpdateCounter = 0
	m.Settings.RandomSeed(8)
	lpStaticMust(t, m.UpdateOrInitColumnNorms(99, 99))
	state := uint32(8)*214013 + 2531011
	want := 1 + float64(((state>>16)&32767)%10000)/100000
	if m.ColumnNorms[0] != want || m.ColumnNormUpdateCounter != 0 {
		t.Fatal(m.ColumnNorms, m.ColumnNormUpdateCounter)
	}
	m.Settings.ColumnNormsUpdateFrequency = 3
	m.PivotRow.Data[0] = 1
	m.Ed.Index = nil
	m.ColumnNorms[0] = 77
	lpStaticMust(t, m.UpdateOrInitColumnNorms(0, 0))
	if m.ColumnNormUpdateCounter != 1 || !reflect.DeepEqual(m.ColumnNorms, []float64{1}) {
		t.Fatal(m.ColumnNorms, m.ColumnNormUpdateCounter)
	}
}

func TestLPFloatPrimalRefreshIEEEAndCostTransitions(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 3)
	lpStaticMust(t, m.ChangeBasis(1, 0))
	m.PivotRow.Data[1] = 2
	m.PivotRow.Data[2] = 3
	m.PivotRow.Index = []uint32{1, 2, 2, 0}
	m.D[1] = 8
	m.D[2] = 20
	(*m.Costs)[0] = 5
	lpStaticMust(t, m.InfSet.Insert(0))
	lpStaticMust(t, m.UpdateReducedCostsFromPivotRow(1, 0))
	if m.D[0] != -9 || m.D[1] != 0 || m.D[2] != -4 || (*m.Costs)[0] != 0 {
		t.Fatal(m.D, *m.Costs)
	}
	m.Settings.UseBreakpointsInFeasibilitySearch = true
	m.D[1] = 8
	(*m.Costs)[0] = 5
	lpStaticMust(t, m.UpdateReducedCostsFromPivotRow(1, 0))
	if m.D[0] != -4 || (*m.Costs)[0] != 5 {
		t.Fatal(m.D, *m.Costs)
	}
	// Indexed norm counts duplicate sparse entries and ignores unindexed data.
	m.Ed.Data[0] = 3
	m.Ed.Index = []uint32{0, 0}
	norm, err := m.CalculateNormOfEnteringExactly()
	if err != nil || norm != 19 {
		t.Fatal(norm, err)
	}
	m.Ed.Index = nil
	m.PivotRow.Index = nil
	m.PivotRow.Data[1] = 1e6
	lpStaticMust(t, m.UpdateColumnNorms(1, 0))
	if m.ColumnNorms[0] != 1e-6 {
		t.Fatal(m.ColumnNorms)
	}
	// std::max must retain an unordered first operand, unlike math.Max.
	m.PivotRow.Data[1] = 1
	m.PivotRow.Data[2] = 0
	m.PivotRow.Index = []uint32{0, 2}
	m.ColumnNorms[2] = math.NaN()
	lpStaticMust(t, m.UpdateColumnNorms(1, 0))
	if !math.IsNaN(m.ColumnNorms[2]) {
		t.Fatal(m.ColumnNorms)
	}
	(*m.ColumnTypes)[2] = ColumnFixed
	m.ColumnNorms[2] = 77
	lpStaticMust(t, m.UpdateColumnNorms(1, 0))
	if m.ColumnNorms[2] != 77 {
		t.Fatal(m.ColumnNorms)
	}
	// IEEE comparisons preserve the source refresh classification.
	m.Ed.Index = nil
	m.Ed.Data[0] = 0
	for _, pair := range [][2]float64{{2, math.NaN()}, {math.Inf(1), math.Inf(1)}, {math.Inf(-1), math.Inf(-1)}} {
		m.D[2] = pair[0]
		(*m.Costs)[2] = pair[1]
		code, err := m.RefreshReducedCostAtEnteringAndCheckThatItIsOff(2)
		if err != nil || code != 0 {
			t.Fatal(pair, code, err)
		}
	}
}

func TestLPFloatPrimalRefreshPublicationAndGuards(t *testing.T) {
	for stage := 0; stage < 6; stage++ {
		m := lpFloatPrimalCore(t, 1, 3)
		lpStaticMust(t, m.ChangeBasis(1, 0))
		m.PivotRow.Data[1] = 2
		m.PivotRow.Data[2] = 3
		m.PivotRow.Index = []uint32{2}
		m.D[1] = 8
		m.D[2] = 20
		switch stage {
		case 0:
			m.PivotRow.Data = nil
		case 1:
			m.PivotRow.Index = []uint32{99}
		case 2:
			m.D = m.D[:2]
		case 3:
			m.D = nil
		case 4:
			*m.Heading = (*m.Heading)[:1]
		case 5:
			lpStaticMust(t, m.InfSet.Insert(0))
			*m.Costs = nil
		}
		err := m.UpdateReducedCostsFromPivotRow(1, 0)
		if err != ErrBounds {
			t.Fatalf("stage %d: %v", stage, err)
		}
		if stage == 5 && (m.D[0] != -4 || m.D[1] != 8 || m.D[2] != 8) {
			t.Fatal("reduced cost mutation prefix", m.D)
		}
	}
	for stage := 0; stage < 5; stage++ {
		m := lpFloatPrimalCore(t, 1, 2)
		m.D[1] = 8
		(*m.Costs)[1] = 10
		m.Ed.Data[0] = 2
		switch stage {
		case 0:
			m.D = nil
		case 1:
			*m.Costs = nil
		case 2:
			*m.Basis = nil
		case 3:
			m.Ed.Data = nil
		case 4:
			(*m.Basis)[0] = 99
		}
		if _, err := m.RefreshReducedCostAtEnteringAndCheckThatItIsOff(1); err != ErrBounds {
			t.Fatal(stage, err)
		}
		if stage != 0 && m.D[1] != 8 {
			t.Fatal("unpublished incomplete dot", stage, m.D)
		}
	}
	m := lpFloatPrimalCore(t, 1, 2)
	m.D[1] = 0
	_, err := m.RefreshReducedCostAtEnteringAndCheckThatItIsOff(1)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	if sourceDebugEnabled {
		if err := m.UpdateReducedCostsFromPivotRow(1, 0); err != ErrAssertion {
			t.Fatal(err)
		}
	}
	// The copied beta remains published when the absent LU guard is reached.
	lpStaticMust(t, m.Factorization.Close())
	m.Factorization = nil
	m.Ed.Data[0] = 4
	m.Ed.Index = []uint32{0}
	if err := m.CalcWorkingVectorBetaForColumnNorms(); err != ErrBounds || m.Beta.Data[0] != 4 {
		t.Fatal(err, m.Beta)
	}
	m.Beta.Index = []uint32{7}
	if err := m.CalcWorkingVectorBetaForColumnNorms(); err != ErrBounds {
		t.Fatal(err)
	}
	m.Ed.Index = []uint32{0, 7}
	if norm, err := m.CalculateNormOfEnteringExactly(); err != ErrBounds || norm != 17 {
		t.Fatal(norm, err)
	}
	m.Settings.ColumnNormsUpdateFrequency = 0
	m.ColumnNormUpdateCounter = 1
	err = m.UpdateOrInitColumnNorms(99, 99)
	if sourceDebugEnabled {
		if err != ErrAssertion || m.ColumnNormUpdateCounter != 1 {
			t.Fatal(err, m.ColumnNormUpdateCounter)
		}
	} else if err != ErrBounds || m.ColumnNormUpdateCounter != 2 {
		t.Fatal(err, m.ColumnNormUpdateCounter)
	}
}

func TestLPFloatPrimalNormUpdateReachedBounds(t *testing.T) {
	for stage := 0; stage < 7; stage++ {
		m := lpFloatPrimalCore(t, 1, 2)
		m.PivotRow.Data[0] = 2
		m.PivotRow.Data[1] = 1
		m.PivotRow.Index = []uint32{1}
		m.Ed.Data[0] = 3
		m.Ed.Index = []uint32{0}
		wantErr := ErrBounds
		switch stage {
		case 0:
			m.Ed.Index = []uint32{8}
		case 1:
			m.ColumnNorms = nil
		case 2:
			m.PivotRow.Index = []uint32{8}
		case 3:
			m.PivotRow.Data = append(m.PivotRow.Data, 1)
			m.PivotRow.Index = []uint32{2}
			if sourceDebugEnabled {
				wantErr = ErrAssertion
			}
		case 4:
			*m.ColumnTypes = nil
		case 5:
			m.ColumnNorms = m.ColumnNorms[:1]
		case 6:
			lpStaticMust(t, m.A.Set(0, 1, 2))
			(*m.ColumnTypes)[1] = ColumnFixed
			m.Beta.Data = nil
		}
		err := m.UpdateColumnNorms(0, 0)
		if err != wantErr {
			t.Fatal(stage, err, wantErr)
		}
		if stage >= 2 && m.ColumnNorms[0] != 2.5 {
			t.Fatal("leaving norm publication", stage, m.ColumnNorms)
		}
	}
	m := lpFloatPrimalCore(t, 1, 2)
	m.D = m.D[:1]
	m.D[0] = 8
	m.PivotRow.Data[0] = 2
	if err := m.UpdateReducedCostsFromPivotRow(0, 1); err != ErrBounds || m.D[0] != 8 {
		t.Fatal(err, m.D)
	}
	if err := m.UpdateColumnNorms(99, 0); err != ErrBounds {
		t.Fatal(err)
	}
}
