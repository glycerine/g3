package lp

import (
	"bytes"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatCoreNumericalBasisUpdateIndependentOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(813991))
	for world := 0; world < 120; world++ {
		r := 2 + rng.Intn(7)
		m := lpFloatValueCore(t, uint32(r), uint32(r+1))
		if world%2 == 0 {
			m.Settings.DensityThreshold = 0
		} else {
			m.Settings.DensityThreshold = math.Inf(1)
		}
		for j := 0; j < r; j++ {
			if err := m.A.MultiplyColumn(uint32(j), 32); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < r; i++ {
				if i != j {
					if err := m.A.Set(uint32(i), uint32(j), float64(rng.Intn(5)-2)/4); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		if err := m.InitLU(); err != nil {
			t.Fatal(err)
		}
		leave := rng.Intn(r)
		d := make([]float64, r)
		before := make([]float64, r+1)
		for i := 0; i < r; i++ {
			d[i] = float64(rng.Intn(9)-4) / 4
			if i == leave {
				d[i] = 1
			}
			before[i] = float64(rng.Intn(9)-4) / 4
		}
		for i, row := range m.A.Rows {
			var incoming, rhs float64
			for _, c := range row {
				incoming += float64(c.Value * d[c.Column])
				rhs += float64(c.Value * before[c.Column])
			}
			if err := m.A.Set(uint32(i), uint32(r), incoming); err != nil {
				t.Fatal(err)
			}
			(*m.B)[i] = rhs
		}
		*m.X = append([]float64(nil), before...)
		if err := m.SolveBd(uint32(r)); err != nil {
			t.Fatal(err)
		}
		if err := m.CalculatePivotRowOfBInverse(uint32(leave)); err != nil {
			t.Fatal(err)
		}
		if err := m.CalculatePivotRowWhenInverseReady(uint32(leave)); err != nil {
			t.Fatal(err)
		}
		if math.Abs(m.PivotRow.Data[r]-1) > 1e-10 {
			t.Fatal("real pivot row", world, m.PivotRow.Data)
		}
		if world%3 == 0 {
			m.Factorization.RefactorCounter = 200
		}
		old := m.Factorization
		m.StartTracingBasisChanges()
		step := before[leave]
		if ok, err := m.UpdateBasisAndX(int32(r), int32(leave), step); !ok || err != nil {
			t.Fatal(world, ok, err)
		}
		if (*m.Basis)[leave] != uint32(r) || (*m.NBasis)[0] != uint32(leave) || !reflect.DeepEqual(m.Trace, []uint32{uint32(r), uint32(leave)}) {
			t.Fatal("basis publication", world)
		}
		for j, x := range *m.X {
			want := step
			if j < r {
				want = before[j] - float64(step*d[j])
			}
			if math.Abs(x-want) > 1e-10 {
				t.Fatal("independent nullspace update", world, j, x, want)
			}
		}
		if world%3 == 0 {
			if m.Factorization == old || !old.closed {
				t.Fatal("full refactor")
			}
		} else if m.Factorization != old || old.RefactorCounter != 1 {
			t.Fatal("actual Suhl replacement")
		}
		if bad, err := m.AMultXIsOff(); bad || err != nil {
			t.Fatal(world, bad, err)
		}
		v := append([]float64(nil), (*m.B)...)
		if err := m.Factorization.SolveBy(v); err != nil {
			t.Fatal(err)
		}
		for i, x := range v {
			if math.Abs(x-(*m.X)[(*m.Basis)[i]]) > 1e-10 {
				t.Fatal("new LU matches new basis", world, i, x)
			}
		}
	}
}

func TestLPFloatCoreNumericalUpdateSingularRollback(t *testing.T) {
	for _, onlyFeasible := range []bool{false, true} {
		for _, forceRefactor := range []bool{false, true} {
			m := lpFloatValueCore(t, 2, 3)
			if err := m.A.Set(1, 2, 1); err != nil {
				t.Fatal(err)
			}
			if err := m.SolveBd(2); err != nil {
				t.Fatal(err)
			}
			if err := m.CalculatePivotRowOfBInverse(0); err != nil {
				t.Fatal(err)
			}
			if err := m.CalculatePivotRowWhenInverseReady(0); err != nil {
				t.Fatal(err)
			}
			m.LookForFeasibleSolutionOnly = onlyFeasible
			if forceRefactor {
				m.Factorization.RefactorCounter = 200
			}
			m.StartTracingBasisChanges()
			ok, err := m.UpdateBasisAndX(2, 0, 0)
			if ok || err != nil || m.Status != StatusUnstable {
				t.Fatal(onlyFeasible, forceRefactor, ok, err, m.Status)
			}
			if onlyFeasible {
				if m.Factorization != nil || (*m.Basis)[0] != 2 || m.IterationsNoCostGrowth != 0 || len(m.Trace) != 2 {
					t.Fatal("feasible-only failed basis retained")
				}
			} else if m.Factorization == nil || m.Factorization.GetStatus() != LUOK || (*m.Basis)[0] != 0 || m.IterationsNoCostGrowth != 1 || len(m.Trace) != 0 {
				t.Fatal("old basis rollback", m.IterationsNoCostGrowth, m.Trace)
			}
		}
	}
	// Rollback still checks the restored primal vector against the RHS.
	m := lpFloatValueCore(t, 2, 3)
	if err := m.A.Set(1, 2, 1); err != nil {
		t.Fatal(err)
	}
	(*m.B)[0] = 1
	m.Factorization.RefactorCounter = 200
	var out bytes.Buffer
	m.Settings.DebugOut = &out
	if ok, err := m.UpdateBasisAndX(2, 0, 0); ok || err != nil || m.Status != StatusFloatingPointError || out.String() != "cannot restore solution\n" {
		t.Fatal(ok, err, m.Status, out.String())
	}
}

func TestLPFloatCoreNumericalUpdateResidualRecovery(t *testing.T) {
	// A real matrix change makes old LU stale. UpdateX uses the old entering
	// solve; reconstruction first fails with old factors, then succeeds after
	// rebuilding them. No replacement numerical callbacks are used.
	m := lpFloatValueCore(t, 1, 2)
	if err := m.A.Set(0, 1, 1); err != nil {
		t.Fatal(err)
	}
	*m.B = []float64{4}
	*m.X = []float64{4, 0}
	if err := m.SolveBd(1); err != nil {
		t.Fatal(err)
	}
	if err := m.A.MultiplyRow(0, 2); err != nil {
		t.Fatal(err)
	}
	m.PivotRow.Data[1] = 1
	old := m.Factorization
	if ok, err := m.UpdateBasisAndX(1, 0, 1); !ok || err != nil || !old.closed || m.Status != StatusFeasible {
		t.Fatal("actual stale-factor recovery", ok, err, m.Status)
	}
	if bad, err := m.AMultXIsOff(); bad || err != nil {
		t.Fatal(bad, err, *m.X)
	}
	// Negative tolerance makes even zero residual fail the source comparison.
	m = lpFloatValueCore(t, 1, 2)
	if err := m.A.Set(0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.SolveBd(1); err != nil {
		t.Fatal(err)
	}
	m.Settings.RefactorTolerance = -1
	m.IterationsNoCostGrowth = math.MaxUint32
	if ok, err := m.UpdateBasisAndX(1, 0, 1); ok || err != nil || m.Status != StatusFloatingPointError || m.IterationsNoCostGrowth != 0 || (*m.Basis)[0] != 0 {
		t.Fatal("recovery refusal and unsigned wrap", ok, err, m.Status, m.IterationsNoCostGrowth)
	}
	// Real drop settings refuse both old/new tiny reconstructed basic values.
	// Restoring the saved original X succeeds, so the source returns false
	// with its original feasible status and basis, after another refactor.
	m = lpFloatValueCore(t, 1, 2)
	if err := m.A.Set(0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.SolveBd(1); err != nil {
		t.Fatal(err)
	}
	if err := m.A.MultiplyColumn(0, 2); err != nil {
		t.Fatal(err)
	}
	*m.B = []float64{1e-5}
	*m.X = []float64{5e-6, 0}
	m.Settings.DropTolerance = .001
	m.Settings.RefactorTolerance = 1e-12
	if ok, err := m.UpdateBasisAndX(1, 0, 2e-5); ok || err != nil || m.Status != StatusFeasible || m.IterationsNoCostGrowth != 1 || (*m.Basis)[0] != 0 || (*m.X)[0] != 5e-6 || (*m.X)[1] != 0 {
		t.Fatal("restored feasible source refusal", ok, err, m.Status, m.IterationsNoCostGrowth, *m.X)
	}
}

func TestLPFloatCoreRestoreRefactorOutputAndFailurePrefixes(t *testing.T) {
	for _, failOutput := range []bool{false, true} {
		m := lpFloatValueCore(t, 1, 2)
		if err := m.A.MultiplyColumn(0, 0); err != nil {
			t.Fatal(err)
		}
		old := m.Factorization
		var out bytes.Buffer
		want := errors.New("refactor output refused")
		m.Settings.DebugOut = &out
		if failOutput {
			m.Settings.DebugOut = lpPresolveFailWriter{want}
		}
		err := m.RestoreXAndRefactor(1, 0, 0)
		if !old.closed || m.Factorization.GetStatus() != LUDegenerated {
			t.Fatal("actual failed owner retained")
		}
		if failOutput {
			if !errors.Is(err, want) || m.Status != StatusFeasible {
				t.Fatal(err, m.Status)
			}
		} else if err != nil || m.Status != StatusFloatingPointError || out.String() != "cannot refactor\n" {
			t.Fatal(err, m.Status, out.String())
		}
	}
	m := lpFloatValueCore(t, 1, 2)
	(*m.B)[0] = 1
	want := errors.New("restore output refused")
	m.Settings.DebugOut = lpPresolveFailWriter{want}
	if err := m.RestoreXAndRefactor(1, 0, 0); !errors.Is(err, want) || m.Status != StatusFeasible {
		t.Fatal(err, m.Status)
	}
	*m.B = nil
	if err := m.RestoreXAndRefactor(1, 0, 0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.B = []float64{0}
	m.Ed.Index = []uint32{9}
	if err := m.RestoreXAndRefactor(1, 0, 1); !errors.Is(err, ErrBounds) || (*m.X)[1] != -1 {
		t.Fatal(err, *m.X)
	}
}

func TestLPFloatCoreNumericalUpdateAndPivotGuards(t *testing.T) {
	m := lpFloatValueCore(t, 1, 2)
	m.Close()
	if ok, err := m.UpdateBasisAndX(1, 0, 0); ok || !errors.Is(err, ErrBounds) {
		t.Fatal(ok, err)
	}
	if err := m.CalculatePivotRowOfBInverse(0); !errors.Is(err, ErrBounds) || !reflect.DeepEqual(m.PivotRowB1.Data, []float64{1}) {
		t.Fatal("unit row before missing LU", err)
	}
	if err := m.InitLU(); err != nil {
		t.Fatal(err)
	}
	if err := m.CalculatePivotRowOfBInverse(99); !errors.Is(err, ErrBounds) || len(m.PivotRowB1.Index) != 0 {
		t.Fatal("clear before bad pivot", err)
	}
	m.Settings.SimplexStrategy = StrategyTableauRows
	err := m.CalculatePivotRowOfBInverse(0)
	if sourceDebugEnabled {
		if !errors.Is(err, ErrAssertion) {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	m.Settings.SimplexStrategy = StrategyLU
	if ok, err := m.UpdateBasisAndX(-1, 0, 0); ok || !errors.Is(err, ErrBounds) {
		t.Fatal(ok, err)
	}
	m.Factorization.Close()
	if ok, err := m.UpdateBasisAndX(1, 0, 0); ok || !errors.Is(err, ErrLUClosed) {
		t.Fatal(ok, err)
	}
	if err := m.RestoreXAndRefactor(-1, 0, 1); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m = lpFloatValueCore(t, 1, 2)
	if err := m.A.Set(0, 1, 1); err != nil {
		t.Fatal(err)
	}
	m.PivotRowB1.Index = []uint32{0}
	m.PivotRowB1.Data[0] = 0
	if err := m.CalculatePivotRowWhenInverseReady(99); err != nil {
		t.Fatal("precise-only row arg erased", err)
	}
	m.PivotRowB1.Data[0] = 1
	if err := m.CalculatePivotRowWhenInverseReady(0); err != nil || m.PivotRow.Data[1] != 1 {
		t.Fatal(err)
	}
	m.PivotRowB1.Index = []uint32{9}
	if err := m.CalculatePivotRowWhenInverseReady(0); !errors.Is(err, ErrBounds) || m.PivotRow.Data[1] != 0 {
		t.Fatal("pivot clear prefix", err)
	}
	if err := m.CalculatePivotRowOfBInverse(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m.PivotRowB1.Index = []uint32{0}
	m.PivotRowB1.Data[0] = 1
	m.PivotRow.Index = []uint32{9}
	if err := m.CalculatePivotRowWhenInverseReady(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m.PivotRow.Index = nil
	*m.Heading = nil
	if err := m.CalculatePivotRowWhenInverseReady(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	*m.Heading = []int32{0, -1}
	m.PivotRow.Data = nil
	if err := m.CalculatePivotRowWhenInverseReady(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
	m.PivotRowB1.Data = make([]float64, 2)
	m.PivotRowB1.Data[1] = 1
	m.PivotRowB1.Index = []uint32{1}
	if err := m.CalculatePivotRowWhenInverseReady(0); !errors.Is(err, ErrBounds) {
		t.Fatal(err)
	}
}

func TestLPFloatCoreNumericalUpdateBorrowedInputErrorPrefixes(t *testing.T) {
	m := lpFloatValueCore(t, 1, 2)
	if ok, err := m.UpdateBasisAndX(-1, 0, 1); ok || !errors.Is(err, ErrBounds) {
		t.Fatal(ok, err)
	}
	if err := m.A.Set(0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := m.SolveBd(1); err != nil {
		t.Fatal(err)
	}
	*m.B = nil
	if ok, err := m.UpdateBasisAndX(1, 0, 1); ok || !errors.Is(err, ErrBounds) || (*m.X)[1] != 1 || (*m.X)[0] != -1 {
		t.Fatal("update before residual input error", ok, err, *m.X)
	}
	*m.B = []float64{1}
	*m.X = []float64{0, 0}
	*m.Heading = nil
	if ok, err := m.UpdateBasisAndX(1, 0, 1); ok || !errors.Is(err, ErrBounds) || (*m.X)[1] != 1 || (*m.X)[0] != -1 {
		t.Fatal("update before reconstruction input error", ok, err, *m.X)
	}
	m = lpFloatValueCore(t, 2, 3)
	m.Factorization.RefactorCounter = 200
	if ok, err := m.UpdateBasisAndX(-1, 0, 0); ok || !errors.Is(err, ErrBounds) {
		t.Fatal(ok, err)
	}
	*m.Basis = (*m.Basis)[:1]
	old := m.Factorization
	ok, err := m.UpdateBasisAndX(2, 0, 0)
	if ok || err == nil || !old.closed || (*m.Basis)[0] != 2 || (*m.Heading)[0] != -1 {
		t.Fatal("basis before refactor input refusal", ok, err, *m.Basis)
	}
	m = lpFloatValueCore(t, 1, 2)
	*m.Basis = nil
	if err := m.RestoreXAndRefactor(1, 0, 0); err == nil {
		t.Fatal("actual short-basis refactor")
	}
	m = lpFloatValueCore(t, 2, 3)
	if err := m.A.Set(1, 2, 1); err != nil {
		t.Fatal(err)
	}
	(*m.B)[0] = 1
	m.Factorization.RefactorCounter = 200
	want := errors.New("update rollback output refused")
	m.Settings.DebugOut = lpPresolveFailWriter{want}
	if ok, err := m.UpdateBasisAndX(2, 0, 0); ok || !errors.Is(err, want) || (*m.Basis)[0] != 0 || m.Status != StatusFeasible {
		t.Fatal("rollback prefix before writer refusal", ok, err, m.Status)
	}
	m = lpFloatValueCore(t, 1, 2)
	if err := m.SolveBd(0); err != nil {
		t.Fatal(err)
	}
	m.PivotRow.Data[0] = 1
	ok, err = m.UpdateBasisAndX(0, 0, 0)
	if ok || m.Factorization.RefactorCounter != 1 || (sourceDebugEnabled && !errors.Is(err, ErrAssertion)) || (!sourceDebugEnabled && !errors.Is(err, ErrBounds)) {
		t.Fatal("numerical replacement precedes invalid entering assertion", ok, err)
	}
}

func TestLPFloatCoreNumericalRollbackRestoresNonzeroStep(t *testing.T) {
	m := lpFloatValueCore(t, 2, 3)
	if err := m.A.Set(1, 2, 1); err != nil {
		t.Fatal(err)
	}
	*m.B = []float64{0, 3}
	*m.X = []float64{0, 3, 0}
	if err := m.SolveBd(2); err != nil {
		t.Fatal(err)
	}
	m.Factorization.RefactorCounter = 200
	if ok, err := m.UpdateBasisAndX(2, 0, 1); ok || err != nil || m.Status != StatusUnstable || !reflect.DeepEqual(*m.X, []float64{0, 3, 0}) {
		t.Fatal("nonzero rollback", ok, err, *m.X, m.Status)
	}
}
