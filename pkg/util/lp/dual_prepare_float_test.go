package lp

import (
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func lpFloatDual(t *testing.T) *FloatDualSimplex {
	t.Helper()
	m, err := NewFloatDualSimplex()
	lpStaticMust(t, err)
	m.Settings.DebugOut = nil
	m.Settings.MessageOut = nil
	return m
}
func lpDualInput(m *FloatDualSimplex) FloatCoreInputs {
	return FloatCoreInputs{A: m.A, B: &m.B, X: &m.X, Costs: &m.Costs, Basis: &m.Basis, NBasis: &m.NBasis, Heading: &m.Heading, Settings: m.Settings, Names: m.FloatLPSolver, ColumnTypes: &m.CoreColumnTypes, LowBounds: &m.LowBounds, UpperBounds: &m.UpperBounds}
}
func TestLPFloatDualPreparationIndependentModels(t *testing.T) {
	rng := rand.New(rand.NewSource(186336))
	for world := 0; world < 160; world++ {
		m := lpFloatDual(t)
		nr, nc := 1+rng.Intn(8), 3+rng.Intn(5)
		for j := 0; j < nc; j++ {
			c := m.GetOrCreateColumnInfo(uint32(j))
			c.Cost = float64(rng.Intn(15)-7) / 4
			switch j % 3 {
			case 0:
				c.SetLowBound(-2)
			case 1:
				c.SetLowBound(-1)
				c.SetUpperBound(4)
			}
		}
		for i := 0; i < nr; i++ {
			shift := 0.0
			for j := 0; j < nc; j++ {
				v := float64(1+rng.Intn(7)) / 4
				if rng.Intn(2) == 0 {
					v = -v
				}
				m.SetRowColumnCoefficient(uint32(i), uint32(j), v)
				shift += float64(v * m.Columns[uint32(j)].Shift())
			}
			lpStaticMust(t, m.AddConstraint(LPRelation(i%3), shift+float64(float64(rng.Intn(11)-5)/2), uint32(i)))
		}
		lpStaticMust(t, m.FlipCosts())
		lpStaticMust(t, m.FillMatrixAAndInitRightSide())
		lpStaticMust(t, m.FillB())
		lpStaticMust(t, m.Scale())
		before := make([][]FloatRowCell, nr)
		for i := range before {
			before[i] = append([]FloatRowCell(nil), m.A.Rows[i]...)
		}
		b := append([]float64(nil), m.B...)
		relations := make([]LPRelation, nr)
		slacks, artificials := uint32(0), uint32(0)
		for i, rs := range b {
			rel := m.Constraints[m.CoreRowsToExternal[uint32(i)]].Relation
			relations[i] = rel
			if rel == RelationEqual {
				artificials++
			} else {
				slacks++
				if rel == RelationGreaterOrEqual && rs > 0 || rel == RelationLessOrEqual && rs < 0 {
					artificials++
				}
			}
		}
		m.X = []float64{91}
		lpStaticMust(t, m.AugmentMatrixAndAllocateFields())
		if m.Slacks != slacks || m.Artificials != artificials || m.A.ColumnCount() != uint32(nc)+slacks+artificials || !reflect.DeepEqual(m.X, []float64{91}) {
			t.Fatal(world, "source allocation dimensions")
		}
		lpStaticMust(t, m.FillFirstStageSolverFields())
		slack, artificial := uint32(nc), uint32(nc)+slacks
		for i, row := range m.A.Rows {
			for k, c := range before[i] {
				if row[k] != c {
					t.Fatal("structural coefficient changed")
				}
			}
			cells := map[uint32]float64{}
			for _, c := range row[len(before[i]):] {
				cells[c.Column] = c.Value
			}
			var basis uint32
			want := map[uint32]float64{}
			switch relations[i] {
			case RelationEqual:
				basis = artificial
				want[artificial] = 1
				artificial++
			case RelationGreaterOrEqual:
				want[slack] = -1
				basis = slack
				if b[i] > 0 {
					basis = artificial
					want[artificial] = 1
					artificial++
				}
				slack++
			case RelationLessOrEqual:
				want[slack] = 1
				basis = slack
				if b[i] < 0 {
					basis = artificial
					want[artificial] = -1
					artificial++
				}
				slack++
			}
			if m.Basis[i] != basis || !reflect.DeepEqual(cells, want) {
				t.Fatal(world, i, basis, m.Basis[i], cells, want)
			}
		}
		for j := uint32(0); j < uint32(nc); j++ {
			c := m.Columns[m.CoreColumnsToExternal[j]]
			if m.CoreColumnTypes[j] != ColumnBoxed {
				t.Fatal(j)
			}
			switch c.ColumnType() {
			case ColumnLowBound:
				if !m.CanEnterBasis[j] || m.LowBounds[j] != 0 || m.UpperBounds[j] != 1 || m.Costs[j] != c.Cost*m.ColumnScale[j] {
					t.Fatal(j)
				}
			case ColumnBoxed:
				if m.CanEnterBasis[j] || m.Costs[j] != 0 || m.LowBounds[j] != 0 || m.UpperBounds[j] != 0 {
					t.Fatal(j)
				}
			case ColumnFree:
				if !m.CanEnterBasis[j] || m.LowBounds[j] != -1e4 || m.UpperBounds[j] != 1e4 || m.Costs[j] != c.Cost*m.ColumnScale[j] {
					t.Fatal(j)
				}
			}
		}
		for j := uint32(nc); j < m.A.ColumnCount(); j++ {
			typ := m.LogicalColumnTypes[j-uint32(nc)]
			if m.Costs[j] != 0 || m.LowBounds[j] != 0 {
				t.Fatal(j)
			}
			if typ == ColumnFixed {
				if m.CoreColumnTypes[j] != ColumnFixed || m.CanEnterBasis[j] || m.UpperBounds[j] != 0 {
					t.Fatal(j)
				}
			} else if typ != ColumnLowBound || m.CoreColumnTypes[j] != ColumnBoxed || !m.CanEnterBasis[j] || m.UpperBounds[j] != 1 {
				t.Fatal(j)
			}
		}
		m.CopyBAsideAndSetToZeros()
		if !reflect.DeepEqual(m.BCopy, b) {
			t.Fatal(world, m.BCopy, b)
		}
		m.X = make([]float64, m.A.ColumnCount())
		m.Settings.SimplexStrategy = StrategyLU
		if world%2 == 0 {
			m.Settings.DensityThreshold = 0
		} else {
			m.Settings.DensityThreshold = math.Inf(1)
		}
		core, err := NewFloatDualCoreSolver(lpDualInput(m), &m.CanEnterBasis)
		lpStaticMust(t, err)
		t.Cleanup(func() { core.Close() })
		if core.Status != StatusFeasible || core.HarrisTolerance != m.Settings.HarrisFeasibilityTolerance {
			t.Fatal("constructor state")
		}
		var wantNB []uint32
		basic := map[uint32]bool{}
		for _, j := range m.Basis {
			basic[j] = true
		}
		for j := m.A.ColumnCount(); j > 0; {
			j--
			if !basic[j] && m.CanEnterBasis[j] {
				wantNB = append(wantNB, j)
			}
		}
		if len(wantNB) != len(m.NBasis) {
			t.Fatal(wantNB, m.NBasis)
		}
		for i, j := range wantNB {
			if m.NBasis[i] != j || m.Heading[j] != -int32(i)-1 {
				t.Fatal("filtered nonbasis")
			}
		}
		lpStaticMust(t, m.UnmarkColumnsAndFixStructuralCosts())
		lpStaticMust(t, m.RestoreRightSides())
		lpStaticMust(t, core.RestoreNonBasis())
		lpStaticMust(t, core.AdjustXBForChangedXNAndInitBetas())
		lpStaticMust(t, core.RecalculateD())
		lpStaticMust(t, core.RecalculateXBAndD())
		for i, row := range m.A.Rows {
			sum := 0.0
			for _, c := range row {
				sum += float64(c.Value * m.X[c.Column])
			}
			if math.Abs(sum-b[i]) > 1e-8 || core.Betas[i] != 1 {
				t.Fatal(world, i, sum, b[i])
			}
		}
		for j := uint32(0); j < uint32(nc); j++ {
			c := m.Columns[m.CoreColumnsToExternal[j]]
			if m.CoreColumnTypes[j] != c.ColumnType() || !m.CanEnterBasis[j] || m.Costs[j] != c.Cost*m.ColumnScale[j] || math.Abs(core.D[j]-m.Costs[j]) > 1e-10 {
				t.Fatal("restored structural", j)
			}
			switch c.ColumnType() {
			case ColumnLowBound:
				if m.LowBounds[j] != 0 {
					t.Fatal(j)
				}
			case ColumnBoxed:
				if m.LowBounds[j] != 0 || m.UpperBounds[j] != 5/m.ColumnScale[j] {
					t.Fatal(j)
				}
			case ColumnFree:
				if m.LowBounds[j] != -1e4 || m.UpperBounds[j] != 1e4 {
					t.Fatal("source leaves free stage-one values")
				}
			}
		}
		for j := uint32(nc); j < m.A.ColumnCount(); j++ {
			typ := m.LogicalColumnTypes[j-uint32(nc)]
			if m.CoreColumnTypes[j] != typ || m.CanEnterBasis[j] != (typ == ColumnLowBound) {
				t.Fatal(j)
			}
		}
		for i := range core.AWave {
			core.AWave[i] = 3
		}
		lpStaticMust(t, core.InitAWaveByZeros())
		for _, v := range core.AWave {
			if v != 0 {
				t.Fatal(v)
			}
		}
	}
}
func lpDualSingle(t *testing.T) *FloatDualSimplex {
	t.Helper()
	m := lpFloatDual(t)
	m.A = NewFloatStaticMatrix(1, 1)
	m.B = []float64{2}
	m.CoreColumnsToExternal[0] = 7
	m.Columns[7] = NewFloatColumnInfo(0)
	m.Columns[7].SetLowBound(0)
	m.ColumnScale = []float64{2}
	m.CoreRowsToExternal[0] = 9
	lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, 2, 9))
	lpStaticMust(t, m.AugmentMatrixAndAllocateFields())
	return m
}
func TestLPFloatDualPreparationSourcePrefixes(t *testing.T) {
	m := lpDualSingle(t)
	m.Columns[7].LowSet = false
	m.Columns[7].SetUpperBound(3)
	if err := m.FillFirstStageStructuralColumn(0); err == nil || !strings.Contains(err.Error(), "unexpected bound type 0 upper_bound") {
		t.Fatal(err)
	}
	m.Columns[7].UpperSet = false
	m.Columns[7].SetLowBound(0)
	m.Costs[0] = 9
	m.LowBounds = nil
	if err := m.FillFirstStageStructuralColumn(0); err != ErrBounds || !m.CanEnterBasis[0] || m.Costs[0] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetUpperBound(3)
	m.CanEnterBasis[0] = true
	m.Costs = nil
	if err := m.FillFirstStageStructuralColumn(0); err != ErrBounds || m.CanEnterBasis[0] {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].LowSet = false
	m.LowBounds = nil
	if err := m.FillFirstStageStructuralColumn(0); err != ErrBounds || m.UpperBounds[0] != 1e4 {
		t.Fatal("free upper before low", err)
	}
	m = lpDualSingle(t)
	m.CoreColumnTypes = nil
	if err := m.FillFirstStageStructuralColumn(0); err != ErrBounds || m.UpperBounds[0] != 1 || !m.CanEnterBasis[0] {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.ColumnScale = nil
	want := ErrBounds
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if err := m.FillFirstStageStructuralColumn(0); err != want || !m.CanEnterBasis[0] {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnUpperBound
	m.Costs[1] = 9
	err := m.FillFirstStageLogicalColumn(1)
	if sourceDebugEnabled {
		if err != ErrAssertion || m.Costs[1] != 0 {
			t.Fatal(err)
		}
	} else if err != nil || m.CoreColumnTypes[1] != ColumnFixed {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetFixedValue(0)
	err = m.FillFirstStageStructuralColumn(0)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && (err != nil || m.CoreColumnTypes[0] != ColumnBoxed) {
		t.Fatal(err)
	}
	// Equal row writes logical type and basis before its cost/matrix access.
	m = lpDualSingle(t)
	m.Constraints[9] = FloatConstraint{Relation: RelationEqual}
	m.Basis = nil
	s, a := uint32(1), uint32(1)
	if err := m.FillFirstStageRow(0, &s, &a); err != ErrBounds || m.LogicalColumnTypes[0] != ColumnFixed || len(m.A.Columns[1]) != 0 || a != 1 {
		t.Fatal(err)
	}
	// Greater row publishes slack coefficient before artificial type refusal.
	m = lpDualSingle(t)
	m.Constraints[9] = FloatConstraint{Relation: RelationGreaterOrEqual}
	s, a = 1, 2
	if err := m.FillFirstStageRow(0, &s, &a); err != ErrBounds || len(m.A.Columns[1]) != 1 || s != 1 || a != 2 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	delete(m.Constraints, 9)
	s, a = 1, 1
	if err := m.FillFirstStageRow(0, &s, &a); err != ErrBounds {
		t.Fatal(err)
	}
	if _, ok := m.Constraints[9]; !ok {
		t.Fatal("default constraint not inserted")
	}
	m = lpDualSingle(t)
	m.BCopy = []float64{8}
	m.CopyBAsideAndSetToZeros()
	m.CopyBAsideAndSetToZeros()
	if !reflect.DeepEqual(m.BCopy, []float64{8, 2, 0}) {
		t.Fatal(m.BCopy)
	}
	m.A = NewFloatStaticMatrix(2, 2)
	m.B = []float64{1, 2, 3}
	m.BCopy = []float64{9, 8}
	lpStaticMust(t, m.RestoreRightSides())
	if !reflect.DeepEqual(m.B, []float64{9, 8, 3}) {
		t.Fatal(m.B)
	}

}

func TestLPFloatDualPreparationIEEEAndResize(t *testing.T) {
	for _, rel := range []LPRelation{RelationEqual, RelationLessOrEqual, RelationGreaterOrEqual} {
		for _, rhs := range []float64{-2, math.Copysign(0, -1), 0, 2, math.NaN(), math.Inf(1), math.Inf(-1)} {
			m := lpFloatDual(t)
			m.A = NewFloatStaticMatrix(1, 0)
			m.B = []float64{rhs}
			m.CoreRowsToExternal[0] = 7
			lpStaticMust(t, m.AddConstraint(rel, rhs, 7))
			lpStaticMust(t, m.AugmentMatrixAndAllocateFields())
			lpStaticMust(t, m.FillFirstStageSolverFields())
			artificial := rel == RelationEqual || rel == RelationLessOrEqual && rhs < 0 || rel == RelationGreaterOrEqual && rhs > 0
			wantSlacks := uint32(1)
			if rel == RelationEqual {
				wantSlacks = 0
			}
			wantArt := uint32(0)
			if artificial {
				wantArt = 1
			}
			if m.Slacks != wantSlacks || m.Artificials != wantArt {
				t.Fatal(rel, rhs, m.Slacks, m.Artificials)
			}
			j := m.Basis[0]
			coefficient := 1.0
			if rel == RelationGreaterOrEqual && !artificial || rel == RelationLessOrEqual && artificial {
				coefficient = -1
			}
			if len(m.A.Columns[j]) != 1 || m.A.Rows[0][m.A.Columns[j][0].Offset].Value != coefficient {
				t.Fatal(rel, rhs, m.A.Rows)
			}
			if artificial && m.CoreColumnTypes[j] != ColumnFixed {
				t.Fatal(rel, rhs)
			}
		}
	}
	m := lpFloatDual(t)
	m.A = NewFloatStaticMatrix(0, 2)
	m.CoreColumnTypes = []ColumnType{ColumnFixed, ColumnBoxed, ColumnLowBound}
	m.LogicalColumnTypes = []ColumnType{ColumnFixed}
	m.Costs = []float64{3, 4, 5}
	m.LowBounds = []float64{-2, -3, -4}
	m.UpperBounds = []float64{8, 9, 10}
	m.CanEnterBasis = []bool{true, false, true}
	m.Basis = []uint32{9}
	m.X = []float64{17}
	lpStaticMust(t, m.AugmentMatrixAndAllocateFields())
	if !reflect.DeepEqual(m.Costs, []float64{3, 4}) || !reflect.DeepEqual(m.CoreColumnTypes, []ColumnType{ColumnFixed, ColumnBoxed}) || len(m.LogicalColumnTypes) != 0 || len(m.Basis) != 0 || !reflect.DeepEqual(m.X, []float64{17}) || !reflect.DeepEqual(m.CanEnterBasis, []bool{true, false}) {
		t.Fatal("source resize lost retained prefix")
	}
	m = lpDualSingle(t)
	m.Slacks, m.Artificials = math.MaxUint32, 0
	// Existing source counters accumulate and wrap before column augmentation.
	lpStaticMust(t, m.AugmentMatrixAndAllocateFields())
	if m.Slacks != 0 || m.A.ColumnCount() != 2 {
		t.Fatal(m.Slacks, m.A.ColumnCount())
	}
}

func TestLPFloatDualPreparationMetadataAndStageTwoPrefixes(t *testing.T) {
	m := lpDualSingle(t)
	delete(m.CoreColumnsToExternal, 0)
	_, err := m.structuralInfo(0, true)
	if sourceDebugEnabled {
		if err != ErrAssertion || len(m.CoreColumnsToExternal) != 0 {
			t.Fatal(err)
		}
	} else if err != ErrBounds || len(m.CoreColumnsToExternal) != 1 || m.Columns[0] != nil {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	delete(m.Columns, 7)
	_, err = m.structuralInfo(0, true)
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != ErrBounds {
		t.Fatal(err)
	}
	m.Columns[7] = nil
	if _, err := m.structuralInfo(0, true); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes = nil
	if _, err := m.GetColumnType(1); err != ErrBounds {
		t.Fatal(err)
	}
	if _, err := m.GetColumnType(2); sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != ErrBounds {
		t.Fatal(err)
	}
	if err := m.SetTypeForLogical(0, ColumnFixed); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnLowBound
	m.Costs[1] = 9
	m.CanEnterBasis = nil
	if err := m.FillFirstStageLogicalColumn(1); err != ErrBounds || m.Costs[1] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnLowBound
	m.CoreColumnTypes = nil
	if err := m.FillFirstStageLogicalColumn(1); err != ErrBounds || !m.CanEnterBasis[1] {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnFixed
	m.LowBounds = nil
	if err := m.FillFirstStageLogicalColumn(1); err != ErrBounds || m.CoreColumnTypes[1] != ColumnFixed {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnLowBound
	m.UpperBounds = nil
	if err := m.FillFirstStageLogicalColumn(1); err != ErrBounds || m.LowBounds[1] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Costs = nil
	if err := m.FillFirstStageLogicalColumn(1); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnLowBound
	m.CoreColumnTypes = nil
	if err := m.FixLogicalForStage2(1); err != ErrBounds || m.LowBounds[1] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnFixed
	m.UpperBounds = nil
	if err := m.FixLogicalForStage2(1); err != ErrBounds || m.LowBounds[1] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnFixed
	m.CoreColumnTypes = nil
	if err := m.FixLogicalForStage2(1); err != ErrBounds || m.UpperBounds[1] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.LogicalColumnTypes[0] = ColumnFree
	err = m.FixLogicalForStage2(1)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	want := ErrBounds
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if err := m.FixLogicalForStage2(0); err != want {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.CanEnterBasis = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds || m.CoreColumnTypes[0] != ColumnLowBound {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetUpperBound(6)
	m.LowBounds = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds || m.UpperBounds[0] != 3 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetUpperBound(6)
	m.ColumnScale = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetUpperBound(6)
	m.UpperBounds = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetUpperBound(6)
	m.CoreColumnTypes = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds || m.LowBounds[0] != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].LowSet = false
	m.CoreColumnTypes = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds || !m.CanEnterBasis[0] {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetUpperBound(6)
	m.Columns[7].LowSet = false
	err = m.FixStructuralForStage2(0)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7].SetFixedValue(2)
	m.Columns[7].SetUpperBound(6)
	err = m.FixStructuralForStage2(0)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Columns[7] = nil
	if err := m.FixStructuralForStage2(0); err != ErrBounds {
		t.Fatal(err)
	}
	// Missing constraint keys are published before undefined source fields are read.
	m = lpDualSingle(t)
	delete(m.CoreRowsToExternal, 0)
	delete(m.Constraints, 0)
	s, a := uint32(1), uint32(1)
	if err := m.FillFirstStageRow(0, &s, &a); err != ErrBounds || m.CoreRowsToExternal[0] != 0 {
		t.Fatal(err)
	}
	if _, ok := m.Constraints[0]; !ok {
		t.Fatal("default constraint not published")
	}
	m = lpDualSingle(t)
	m.B = nil
	if err := m.FillFirstStageRow(0, &s, &a); err != ErrBounds {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Constraints[9] = FloatConstraint{Relation: RelationEqual}
	m.Costs = nil
	if err := m.FillFirstStageRow(0, &s, &a); err != ErrBounds || m.Basis[0] != a || len(m.A.Columns[a]) != 0 {
		t.Fatal(err)
	}
	m = lpDualSingle(t)
	m.Constraints[9] = FloatConstraint{Relation: LPRelation(99)}
	lpStaticMust(t, m.FillFirstStageRow(0, &s, &a))
	if s != 1 || a != 1 {
		t.Fatal(s, a)
	}
	m = lpDualSingle(t)
	m.BCopy = nil
	before := append([]float64(nil), m.B...)
	if err := m.RestoreRightSides(); err != ErrBounds || !reflect.DeepEqual(m.B, before) {
		t.Fatal(err)
	}
	for method := 0; method < 7; method++ {
		m = lpFloatDual(t)
		switch method {
		case 0:
			_, err = m.GetColumnType(0)
		case 1:
			err = m.FillFirstStageSolverFields()
		case 2:
			err = m.FillFirstStageCostsBoundsAndTypes()
		case 3:
			err = m.AugmentMatrixAndAllocateFields()
		case 4:
			err = m.RestoreRightSides()
		case 5:
			err = m.UnmarkColumnsAndFixStructuralCosts()
		case 6:
			err = m.FillFirstStageRow(0, nil, nil)
		}
		if err != ErrBounds {
			t.Fatal(method, err)
		}
	}
}

func TestLPFloatDualCostFlipIEEEAndPublication(t *testing.T) {
	m := lpFloatDual(t)
	for j, v := range []float64{2, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN()} {
		m.GetOrCreateColumnInfo(uint32(j)).Cost = v
	}
	lpStaticMust(t, m.FlipCosts())
	if m.Columns[0].Cost != -2 || m.Columns[1].Cost != 0 || math.Signbit(m.Columns[1].Cost) || !math.IsInf(m.Columns[2].Cost, -1) || !math.IsInf(m.Columns[3].Cost, 1) || !math.IsNaN(m.Columns[4].Cost) {
		t.Fatal("cost flip IEEE behavior")
	}
	lpStaticMust(t, m.FlipCosts())
	if m.Columns[0].Cost != 2 || !math.Signbit(m.Columns[1].Cost) {
		t.Fatal("double flip")
	}
	// libstdc++ order visits 4 3 2 1 0: columns 4..2 flip, then the null
	// column 1 stops the walk before column 0.
	m.Columns[1] = nil
	if err := m.FlipCosts(); err != ErrBounds || m.Columns[0].Cost != 2 || !math.IsInf(m.Columns[2].Cost, -1) || !math.IsInf(m.Columns[3].Cost, 1) {
		t.Fatal("cost flip failure prefix", err)
	}
	// Source metadata aliases are visited per key, so the shared value flips twice.
	m = lpFloatDual(t)
	c := m.GetOrCreateColumnInfo(0)
	c.Cost = 3
	m.Columns[1] = c
	lpStaticMust(t, m.FlipCosts())
	if c.Cost != 3 {
		t.Fatal(c.Cost)
	}
}
