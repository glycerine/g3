package lp

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func lpDualInitialFixture(t *testing.T, rows, cols uint32) *FloatDualCoreSolver {
	t.Helper()
	in := lpFloatCoreInput(t, rows, cols)
	in.Settings.SimplexStrategy = StrategyLU
	for i := uint32(0); i < rows; i++ {
		lpStaticMust(t, in.A.Set(i, i, 1))
	}
	can := make([]bool, cols)
	for i := range can {
		can[i] = true
	}
	m, err := NewFloatDualCoreSolver(in, &can)
	lpStaticMust(t, err)
	t.Cleanup(func() { m.Close() })
	return m
}

func TestLPFloatDualInitialIndependentEquations(t *testing.T) {
	for world := 0; world < 160; world++ {
		m := lpDualInitialFixture(t, 3, 8)
		want := []float64{float64(world - 20), 3, -7, 0, 0, 0, 9, -11}
		types := []ColumnType{ColumnBoxed, ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnBoxed, ColumnLowBound, ColumnFixed, ColumnFree}
		copy(*m.ColumnTypes, types)
		for j := 3; j < 8; j++ {
			(*m.LowBounds)[j], (*m.UpperBounds)[j] = float64(j-9), float64(j+2)
			(*m.X)[j] = want[j]
			m.D[j] = float64(j - 4)
			if types[j] == ColumnBoxed {
				want[j] = (*m.LowBounds)[j]
				if j == 3 {
					want[j] = (*m.UpperBounds)[j]
				}
			} else if types[j] == ColumnLowBound {
				want[j] = (*m.LowBounds)[j]
			}
			for i := 0; i < 3; i++ {
				lpStaticMust(t, m.A.Set(uint32(i), uint32(j), float64((world+i+j)%9-4)/4))
			}
		}
		for i := 0; i < 3; i++ {
			b := want[i]
			for j := 3; j < 8; j++ {
				b += float64(float64((world+i+j)%9-4) / 4 * want[j])
			}
			(*m.B)[i] = b
			(*m.X)[i] = 1000
			m.Betas[i] = 99
		}
		lpStaticMust(t, m.StartWithInitialBasisAndMakeItDualFeasible())
		for j, v := range want {
			if (*m.X)[j] != v {
				t.Fatal(world, j, (*m.X)[j], v)
			}
		}
		for i, beta := range m.Betas {
			if beta != 1 {
				t.Fatal(i, beta)
			}
		}
	}
}

func TestLPFloatDualInitialStrictSignsAndEndpoints(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	values := []float64{math.Inf(-1), -1e-5, math.Nextafter(-1e-5, 0), math.Copysign(0, -1), 0, math.Nextafter(1e-5, 0), 1e-5, math.Inf(1), math.NaN()}
	for _, d := range values {
		m.D[1] = d
		pos, err := m.DIsNotNegative(1)
		lpStaticMust(t, err)
		neg, err := m.DIsNotPositive(1)
		lpStaticMust(t, err)
		if pos != (d > -1e-5) || neg != (d < 1e-5) {
			t.Fatal(d, pos, neg)
		}
		for _, typ := range []ColumnType{ColumnBoxed, ColumnFixed, ColumnLowBound, ColumnFree} {
			(*m.ColumnTypes)[1] = typ
			(*m.LowBounds)[1], (*m.UpperBounds)[1] = -2, 3
			for _, x := range []float64{-2, 0, 3, math.NaN()} {
				(*m.X)[1] = x
				want := (x == -2 || math.IsNaN(x)) && pos
				if typ == ColumnBoxed || typ == ColumnFixed {
					want = want || (x == 3 || math.IsNaN(x)) && neg
				}
				if typ == ColumnFree {
					want = d == 0
				}
				got, err := m.ColumnIsDualFeasible(1)
				lpStaticMust(t, err)
				if got != want {
					t.Fatal(typ, x, d, got, want)
				}
			}
		}
		(*m.ColumnTypes)[1] = ColumnBoxed
		lpStaticMust(t, m.SetNonBasicXToCorrectBounds())
		want := -2.0
		if d < 0 {
			want = 3
		}
		if (*m.X)[1] != want {
			t.Fatal(d, *m.X)
		}
	}
	// Low-first short circuit must not read an absent upper bound.
	(*m.ColumnTypes)[1], (*m.X)[1], m.D[1] = ColumnBoxed, -2, 0
	*m.UpperBounds = nil
	ok, err := m.ColumnIsDualFeasible(1)
	lpStaticMust(t, err)
	if !ok {
		t.Fatal("low endpoint")
	}
	m.D[1] = -1
	if _, err := m.ColumnIsDualFeasible(1); err != ErrBounds {
		t.Fatal(err)
	}
}

func TestLPFloatDualPricingIndependentScores(t *testing.T) {
	m := lpDualInitialFixture(t, 3, 3)
	*m.Basis = []uint32{2, 0, 1}
	*m.Heading = []int32{1, 2, 0}
	for world := 0; world < 160; world++ {
		for j := 0; j < 3; j++ {
			typ := ColumnType((world + j) % 5)
			(*m.ColumnTypes)[j] = typ
			(*m.LowBounds)[j], (*m.UpperBounds)[j] = -4, 6
			(*m.X)[j] = float64((world+j)%25 - 10)
			m.Betas[(*m.Heading)[j]] = float64(j+1) / 4
		}
		for i, p := range *m.Basis {
			x, typ := (*m.X)[p], (*m.ColumnTypes)[p]
			gap := 0.0
			if typ != ColumnFree && typ != ColumnUpperBound && x < -4 {
				gap = -4 - x
			}
			if gap == 0 && typ != ColumnFree && typ != ColumnLowBound && x > 6 {
				gap = x - 6
			}
			want := gap * gap / (float64(p+1) / 4)
			got, err := m.PricingForRow(uint32(i))
			lpStaticMust(t, err)
			if got != want {
				t.Fatal(world, i, got, want)
			}
		}
	}
	// Strict primal tolerance boundaries, at a zero bound.
	*m.Basis = []uint32{0, 1, 2}
	*m.Heading = []int32{0, 1, 2}
	(*m.ColumnTypes)[0] = ColumnBoxed
	(*m.LowBounds)[0], (*m.UpperBounds)[0] = 0, 0
	m.Betas[0] = 1
	eps := m.Settings.PrimalFeasibilityTolerance
	for _, x := range []float64{-eps, eps, math.Nextafter(-eps, math.Inf(-1)), math.Nextafter(eps, math.Inf(1)), math.NaN()} {
		(*m.X)[0] = x
		want := 0.0
		if x < -eps || x > eps {
			want = x * x
		}
		got, err := m.PricingForRow(0)
		lpStaticMust(t, err)
		if got != want {
			t.Fatal(x, got, want)
		}
	}
	(*m.X)[0] = 2
	for _, beta := range []float64{0, math.Copysign(0, -1), -2, math.Inf(1), math.NaN()} {
		m.Betas[0] = beta
		got, err := m.GetEdgeSteepnessForUpperBound(0)
		lpStaticMust(t, err)
		want := 4 / beta
		if !(math.IsNaN(got) && math.IsNaN(want)) && (got != want || math.Signbit(got) != math.Signbit(want)) {
			t.Fatal(beta, got, want)
		}
	}
	// Malformed boxed interval: the source gives the lower violation priority.
	(*m.LowBounds)[0], (*m.UpperBounds)[0], (*m.X)[0], m.Betas[0] = 5, -3, 0, 2
	got, err := m.PricingForRow(0)
	lpStaticMust(t, err)
	if got != 12.5 {
		t.Fatal(got)
	}
}

func TestLPFloatDualInitialPublicationAndDiagnostics(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 3)
	*m.NBasis = []uint32{1, 2}
	(*m.ColumnTypes)[1], (*m.ColumnTypes)[2] = ColumnLowBound, ColumnBoxed
	(*m.LowBounds)[1], (*m.X)[1], (*m.X)[2], m.D[1] = 4, 99, 88, -1
	err := m.SetNonBasicXToCorrectBounds()
	if sourceDebugEnabled {
		if err != ErrAssertion || (*m.X)[1] != 4 || (*m.X)[2] != 88 {
			t.Fatal(err, *m.X)
		}
	} else {
		lpStaticMust(t, err)
	}
	var out bytes.Buffer
	m.Settings.DebugOut = &out
	(*m.ColumnTypes)[1], m.D[1] = ColumnUpperBound, 0
	ok, err := m.ColumnIsDualFeasible(1)
	if out.String() != "upper_bound type should be switched to low_bound\n" {
		t.Fatal(out.String())
	}
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || !ok {
		t.Fatal(ok, err)
	}
	wantErr := errors.New("dual output")
	m.Settings.DebugOut = lpPresolveFailWriter{wantErr}
	if _, err := m.ColumnIsDualFeasible(1); err != wantErr {
		t.Fatal(err)
	}
	m.Settings.DebugOut = &out
	out.Reset()
	(*m.ColumnTypes)[1] = ColumnType(999)
	_, err = m.ColumnIsDualFeasible(1)
	if sourceDebugEnabled {
		if err != ErrAssertion || out.String() != "column = 1\n" {
			t.Fatal(err, out.String())
		}
	} else if err != nil || out.String() != "column = 1\nunexpected column type = unknown\n" {
		t.Fatal(err, out.String())
	}
	// Real closed-factor failure occurs after endpoint assignment, before betas.
	(*m.ColumnTypes)[1], m.D[1], (*m.LowBounds)[1] = ColumnLowBound, 0, 7
	*m.NBasis = []uint32{1}
	m.Betas[0] = 42
	m.Factorization.Close()
	if err := m.StartWithInitialBasisAndMakeItDualFeasible(); err == nil || (*m.X)[1] != 7 || m.Betas[0] != 42 {
		t.Fatal(err, *m.X, m.Betas)
	}
}

func TestLPFloatDualDoneSourceLimits(t *testing.T) {
	m := lpDualInitialFixture(t, 0, 0)
	m.Settings.MaxTotalIterations = 3
	for _, status := range []Status{StatusOptimal, StatusUnknown, StatusUnstable, StatusFloatingPointError, StatusCancelled, StatusIterationsExhausted} {
		for _, n := range []uint32{0, 3, 4, math.MaxUint32} {
			m.Status, m.TotalIterations = status, n
			want := status == StatusOptimal || n > 3
			if m.Done() != want {
				t.Fatal(status, n)
			}
			end := status
			if status != StatusOptimal && n > 3 {
				end = StatusIterationsExhausted
			}
			if m.Status != end {
				t.Fatal(m.Status, end)
			}
		}
	}
}

func TestLPFloatDualInitialReachedGuards(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	check := func(err error) {
		t.Helper()
		if err != ErrBounds {
			t.Fatal(err)
		}
	}
	_, err := m.DIsNotNegative(2)
	check(err)
	_, err = m.DIsNotPositive(2)
	check(err)
	_, err = m.ColumnIsDualFeasible(2)
	check(err)
	*m.NBasis = []uint32{2}
	check(m.SetNonBasicXToCorrectBounds())
	*m.NBasis = []uint32{1}
	(*m.ColumnTypes)[1] = ColumnBoxed
	d := m.D
	m.D = nil
	check(m.SetNonBasicXToCorrectBounds())
	m.D = d
	low := *m.LowBounds
	*m.LowBounds = nil
	check(m.SetNonBasicXToCorrectBounds())
	_, err = m.ColumnIsDualFeasible(1)
	check(err)
	*m.LowBounds = low
	x := *m.X
	*m.X = nil
	check(m.SetNonBasicXToCorrectBounds())
	_, err = m.ColumnIsDualFeasible(1)
	check(err)
	*m.X = x
	(*m.ColumnTypes)[1] = ColumnUpperBound
	if sourceDebugEnabled {
		if err := m.SetNonBasicXToCorrectBounds(); err != ErrAssertion {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, m.SetNonBasicXToCorrectBounds())
	}
	(*m.ColumnTypes)[1] = ColumnFree
	m.D = nil
	_, err = m.ColumnIsDualFeasible(1)
	check(err)
	m.D = d
	(*m.ColumnTypes)[1] = ColumnType(999)
	m.Settings.DebugOut = nil
	_, err = m.ColumnIsDualFeasible(1)
	if err != sourceAssertionFailure() {
		t.Fatal(err)
	}
	for _, allowed := range []int{0, 1} {
		want := errors.New("dual diagnostic prefix")
		m.Settings.DebugOut = &lpValidationWriter{allowed: allowed, err: want}
		_, err = m.ColumnIsDualFeasible(1)
		if allowed == 1 && sourceDebugEnabled {
			if err != ErrAssertion {
				t.Fatal(err)
			}
		} else if err != want {
			t.Fatal(err)
		}
	}
	m.Settings.DebugOut = nil
	// Debug validation reads D only after publishing the lower endpoint.
	(*m.ColumnTypes)[1] = ColumnLowBound
	m.D = nil
	err = m.StartWithInitialBasisAndMakeItDualFeasible()
	if sourceDebugEnabled {
		check(err)
	} else {
		lpStaticMust(t, err)
	}
	m.D = d
	_, err = m.PricingForRow(1)
	check(err)
	*m.Basis = []uint32{2}
	_, err = m.PricingForRow(0)
	check(err)
	*m.Basis = []uint32{0}
	(*m.ColumnTypes)[0] = ColumnType(999)
	_, err = m.PricingForRow(0)
	if err != sourceAssertionFailure() {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnFree
	m.D[0] = 1
	_, err = m.PricingForRow(0)
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, err)
	}
	m.D = nil
	_, err = m.PricingForRow(0)
	if sourceDebugEnabled {
		check(err)
	} else {
		lpStaticMust(t, err)
	}
	m.D = d
	(*m.ColumnTypes)[0] = ColumnBoxed
	*m.X = nil
	_, err = m.PricingForRow(0)
	check(err)
	*m.X = x
	*m.LowBounds = nil
	_, err = m.PricingForRow(0)
	check(err)
	_, err = m.GetEdgeSteepnessForLowBound(0)
	check(err)
	*m.LowBounds = low
	upper := *m.UpperBounds
	*m.UpperBounds = nil
	_, err = m.PricingForRow(0)
	check(err)
	*m.UpperBounds = upper
	_, err = m.GetEdgeSteepnessForLowBound(2)
	check(err)
	(*m.Heading)[0] = -1
	_, err = m.GetEdgeSteepnessForLowBound(0)
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else {
		check(err)
	}
	(*m.Heading)[0] = 0
	m.Betas = nil
	_, err = m.GetEdgeSteepnessForLowBound(0)
	check(err)
}
