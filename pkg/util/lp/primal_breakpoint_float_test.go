package lp

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestLPFloatPrimalBreakpointIndependentCrossings(t *testing.T) {
	rng := rand.New(rand.NewSource(124513))
	m := lpFloatPrimalCore(t, 12, 13)
	m.BreakpointQueue.Resize(26)
	for world := 0; world < 600; world++ {
		sign := int32(1)
		if world%2 != 0 {
			sign = -1
		}
		m.SignOfEnteringDelta = sign
		m.Ed.Index = nil
		var expected []FloatBreakpoint
		for i := 0; i < 12; i++ {
			m.Ed.Index = append(m.Ed.Index, uint32(i))
			typ := ColumnType(rng.Intn(5))
			(*m.ColumnTypes)[i] = typ
			low := float64(rng.Intn(15) - 8)
			upper := low + float64(1+rng.Intn(12))
			if typ == ColumnFixed {
				upper = low
			}
			(*m.LowBounds)[i] = low
			(*m.UpperBounds)[i] = upper
			x := low + float64(rng.Intn(24)-6)
			if world%3 == 0 {
				x = low
			}
			if world%7 == 0 {
				x = upper
			}
			(*m.X)[i] = x
			d := float64(rng.Intn(13) - 6)
			m.Ed.Data[i] = d
			if d == 0 {
				continue
			}
			// Solve x - ed*delta = bound along the chosen movement ray.
			for _, b := range []struct {
				v    float64
				typ  BreakpointType
				live bool
			}{
				{low, BreakpointLow, typ == ColumnLowBound || typ == ColumnBoxed},
				{upper, BreakpointUpper, typ == ColumnUpperBound || typ == ColumnBoxed},
				{low, BreakpointFixed, typ == ColumnFixed},
			} {
				if !b.live {
					continue
				}
				distance := (x - b.v) / (d * float64(sign))
				keep := distance > 0
				if x == b.v {
					keep = b.typ == BreakpointFixed || b.typ == BreakpointLow && d*float64(sign) > 0 || b.typ == BreakpointUpper && d*float64(sign) < 0
				}
				if keep {
					delta := distance * float64(sign)
					if distance == 0 {
						delta = 0
					}
					expected = append(expected, FloatBreakpoint{uint32(i), b.typ, delta})
				}
			}
		}
		(*m.ColumnTypes)[12] = ColumnFree
		if world%4 == 0 {
			(*m.ColumnTypes)[12] = ColumnBoxed
			(*m.LowBounds)[12] = -3
			(*m.UpperBounds)[12] = 9
			typ := BreakpointUpper
			if sign < 0 {
				typ = BreakpointLow
			}
			expected = append(expected, FloatBreakpoint{12, typ, 12 * float64(sign)})
		}
		lpStaticMust(t, m.FillBreakpointsArray(12))
		if len(expected) != len(m.Breakpoints) {
			t.Fatalf("world %d: %v != %v", world, m.Breakpoints, expected)
		}
		for i, b := range expected {
			if b != m.Breakpoints[i] {
				t.Fatalf("world %d entry %d: %v != %v", world, i, m.Breakpoints[i], b)
			}
		}
		if len(expected) == 0 {
			continue
		}
		minimum := math.Inf(1)
		for _, b := range expected {
			if math.Abs(b.Delta) < minimum {
				minimum = math.Abs(b.Delta)
			}
		}
		// Double always chooses the nearest breakpoint even when the initial
		// cost slope remains strongly negative after that crossing.
		m.D[12] = -1e6 * float64(sign)
		step := 999.0
		leaving, err := m.AdvanceOnSortedBreakpoints(12, &step)
		if err != nil || math.Abs(step) != minimum || m.BreakpointQueue.Size() != uint32(len(expected)-1) {
			t.Fatalf("world %d leave %d step %g min %g err %v", world, leaving, step, minimum, err)
		}
		found := false
		for _, b := range expected {
			if int32(b.Column) == leaving && b.Delta == step {
				found = true
			}
		}
		if !found {
			t.Fatal("not an actual crossing")
		}
		if step != 0 && (step > 0) != (sign > 0) {
			t.Fatal("signed delta")
		}
	}
}

func TestLPFloatPrimalBreakpointIndependentSlope(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	for _, sign := range []int32{-1, 1} {
		m.SignOfEnteringDelta = sign
		for _, ed := range []float64{-7, -0.5, 0, 0.25, 9} {
			m.Ed.Data[0] = ed
			for _, typ := range []BreakpointType{BreakpointLow, BreakpointUpper, BreakpointFixed} {
				for _, delta := range []float64{0, 2} {
					// Independent left/right slopes of the hinge cost on the
					// movement ray: one hinge per bound, two for a fixed crossing.
					hinge := func(s float64) float64 { return math.Max(0, math.Abs(ed)*s) }
					jump := hinge(1) + hinge(-1)
					if typ == BreakpointFixed && delta != 0 {
						jump *= 2
					}
					want := 3 + float64(float64(sign)*jump)
					slope := 3.0
					lpStaticMust(t, m.ChangeSlopeOnBreakpoint(1, FloatBreakpoint{0, typ, delta}, &slope))
					if slope != want {
						t.Fatal(sign, ed, typ, delta, slope, want)
					}
				}
			}
		}
		slope := 3.0
		lpStaticMust(t, m.ChangeSlopeOnBreakpoint(1, FloatBreakpoint{1, BreakpointUpper, 2}, &slope))
		if slope != 3+float64(sign) {
			t.Fatal(slope)
		}
	}
}

func TestLPFloatPrimalLeavingPathIntegration(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	m.BreakpointQueue.Resize(2)
	m.SignOfEnteringDelta = 1
	(*m.ColumnTypes)[0] = ColumnLowBound
	(*m.LowBounds)[0] = 0
	(*m.X)[0] = 6
	m.Ed.Index = []uint32{0}
	m.Ed.Data[0] = 2
	(*m.ColumnTypes)[1] = ColumnFree
	step := 99.0
	leaving, err := m.FindLeavingAndT(1, &step)
	if err != nil || leaving != 0 || step != 3 {
		t.Fatal(leaving, step, err)
	}
	(*m.ColumnTypes)[1] = ColumnBoxed
	(*m.LowBounds)[1] = 0
	(*m.UpperBounds)[1] = 2
	(*m.X)[1] = 0
	leaving, err = m.FindLeavingAndT(1, &step)
	if err != nil || leaving != 1 || step != 2 {
		t.Fatal(leaving, step, err)
	}
	(*m.ColumnTypes)[1] = ColumnFree
	m.Ed.Index = nil
	step = 99
	leaving, err = m.FindLeavingAndT(1, &step)
	if err != nil || leaving != -1 || step != 99 {
		t.Fatal(leaving, step, err)
	}
	// Cached infeasibility plus the flag dispatches to signed breakpoints.
	lpStaticMust(t, m.InfSet.Insert(0))
	m.Settings.UseBreakpointsInFeasibilitySearch = true
	m.Ed.Index = []uint32{0}
	m.Ed.Data[0] = -2
	(*m.X)[0] = -6
	leaving, err = m.FindLeavingAndT(1, &step)
	if err != nil || leaving != 0 || step != 3 {
		t.Fatal(leaving, step, err)
	}
	// Equal priorities preserve the source queue's insertion/root choice.
	lpStaticMust(t, m.ClearBreakpoints())
	lpStaticMust(t, m.AddBreakpoint(0, 3, BreakpointLow))
	lpStaticMust(t, m.AddBreakpoint(1, 3, BreakpointUpper))
	if !reflect.DeepEqual(m.Breakpoints, []FloatBreakpoint{{0, BreakpointLow, 3}, {1, BreakpointUpper, 3}}) {
		t.Fatal(m.Breakpoints)
	}
	leaving, err = m.AdvanceOnSortedBreakpoints(1, &step)
	if err != nil || leaving != 0 {
		t.Fatal(leaving, err)
	}
}

func TestLPFloatPrimalBreakpointPublicationAndGuards(t *testing.T) {
	for stage := 0; stage < 7; stage++ {
		m := lpFloatPrimalCore(t, 1, 2)
		m.BreakpointQueue.Resize(4)
		m.SignOfEnteringDelta = 1
		m.Ed.Index = []uint32{0}
		m.Ed.Data[0] = 1
		(*m.ColumnTypes)[0] = ColumnBoxed
		(*m.LowBounds)[0] = 0
		(*m.UpperBounds)[0] = 4
		(*m.X)[0] = 2
		(*m.ColumnTypes)[1] = ColumnBoxed
		switch stage {
		case 0:
			m.Ed.Data = nil
		case 1:
			*m.Basis = nil
		case 2:
			*m.X = nil
		case 3:
			*m.LowBounds = nil
		case 4:
			*m.UpperBounds = nil
		case 5:
			*m.ColumnTypes = (*m.ColumnTypes)[:1]
		case 6:
			*m.LowBounds = (*m.LowBounds)[:1]
		}
		lpStaticMust(t, m.AddBreakpoint(1, 9, BreakpointUpper))
		step := 71.0
		_, err := m.FindLeavingAndTWithBreakpoints(1, &step)
		if err != ErrBounds || step != 71 {
			t.Fatalf("stage %d err %v step %g", stage, err, step)
		}
		want := 0
		if stage >= 4 {
			want = 1
		}
		if len(m.Breakpoints) != want || int(m.BreakpointQueue.Size()) != want {
			t.Fatalf("stage %d retained %v size %d", stage, m.Breakpoints, m.BreakpointQueue.Size())
		}
	}
	m := lpFloatPrimalCore(t, 1, 2)
	m.BreakpointQueue.Resize(4)
	m.SignOfEnteringDelta = 1
	step := 55.0
	errWant := ErrBounds
	if sourceDebugEnabled {
		errWant = ErrAssertion
	}
	if _, err := m.AdvanceOnSortedBreakpoints(1, &step); err != errWant || step != 55 {
		t.Fatal(err, step)
	}
	if _, err := m.AdvanceOnSortedBreakpoints(99, &step); err != ErrBounds {
		t.Fatal(err)
	}
	// A valid queue mutation survives a subsequent malformed heading error.
	lpStaticMust(t, m.AddBreakpoint(1, 2, BreakpointLow))
	(*m.Heading)[1] = -1
	if _, err := m.AdvanceOnSortedBreakpoints(0, &step); err != errWant || step != 55 || !m.BreakpointQueue.IsEmpty() {
		t.Fatal(err, step)
	}
	(*m.Heading)[1] = 7
	lpStaticMust(t, m.AddBreakpoint(1, 2, BreakpointLow))
	if _, err := m.AdvanceOnSortedBreakpoints(0, &step); err != ErrBounds {
		t.Fatal(err)
	}
	lpStaticMust(t, m.AddBreakpoint(1, 2, BreakpointLow))
	m.Breakpoints = nil
	if _, err := m.AdvanceOnSortedBreakpoints(0, &step); err != ErrBounds {
		t.Fatal(err)
	}
	slope := 3.0
	if err := m.ChangeSlopeOnBreakpoint(0, FloatBreakpoint{99, BreakpointLow, 1}, &slope); err != ErrBounds || slope != 3 {
		t.Fatal(err, slope)
	}
	err := m.ChangeSlopeOnBreakpoint(0, FloatBreakpoint{0, BreakpointFixed, 0}, &slope)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && (err != nil || slope != 4) {
		t.Fatal(err, slope)
	}
	m.Ed.Data[0] = 2
	err = m.ChangeSlopeOnBreakpoint(1, FloatBreakpoint{0, 99, 1}, &slope)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	(*m.ColumnTypes)[0] = 99
	err = m.TryAddBreakpointInRow(0)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	if err := m.TryAddBreakpointInRow(99); err != errWant {
		t.Fatal(err)
	}
	// NaN distances cannot pass the strict same-sign gate; signed zero is
	// published as positive zero for a crossing already at a fixed bound.
	lpStaticMust(t, m.ClearBreakpoints())
	lpStaticMust(t, m.TryAddBreakpoint(0, math.NaN(), 1, BreakpointLow, 0))
	lpStaticMust(t, m.TryAddBreakpoint(0, 1, math.NaN(), BreakpointUpper, 0))
	if len(m.Breakpoints) != 0 {
		t.Fatal(m.Breakpoints)
	}
	lpStaticMust(t, m.TryAddBreakpoint(0, math.Copysign(0, -1), 0, BreakpointFixed, 0))
	if len(m.Breakpoints) != 1 || math.Signbit(m.Breakpoints[0].Delta) {
		t.Fatal(m.Breakpoints)
	}
	// A newly constructed source queue has zero extent. Its object-zero
	// enqueue fails after the breakpoint append, preserving that prefix.
	n := lpFloatPrimalCore(t, 0, 0)
	if err := n.AddBreakpoint(0, 3, BreakpointLow); err != ErrBounds || len(n.Breakpoints) != 1 {
		t.Fatal(err, n.Breakpoints)
	}
}

func TestLPFloatPrimalLeavingReachedErrors(t *testing.T) {
	m := lpFloatPrimalCore(t, 1, 2)
	m.SignOfEnteringDelta = 1
	step := 55.0
	if _, err := m.FindLeavingAndT(99, &step); err != ErrBounds || step != 55 {
		t.Fatal(err, step)
	}
	m.Ed.Index = []uint32{7}
	if _, err := m.FindLeavingAndT(1, &step); err != ErrBounds || step != 55 {
		t.Fatal(err, step)
	}
	m.Ed.Index = nil
	(*m.ColumnTypes)[1] = ColumnUpperBound
	(*m.X)[1] = 2
	(*m.UpperBounds)[1] = 1
	leaving, err := m.FindLeavingAndT(1, &step)
	if sourceDebugEnabled && (err != ErrAssertion || leaving != -1) || !sourceDebugEnabled && (err != nil || leaving != 1) || step != -1 {
		t.Fatal(leaving, step, err)
	}
	// NaN admitted by a non-small pivot propagates through the Harris limit.
	// The source boxed negative-helper sign assertion fires in debug.
	(*m.ColumnTypes)[0] = ColumnBoxed
	m.Ed.Index = []uint32{0}
	m.Ed.Data[0] = math.NaN()
	(*m.ColumnTypes)[1] = ColumnFree
	step = 55
	leaving, err = m.FindLeavingAndT(1, &step)
	if sourceDebugEnabled {
		// The feasible negative helper's earlier strict sign assertion fires.
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || leaving != -1 || step != 55 {
		t.Fatal(leaving, step, err)
	}
	// Infinite bounds with finite movement give a NaN theta at zero Harris
	// epsilon, without violating the helper's movement-sign assertion.
	m.Ed.Data[0] = 1
	(*m.LowBounds)[0] = math.Inf(-1)
	m.ZeroHarrisEps()
	leaving, err = m.FindLeavingAndT(1, &step)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && (err != nil || leaving != -1) {
		t.Fatal(leaving, err)
	}
}
