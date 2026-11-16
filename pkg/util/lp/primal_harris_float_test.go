package lp

import (
	"math"
	"math/rand"
	"testing"
)

func TestLPFloatPrimalHarrisIndependentBounds(t *testing.T) {
	r := rand.New(rand.NewSource(90211))
	m := lpFloatPrimalCore(t, 8, 8)
	m.Settings.PrimalFeasibilityTolerance = 0
	m.Settings.HarrisFeasibilityTolerance = 0.003
	m.RestoreHarrisEps()
	for world := 0; world < 500; world++ {
		m.SignOfEnteringDelta = 1
		m.Ed.Index = nil
		want := math.Inf(1)
		ratios := make([]float64, 8)
		for i := 0; i < 8; i++ {
			low := float64(r.Intn(30) - 20)
			upper := low + float64(1+r.Intn(20))
			x := low + float64((upper-low)*r.Float64())
			(*m.LowBounds)[i] = low
			(*m.UpperBounds)[i] = upper
			(*m.X)[i] = x
			typ := ColumnType(r.Intn(4))
			(*m.ColumnTypes)[i] = typ
			ed := float64(1+r.Intn(20)) / 4
			if r.Intn(2) == 0 {
				ed = -ed
			}
			m.Ed.Data[i] = ed
			m.Ed.Index = append(m.Ed.Index, uint32(i))
			// Independently intersect the movement ray with each allowed half-space.
			ratios[i] = math.Inf(1)
			if ed > 0 && (typ == ColumnLowBound || typ == ColumnBoxed) {
				ratios[i] = (x - low) / ed
				relaxed := (x - low + (1+math.Abs(low)/10)*0.003/3) / ed
				if relaxed < want {
					want = relaxed
				}
			}
			if ed < 0 && (typ == ColumnUpperBound || typ == ColumnBoxed) {
				ratios[i] = (upper - x) / (-ed)
				relaxed := (upper - x + (1+math.Abs(upper)/10)*0.003/3) / (-ed)
				if relaxed < want {
					want = relaxed
				}
			}
		}
		theta := 937.0
		unlimited, err := m.GetHarrisTheta(&theta)
		if err != nil || unlimited != math.IsInf(want, 1) || !unlimited && math.Abs(theta-want) > 1e-12 {
			t.Fatalf("world %d theta %g want %g unlimited %v err %v", world, theta, want, unlimited, err)
		}
		if unlimited {
			continue
		}
		m.Settings.RandomSeed(uint32(world))
		state := uint32(world)*214013 + 2531011
		start := int(((state >> 16) & 32767) % 8)
		expected := -1
		maxPivot := 0.0
		for k := 0; k < 8; k++ {
			i := (start + k) % 8
			if ratios[i] <= want && (expected < 0 || math.Abs(m.Ed.Data[i]) > maxPivot) {
				expected = i
				maxPivot = math.Abs(m.Ed.Data[i])
			}
		}
		step := -999.0
		leaving, err := m.FindLeavingOnHarrisTheta(theta, &step)
		if err != nil || int(leaving) != expected || step != ratios[expected] || m.ConvertedHarrisEpsilon != 0.003 {
			t.Fatalf("world %d leaving %d want %d step %g err %v", world, leaving, expected, step, err)
		}
	}
}

func TestLPFloatPrimalHarrisInfeasibleCrossings(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 1)
	m.Settings.PrimalFeasibilityTolerance = 0
	m.ConvertedHarrisEpsilon = 0.3
	lpStaticMust(t, m.InfSet.Insert(0))
	(*m.LowBounds)[0] = 2
	(*m.UpperBounds)[0] = 20
	for _, typ := range []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed} {
		(*m.ColumnTypes)[0] = typ
		for _, x := range []float64{-4, 2, 11, 20, 28} {
			(*m.X)[0] = x
			for _, d := range []float64{-2, 2} {
				expected := math.Inf(1)
				// In feasibility search, stop at the next relevant boundary in the
				// movement direction. Boxed lower crossings use the source upper scale.
				if d > 0 {
					if (typ == ColumnLowBound || typ == ColumnBoxed || typ == ColumnFixed) && x < 2 {
						eps := 0.12
						if typ != ColumnLowBound {
							eps = 0.3
						}
						expected = (2 - x + eps) / d
					} else if typ == ColumnUpperBound || typ == ColumnBoxed || typ == ColumnFixed {
						if x < 20 {
							expected = (20 - x + 0.3) / d
						} else if x == 20 {
							expected = 0
						}
					}
				} else {
					if (typ == ColumnUpperBound || typ == ColumnBoxed || typ == ColumnFixed) && x > 20 {
						expected = (20 - x - 0.3) / d
					} else if typ == ColumnLowBound || typ == ColumnBoxed || typ == ColumnFixed {
						if x > 2 {
							expected = (2 - x - 0.12) / d
						} else if x == 2 {
							expected = 0
						}
					}
				}
				theta := 777.0
				unlimited := true
				err := m.LimitThetaOnBasisColumn(0, d, &theta, &unlimited)
				if err != nil || unlimited != math.IsInf(expected, 1) || !unlimited && math.Abs(theta-expected) > 1e-12 {
					t.Fatalf("type %v x %g d %g theta %g expected %g unlimited %v err %v", typ, x, d, theta, expected, unlimited, err)
				}
			}
		}
	}
}

func TestLPFloatPrimalHarrisIEEEAndPrefixes(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 1)
	theta := math.NaN()
	unlimited := false
	m.LimitTheta(3, &theta, &unlimited)
	if theta != 3 {
		t.Fatal(theta)
	}
	m.LimitTheta(math.NaN(), &theta, &unlimited)
	if !math.IsNaN(theta) {
		t.Fatal(theta)
	}
	theta = 0
	m.LimitTheta(math.Copysign(0, -1), &theta, &unlimited)
	if !math.Signbit(theta) {
		t.Fatal("tie sign")
	}
	m.ZeroHarrisEps()
	if !math.IsNaN(m.HarrisEpsForBound(math.Inf(1))) {
		t.Fatal("infinite bound times zero")
	}
	(*m.ColumnTypes)[0] = ColumnFree
	theta = -2
	lpStaticMust(t, m.LimitThetaOnBasisColumn(0, 0, &theta, &unlimited))
	if theta != 0 {
		t.Fatal("free final clamp")
	}
	// A zero step stops before the invalid later sparse entry.
	(*m.ColumnTypes)[0] = ColumnLowBound
	(*m.X)[0] = 0
	(*m.LowBounds)[0] = 0
	m.SignOfEnteringDelta = 1
	m.Ed.Data[0] = 1
	m.Ed.Index = []uint32{0, 99}
	unlimited, err := m.GetHarrisTheta(&theta)
	if err != nil || unlimited || theta != 0 {
		t.Fatal(theta, unlimited, err)
	}
	// Empty selection consumes random state and leaves epsilon zero at the
	// typed guard corresponding to undefined source modulo-zero.
	m.Ed.Index = nil
	m.ConvertedHarrisEpsilon = 8
	m.Settings.RandomSeed(4)
	step := 91.0
	if _, err = m.FindLeavingOnHarrisTheta(5, &step); err != ErrBounds || step != 91 || m.ConvertedHarrisEpsilon != 0 {
		t.Fatal(err, step)
	}
	state := uint32(4)*214013 + 2531011
	state = state*214013 + 2531011
	if m.Settings.RandomNext() != ((state >> 16) & 32767) {
		t.Fatal("random prefix")
	}
	for _, typ := range []ColumnType{ColumnFree, ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed, 99} {
		(*m.ColumnTypes)[0] = typ
		(*m.X)[0] = 3
		(*m.LowBounds)[0] = 1
		(*m.UpperBounds)[0] = 8
		for _, sign := range []int32{-1, 0, 1} {
			m.SignOfEnteringDelta = sign
			step = 91
			flip, err := m.TryJumpToAnotherBoundOnEntering(0, 1, &step, false)
			active := typ == ColumnBoxed || typ == ColumnLowBound && sign < 0 || typ == ColumnUpperBound && sign > 0
			want := 91.0
			if active {
				want = 2
				if sign > 0 {
					want = 5
				}
			}
			if err != nil || flip || step != want {
				t.Fatal(typ, sign, flip, step, err)
			}
			flip, err = m.TryJumpToAnotherBoundOnEntering(0, 5, &step, true)
			if err != nil || flip != active {
				t.Fatal(typ, sign, flip, err)
			}
		}
	}
}

func TestLPFloatPrimalHarrisReachedGuards(t *testing.T) {
	for _, stage := range []int{0, 1, 2, 3, 4, 5} {
		m := lpFloatPrimalCore(t, 1, 1)
		(*m.ColumnTypes)[0] = ColumnBoxed
		m.SignOfEnteringDelta = 1
		(*m.X)[0] = 3
		(*m.LowBounds)[0] = 1
		(*m.UpperBounds)[0] = 5
		m.Ed.Data[0] = 1
		m.Ed.Index = []uint32{0}
		switch stage {
		case 0:
			*m.ColumnTypes = nil
		case 1:
			*m.X = nil
		case 2:
			*m.LowBounds = nil
		case 3:
			m.Ed.Index = []uint32{3}
		case 4:
			*m.Basis = nil
		case 5:
			lpStaticMust(t, m.InfSet.Insert(0))
			*m.UpperBounds = nil
		}
		theta := 19.0
		if _, err := m.GetHarrisTheta(&theta); err != ErrBounds {
			t.Fatalf("stage %d: %v", stage, err)
		}
		step := 21.0
		if _, err := m.FindLeavingOnHarrisTheta(20, &step); err != ErrBounds || step != 21 || m.ConvertedHarrisEpsilon != 0 {
			t.Fatalf("stage %d: %v step %g", stage, err, step)
		}
	}
	m := lpFloatPrimalCore(t, 1, 1)
	theta := 17.0
	unlimited := true
	(*m.ColumnTypes)[0] = ColumnType(99)
	err := m.LimitThetaOnBasisColumn(0, 1, &theta, &unlimited)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnLowBound
	lpStaticMust(t, m.InfSet.Insert(0))
	err = m.LimitThetaOnBasisColumn(0, 0, &theta, &unlimited)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	// Inactive pivots are skipped before basis access in both scans.
	m.Ed.Index = []uint32{0}
	m.Ed.Data[0] = 0
	*m.Basis = nil
	if u, err := m.GetHarrisTheta(&theta); err != nil || !u {
		t.Fatal(u, err)
	}
	step := 13.0
	if leaving, err := m.FindLeavingOnHarrisTheta(4, &step); err != nil || leaving != -1 || step != 13 {
		t.Fatal(leaving, err, step)
	}
	// Missing low storage is never read when an infeasible boxed variable
	// above its upper bound moves down toward that upper bound.
	(*m.ColumnTypes)[0] = ColumnBoxed
	(*m.X)[0] = 9
	(*m.UpperBounds)[0] = 5
	*m.LowBounds = nil
	unlimited = true
	lpStaticMust(t, m.LimitThetaOnBasisColumn(0, -2, &theta, &unlimited))
	if unlimited || theta <= 0 {
		t.Fatal(theta, unlimited)
	}
	unlimited = true
	if err := m.LimitThetaOnBasisColumn(0, 2, &theta, &unlimited); err != ErrBounds {
		t.Fatal(err)
	}
	(*m.X)[0] = 3
	unlimited = true
	if err := m.LimitThetaOnBasisColumn(0, -2, &theta, &unlimited); err != ErrBounds {
		t.Fatal(err)
	}
	*m.LowBounds = []float64{1}
	*m.UpperBounds = nil
	unlimited = true
	if err := m.LimitThetaOnBasisColumn(0, 2, &theta, &unlimited); err != ErrBounds {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = ColumnUpperBound
	m.SignOfEnteringDelta = 1
	if _, err := m.TryJumpToAnotherBoundOnEntering(0, 2, &step, true); err != ErrBounds {
		t.Fatal(err)
	}
	*m.UpperBounds = []float64{1}
	(*m.X)[0] = 3
	flip, err := m.TryJumpToAnotherBoundOnEntering(0, 2, &step, true)
	if step != -2 || sourceDebugEnabled && (flip || err != ErrAssertion) || !sourceDebugEnabled && (!flip || err != nil) {
		t.Fatal(flip, step, err)
	}
	*m.ColumnTypes = nil
	if _, err := m.TryJumpToAnotherBoundOnEntering(0, 2, &step, true); err != ErrBounds {
		t.Fatal(err)
	}
}
