package lp

import (
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func TestLPFloatDualRatioIndependentGroups(t *testing.T) {
	rng := rand.New(rand.NewSource(33171))
	type candidate struct {
		j                 uint32
		time, pivot, loss float64
		boxed             bool
	}
	for world := 0; world < 240; world++ {
		n := 3 + rng.Intn(10)
		m := lpDualInitialFixture(t, 1, uint32(n+1))
		m.P = 0
		m.R = 0
		m.HarrisTolerance = 0
		sign := 1.0
		if world%2 == 0 {
			sign = -1
		}
		(*m.ColumnTypes)[0] = ColumnBoxed
		(*m.LowBounds)[0], (*m.UpperBounds)[0] = 0, 10
		(*m.X)[0] = -3
		if sign > 0 {
			(*m.X)[0] = 13
		}
		m.Delta = sign * float64(1+rng.Intn(100)) / 4
		initial := m.Delta
		candidates := make([]candidate, 0, n)
		for j := 1; j <= n; j++ {
			boxed := rng.Intn(4) != 0
			(*m.ColumnTypes)[j] = ColumnLowBound
			if boxed {
				(*m.ColumnTypes)[j] = ColumnBoxed
			}
			upper := boxed && rng.Intn(2) == 0
			(*m.LowBounds)[j], (*m.UpperBounds)[j] = 0, float64(1+rng.Intn(9))/4
			(*m.X)[j] = 0
			if upper {
				(*m.X)[j] = (*m.UpperBounds)[j]
			}
			a := float64(1+rng.Intn(8)) / 4
			time := float64(rng.Intn(9)) / 2
			m.PivotRow.Data[j] = sign * a
			m.D[j] = time * a
			if upper {
				m.PivotRow.Data[j] = -sign * a
				m.D[j] = -time * a
			}
			candidates = append(candidates, candidate{uint32(j), time, a, (*m.UpperBounds)[j] * a, boxed})
		}
		// The independent model sorts exact dyadic times into groups, then
		// consumes each whole boxed group while the displacement survives.
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].time != candidates[j].time {
				return candidates[i].time < candidates[j].time
			}
			return candidates[i].j < candidates[j].j
		})
		flipped := map[uint32]bool{}
		delta := initial
		var q uint32
		var tight []uint32
		for start := 0; start < len(candidates); {
			end := start + 1
			for end < len(candidates) && candidates[end].time == candidates[start].time {
				end++
			}
			group := candidates[start:end]
			best := group[0]
			boxed := true
			loss := 0.0
			for _, c := range group {
				if c.pivot > best.pivot {
					best = c
				}
				boxed = boxed && c.boxed
				loss += c.loss
			}
			q = best.j
			tight = nil
			for _, c := range group {
				if c.j != q {
					tight = append(tight, c.j)
				}
			}
			sort.Slice(tight, func(i, j int) bool { return tight[i] < tight[j] })
			remaining := math.Abs(delta) - loss
			if !boxed || remaining <= m.Settings.ZeroTolerance || end == len(candidates) {
				break
			}
			for _, c := range group {
				flipped[c.j] = true
			}
			delta = sign * remaining
			start = end
		}
		ok, err := m.RatioTest()
		lpStaticMust(t, err)
		if !ok || m.Q != int32(q) || m.Delta != delta || m.Status != StatusFeasible || !reflect.DeepEqual(m.FlippedBoxed, flipped) || !reflect.DeepEqual(dualOrderedKeys(m.TightSet), append([]uint32{}, tight...)) {
			t.Fatal(world, ok, m.Q, q, m.Delta, delta, m.FlippedBoxed, flipped, m.TightSet, tight)
		}
		if m.ThetaD != m.D[q]/m.PivotRow.Data[q] {
			t.Fatal(world, m.ThetaD)
		}
		position := PositionAtLowBound
		if (*m.X)[q] != 0 {
			position = PositionAtUpperBound
		}
		if m.EnteringBoundaryPosition != position {
			t.Fatal(m.EnteringBoundaryPosition, position)
		}
	}
}

func TestLPFloatDualHarrisIndependentWindows(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 7)
	m.HarrisTolerance = 0.125
	for world := 0; world < 120; world++ {
		m.BreakpointSet = map[uint32]bool{}
		want := math.Inf(1)
		times := map[uint32]float64{}
		for j := uint32(1); j < 7; j++ {
			m.BreakpointSet[j] = true
			(*m.ColumnTypes)[j] = ColumnBoxed
			(*m.LowBounds)[j], (*m.UpperBounds)[j] = 0, 10
			(*m.X)[j] = 0
			d := float64(float64((world+int(j))%13) / 8)
			a := float64(j) / 4
			m.D[j], m.PivotRow.Data[j] = d, a
			if (world+int(j))%2 == 0 {
				(*m.X)[j] = 10
				m.D[j] = -d
				m.PivotRow.Data[j] = -a
			}
			times[j] = d / a
			limit := (d + 0.125) / a
			if limit < want {
				want = limit
			}
		}
		got, err := m.CalculateHarrisDeltaOnBreakpointSet()
		lpStaticMust(t, err)
		if got != want {
			t.Fatal(world, got, want)
		}
		lpStaticMust(t, m.FillTightSetOnHarrisDelta(got))
		q := int32(-1)
		max := 0.0
		for j := uint32(1); j < 7; j++ {
			if m.TightSet[j] != (times[j] <= want) {
				t.Fatal(world, j, m.TightSet, times)
			}
			if times[j] <= want && float64(j)/4 > max {
				q = int32(j)
				max = float64(j) / 4
			}
		}
		lpStaticMust(t, m.FindQOnTightSet())
		if m.Q != q || m.TightSet[uint32(q)] {
			t.Fatal(m.Q, q, m.TightSet)
		}
	}
	// std::max/min keep a first-operand NaN. The first set element's
	// NaN Harris time remains the result; an empty tight set is then exposed.
	m.D[1] = math.NaN()
	delta, err := m.CalculateHarrisDeltaOnBreakpointSet()
	lpStaticMust(t, err)
	if !math.IsNaN(delta) {
		t.Fatal(delta)
	}
	lpStaticMust(t, m.FillTightSetOnHarrisDelta(delta))
	err = m.FindQOnTightSet()
	if m.Q != -1 || len(m.TightSet) != 0 {
		t.Fatal(m.Q, m.TightSet)
	}
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else {
		lpStaticMust(t, err)
	}
}

func TestLPFloatDualBreakpointSignsAndDeltaQuirks(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	m.P = 0
	m.R = 0
	for _, sign := range []int{-1, 0, 1} {
		m.SignOfAlphaR = sign
		for _, typ := range []ColumnType{ColumnLowBound, ColumnUpperBound, ColumnBoxed, ColumnFixed, ColumnFree, ColumnType(99)} {
			(*m.ColumnTypes)[1] = typ
			(*m.LowBounds)[1], (*m.UpperBounds)[1] = 0, 2
			for _, atUpper := range []bool{false, true} {
				(*m.X)[1] = 0
				if atUpper {
					(*m.X)[1] = 2
				}
				for _, a := range []float64{-2, -m.Settings.PivotTolerance, 0, m.Settings.PivotTolerance, 2, math.NaN()} {
					m.PivotRow.Data[1] = a
					got, err := m.CanBeBreakpoint(1)
					if sourceDebugEnabled && (typ == ColumnLowBound && atUpper || typ == ColumnUpperBound && !atUpper) && !m.Settings.FloatPivotSmall(a) {
						if err != ErrAssertion {
							t.Fatal(typ, atUpper, a, err)
						}
						continue
					}
					lpStaticMust(t, err)
					want := false
					if !m.Settings.FloatPivotSmall(a) {
						switch typ {
						case ColumnFree:
							want = true
						case ColumnLowBound:
							want = float64(sign)*a > 0
						case ColumnUpperBound:
							want = float64(sign)*a < 0
						case ColumnBoxed:
							want = (!atUpper) == (float64(sign)*a > 0)
						}
					}
					if got != want {
						t.Fatal(sign, typ, atUpper, a, got, want)
					}
				}
			}
		}
	}
	(*m.LowBounds)[0], (*m.UpperBounds)[0], m.Betas[0] = 0, 2, 4
	for _, typ := range []ColumnType{ColumnBoxed, ColumnLowBound, ColumnUpperBound, ColumnFixed} {
		(*m.ColumnTypes)[0] = typ
		(*m.X)[0] = -3
		if typ == ColumnUpperBound {
			(*m.X)[0] = 5
		}
		delta, err := m.GetDelta()
		lpStaticMust(t, err)
		want := -3.0
		if typ == ColumnFixed {
			want = -5
		}
		if typ == ColumnUpperBound {
			want = 2.25
		}
		if delta != want {
			t.Fatal(typ, delta, want)
		}
		sign, err := m.DefineSignOfAlphaR()
		lpStaticMust(t, err)
		wantSign := -1
		if typ == ColumnUpperBound {
			wantSign = 1
		}
		if sign != wantSign {
			t.Fatal(typ, sign)
		}
	}
	// Source get_delta uses squared edge steepness for upper-only columns.
	(*m.ColumnTypes)[0], (*m.X)[0] = ColumnBoxed, 5
	delta, err := m.GetDelta()
	lpStaticMust(t, err)
	if delta != 3 {
		t.Fatal(delta)
	}
	sign, err := m.DefineSignOfAlphaR()
	lpStaticMust(t, err)
	if sign != 1 {
		t.Fatal(sign)
	}
	for _, d := range []float64{-m.Settings.ZeroTolerance, m.Settings.ZeroTolerance, math.Nextafter(m.Settings.ZeroTolerance, math.Inf(1)), math.NaN(), math.Inf(1)} {
		for _, s := range []int{-1, 0, 1, 2} {
			want := s == 1 && d > m.Settings.ZeroTolerance || s == -1 && d < -m.Settings.ZeroTolerance
			if m.DeltaKeepsTheSign(s, d) != want {
				t.Fatal(s, d)
			}
		}
	}
}

func TestLPFloatDualFlipsIndependentEquations(t *testing.T) {
	for world := 0; world < 120; world++ {
		m := lpDualInitialFixture(t, 3, 8)
		m.P = 0
		m.ThetaD = float64(world-20) / 8
		m.FlippedBoxed = map[uint32]bool{3: true, 5: true, 7: true}
		old := make([]float64, 8)
		copy(old, *m.X)
		for j := 3; j < 8; j++ {
			(*m.ColumnTypes)[j] = ColumnBoxed
			(*m.LowBounds)[j], (*m.UpperBounds)[j] = -2, float64(j)
			(*m.X)[j] = -2
			if (world+j)%2 == 0 {
				(*m.X)[j] = float64(j)
			}
			old[j] = (*m.X)[j]
			m.PivotRow.Data[j] = float64(j-5) / 4
			m.D[j] = float64(j)
			for i := 0; i < 3; i++ {
				lpStaticMust(t, m.A.Set(uint32(i), uint32(j), float64((world+i+j)%7-3)/4))
			}
		}
		// Build B*x_B = b-A_N*x_N, then predict the replacement endpoint
		// solution independently for the identity basis.
		for i := 0; i < 3; i++ {
			(*m.B)[i] = float64(i+world) / 4
			x := (*m.B)[i]
			for j := 3; j < 8; j++ {
				x -= float64(float64((world+i+j)%7-3) / 4 * old[j])
			}
			(*m.X)[i] = x
		}
		lpStaticMust(t, m.UpdateDAndXB())
		for j := 3; j < 8; j++ {
			want := float64(j) - float64(m.ThetaD*float64(j-5)/4)
			if m.D[j] != want {
				t.Fatal(world, j, m.D[j], want)
			}
		}
		if m.D[0] != -m.ThetaD {
			t.Fatal(m.D)
		}
		lpStaticMust(t, m.ApplyFlips())
		for j := 3; j < 8; j++ {
			want := old[j]
			if m.FlippedBoxed[uint32(j)] {
				want = float64(j)
				if old[j] != -2 {
					want = -2
				}
			}
			if (*m.X)[j] != want {
				t.Fatal(world, j, *m.X)
			}
		}
		for i := 0; i < 3; i++ {
			lhs := (*m.X)[i]
			for j := 3; j < 8; j++ {
				lhs += float64(float64((world+i+j)%7-3) / 4 * (*m.X)[j])
			}
			if lhs != (*m.B)[i] {
				t.Fatal(world, i, lhs, (*m.B)[i])
			}
		}
	}
}

func TestLPFloatDualRatioUnboundedAndBoundaryPositions(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	m.P = 0
	(*m.ColumnTypes)[0] = ColumnLowBound
	(*m.X)[0] = -1
	m.Delta = -1
	m.ThetaD = 17
	m.FlippedBoxed = map[uint32]bool{1: true}
	for _, want := range []Status{StatusTentativeDualUnbounded, StatusDualUnbounded, StatusTentativeDualUnbounded} {
		ok, err := m.RatioTest()
		lpStaticMust(t, err)
		if ok || m.Status != want || m.ThetaD != 17 || len(m.FlippedBoxed) != 0 {
			t.Fatal(ok, m.Status, m.ThetaD, m.FlippedBoxed)
		}
	}
	for _, typ := range []ColumnType{ColumnFixed, ColumnFree, ColumnBoxed, ColumnLowBound, ColumnUpperBound} {
		(*m.ColumnTypes)[1] = typ
		(*m.LowBounds)[1], (*m.UpperBounds)[1] = 0, 2
		for _, x := range []float64{0, 1, 2} {
			(*m.X)[1] = x
			want := PositionNotAtBound
			switch typ {
			case ColumnFree:
				want = PositionFreeOfBounds
			case ColumnFixed:
				if x == 0 {
					want = PositionAtFixed
				}
			case ColumnLowBound:
				if x == 0 {
					want = PositionAtLowBound
				}
			case ColumnUpperBound:
				if x == 2 {
					want = PositionAtUpperBound
				}
			case ColumnBoxed:
				if x == 0 {
					want = PositionAtLowBound
				} else if x == 2 {
					want = PositionAtUpperBound
				}
			}
			got, err := m.GetNonBasicColumnValuePosition(1)
			lpStaticMust(t, err)
			if got != want {
				t.Fatal(typ, x, got, want)
			}
		}
	}
}

func TestLPFloatDualRatioSourceFallthroughs(t *testing.T) {
	m := lpDualInitialFixture(t, 1, 2)
	m.P = 0
	(*m.LowBounds)[0], (*m.UpperBounds)[0] = 0, 2
	m.Betas[0] = 4
	for _, typ := range []ColumnType{ColumnBoxed, ColumnLowBound, ColumnUpperBound, ColumnFree, ColumnType(99)} {
		(*m.ColumnTypes)[0] = typ
		(*m.X)[0] = 1
		got, err := m.GetDelta()
		if sourceDebugEnabled {
			if err != ErrAssertion {
				t.Fatal(typ, err)
			}
		} else {
			lpStaticMust(t, err)
			want := 0.0
			if typ != ColumnFree && typ != ColumnType(99) {
				want = -1
			}
			if got != want {
				t.Fatal(typ, got, want)
			}
		}
		gotSign, err := m.DefineSignOfAlphaR()
		if sourceDebugEnabled {
			if err != ErrAssertion {
				t.Fatal(typ, err)
			}
		} else if err != nil || gotSign != 0 {
			t.Fatal(typ, gotSign, err)
		}
	}
	(*m.ColumnTypes)[0], (*m.X)[0] = ColumnLowBound, 6
	got, err := m.GetDelta()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || got != 4 {
		t.Fatal(got, err)
	}
	sign, err := m.DefineSignOfAlphaR()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || sign != 1 {
		t.Fatal(sign, err)
	}
	m.BreakpointSet = nil
	got, err = m.CalculateHarrisDeltaOnBreakpointSet()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || got != 0 {
		t.Fatal(got, err)
	}
	(*m.ColumnTypes)[1] = ColumnType(99)
	position, err := m.GetNonBasicColumnValuePosition(1)
	if err != sourceAssertionFailure() || position != PositionAtLowBound {
		t.Fatal(position, err)
	}
	// Direct helper invocation counts q again if the caller still includes
	// it in the tight set; ordinary FindQOnTightSet removes it first.
	m.Q = 1
	m.TightSet = map[uint32]bool{1: true}
	(*m.LowBounds)[1], (*m.UpperBounds)[1], m.PivotRow.Data[1] = 0, 3, -2
	loss, err := m.DeltaLostOnFlipsOfTightBreakpoints()
	lpStaticMust(t, err)
	if loss != 12 {
		t.Fatal(loss)
	}
	(*m.ColumnTypes)[0], (*m.ColumnTypes)[1], (*m.X)[0] = ColumnLowBound, ColumnBoxed, -1
	(*m.X)[1], m.D[1], m.PivotRow.Data[1] = 0, 1, -2
	for _, delta := range []float64{math.NaN(), math.Inf(-1), math.Copysign(0, -1), 0} {
		m.Delta = delta
		ok, err := m.RatioTest()
		lpStaticMust(t, err)
		if !ok || m.Q != 1 || len(m.FlippedBoxed) != 0 || m.ThetaD != -0.5 {
			t.Fatal(ok, m.Q, m.FlippedBoxed, m.ThetaD)
		}
		if !(math.IsNaN(delta) && math.IsNaN(m.Delta)) && (delta != m.Delta || math.Signbit(delta) != math.Signbit(m.Delta)) {
			t.Fatal(delta, m.Delta)
		}
	}
}

func TestLPFloatDualRatioReachedGuards(t *testing.T) {
	type scenario struct {
		name string
		run  func(*FloatDualCoreSolver) error
	}
	cases := []scenario{
		{"position-type", func(m *FloatDualCoreSolver) error { _, e := m.GetNonBasicColumnValuePosition(9); return e }},
		{"position-upper", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[1] = ColumnUpperBound
			*m.UpperBounds = nil
			_, e := m.GetNonBasicColumnValuePosition(1)
			return e
		}},
		{"position-low", func(m *FloatDualCoreSolver) error {
			*m.LowBounds = nil
			_, e := m.GetNonBasicColumnValuePosition(1)
			return e
		}},
		{"position-box-upper", func(m *FloatDualCoreSolver) error {
			(*m.X)[1] = 1
			*m.UpperBounds = nil
			_, e := m.GetNonBasicColumnValuePosition(1)
			return e
		}},
		{"sign-type", func(m *FloatDualCoreSolver) error { m.P = -1; _, e := m.DefineSignOfAlphaR(); return e }},
		{"sign-box-low", func(m *FloatDualCoreSolver) error { *m.LowBounds = nil; _, e := m.DefineSignOfAlphaR(); return e }},
		{"sign-box-upper", func(m *FloatDualCoreSolver) error {
			(*m.X)[0] = 0
			*m.UpperBounds = nil
			_, e := m.DefineSignOfAlphaR()
			return e
		}},
		{"sign-low", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[0] = ColumnLowBound
			*m.LowBounds = nil
			_, e := m.DefineSignOfAlphaR()
			return e
		}},
		{"sign-upper", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[0] = ColumnUpperBound
			*m.UpperBounds = nil
			_, e := m.DefineSignOfAlphaR()
			return e
		}},
		{"delta-type", func(m *FloatDualCoreSolver) error { m.P = -1; _, e := m.GetDelta(); return e }},
		{"delta-box-low", func(m *FloatDualCoreSolver) error { *m.LowBounds = nil; _, e := m.GetDelta(); return e }},
		{"delta-box-upper", func(m *FloatDualCoreSolver) error {
			(*m.X)[0] = 0
			*m.UpperBounds = nil
			_, e := m.GetDelta()
			return e
		}},
		{"delta-low", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[0] = ColumnLowBound
			*m.LowBounds = nil
			_, e := m.GetDelta()
			return e
		}},
		{"delta-upper", func(m *FloatDualCoreSolver) error {
			(*m.ColumnTypes)[0] = ColumnUpperBound
			*m.UpperBounds = nil
			_, e := m.GetDelta()
			return e
		}},
		{"breakpoint-pivot", func(m *FloatDualCoreSolver) error { m.PivotRow.Data = nil; _, e := m.CanBeBreakpoint(1); return e }},
		{"breakpoint-type", func(m *FloatDualCoreSolver) error { *m.ColumnTypes = nil; _, e := m.CanBeBreakpoint(1); return e }},
		{"breakpoint-box", func(m *FloatDualCoreSolver) error { *m.LowBounds = nil; _, e := m.CanBeBreakpoint(1); return e }},
		{"fill-breakpoint", func(m *FloatDualCoreSolver) error { *m.NBasis = []uint32{1, 9}; return m.FillBreakpointSet() }},
		{"harris-low", func(m *FloatDualCoreSolver) error {
			*m.LowBounds = nil
			_, e := m.CalculateHarrisDeltaOnBreakpointSet()
			return e
		}},
		{"harris-d", func(m *FloatDualCoreSolver) error {
			m.D = nil
			_, e := m.CalculateHarrisDeltaOnBreakpointSet()
			return e
		}},
		{"tight-low", func(m *FloatDualCoreSolver) error { *m.LowBounds = nil; return m.FillTightSetOnHarrisDelta(1) }},
		{"q-pivot", func(m *FloatDualCoreSolver) error { m.TightSet = map[uint32]bool{9: true}; return m.FindQOnTightSet() }},
		{"q-harris", func(m *FloatDualCoreSolver) error { m.D = nil; return m.FindQAndTightSet() }},
		{"q-position", func(m *FloatDualCoreSolver) error { *m.ColumnTypes = nil; return m.FindQAndTightSet() }},
		{"boxed-types", func(m *FloatDualCoreSolver) error { m.Q = 9; _, e := m.TightBreakpointsAreAllBoxed(); return e }},
		{"loss-span", func(m *FloatDualCoreSolver) error {
			*m.UpperBounds = nil
			_, e := m.DeltaLostOnFlipsOfTightBreakpoints()
			return e
		}},
		{"loss-pivot", func(m *FloatDualCoreSolver) error {
			m.PivotRow.Data = nil
			_, e := m.DeltaLostOnFlipsOfTightBreakpoints()
			return e
		}},
		{"ratio-sign", func(m *FloatDualCoreSolver) error { m.P = -1; _, e := m.RatioTest(); return e }},
		{"ratio-fill", func(m *FloatDualCoreSolver) error { *m.NBasis = []uint32{9}; _, e := m.RatioTest(); return e }},
		{"ratio-q", func(m *FloatDualCoreSolver) error { m.D = nil; _, e := m.RatioTest(); return e }},
		{"ratio-loss", func(m *FloatDualCoreSolver) error { *m.UpperBounds = nil; _, e := m.RatioTest(); return e }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := lpDualInitialFixture(t, 1, 2)
			m.P = 0
			m.Q = 1
			m.SignOfAlphaR = 1
			m.Delta = -10
			(*m.ColumnTypes)[0], (*m.ColumnTypes)[1] = ColumnBoxed, ColumnBoxed
			(*m.X)[0] = -10
			(*m.LowBounds)[1], (*m.UpperBounds)[1] = 0, 2
			m.PivotRow.Data[1] = -1
			m.D[1] = 1
			m.BreakpointSet = map[uint32]bool{1: true}
			m.TightSet = map[uint32]bool{1: true}
			if err := c.run(m); err != ErrBounds {
				t.Fatal(err)
			}
		})
	}
}

func TestLPFloatDualFlipFailurePrefixes(t *testing.T) {
	m := lpDualInitialFixture(t, 2, 4)
	m.P = 0
	m.ThetaD = 2
	*m.ColumnTypes = []ColumnType{ColumnFree, ColumnFree, ColumnBoxed, ColumnBoxed}
	*m.LowBounds = []float64{0, 0, 0, 0}
	*m.UpperBounds = []float64{0, 0, 3, 5}
	m.FlippedBoxed = map[uint32]bool{2: true, 3: true}
	m.AWave = []float64{99, 88}
	lpStaticMust(t, m.A.Set(0, 2, 2))
	lpStaticMust(t, m.A.Set(1, 3, -1))
	m.D = []float64{0, 0, 9, 8}
	m.PivotRow.Data = []float64{0, 0, 1, 2}
	// Reduced costs publish first; closed LU then stops X_B reconstruction.
	m.Factorization.Close()
	err := m.UpdateDAndXB()
	if err == nil || !reflect.DeepEqual(m.D, []float64{-2, 0, 7, 4}) || !reflect.DeepEqual(m.AWave, []float64{6, -5}) || (*m.X)[2] != 0 {
		t.Fatal(err, m.D, m.AWave, *m.X)
	}
	m.Factorization = nil
	if err = m.UpdateXBAfterBoundFlips(); err != ErrBounds {
		t.Fatal(err)
	}
	// Sorted flip publication survives a later missing upper-bound slot.
	*m.UpperBounds = (*m.UpperBounds)[:3]
	err = m.ApplyFlips()
	if err != ErrBounds || (*m.X)[2] != 3 || (*m.X)[3] != 0 {
		t.Fatal(err, *m.X)
	}
	(*m.X)[2] = 1
	m.FlippedBoxed = map[uint32]bool{2: true}
	err = m.ApplyFlips()
	if sourceDebugEnabled {
		if err != ErrAssertion {
			t.Fatal(err)
		}
	} else if err != nil || (*m.X)[2] != 0 {
		t.Fatal(err, *m.X)
	}
	// Source reset of the A-wave precedes bad-column detection.
	m.AWave = []float64{99, 88}
	m.FlippedBoxed = map[uint32]bool{9: true}
	if err = m.ProcessFlipped(); err != ErrBounds || !reflect.DeepEqual(m.AWave, []float64{0, 0}) {
		t.Fatal(err, m.AWave)
	}
	m.AWave = nil
	if err = m.ProcessFlipped(); err != ErrBounds {
		t.Fatal(err)
	}
	m.FlippedBoxed = nil
	*m.NBasis = []uint32{2, 9}
	m.D[2] = 7
	if err = m.UpdateDAndXB(); err != ErrBounds || m.D[2] != 5 {
		t.Fatal(err, m.D)
	}
	*m.NBasis = nil
	m.P = -1
	if err = m.UpdateDAndXB(); err != ErrBounds {
		t.Fatal(err)
	}
	m.P = 0
	lpStaticMust(t, m.UpdateDAndXB())
	m.FlippedBoxed = map[uint32]bool{9: true}
	if err = m.ApplyFlips(); err != ErrBounds {
		t.Fatal(err)
	}
	*m.UpperBounds = nil
	if _, err = m.SignedSpanOfBoxed(2); err != ErrBounds {
		t.Fatal(err)
	}
}

func TestLPFloatDualRatioActualPivotAndFlips(t *testing.T) {
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
	lpStaticMust(t, m.CalculatePivotRowOfBInverse(0))
	lpStaticMust(t, m.CalculatePivotRowWhenInverseReady(0))
	m.Delta, err = m.GetDelta()
	lpStaticMust(t, err)
	ok, err := m.RatioTest()
	lpStaticMust(t, err)
	if !ok || m.Q != 3 || m.Delta != -7 || m.ThetaD != -8 || !reflect.DeepEqual(m.FlippedBoxed, map[uint32]bool{1: true, 2: true}) {
		t.Fatal(ok, m.Q, m.Delta, m.ThetaD, m.FlippedBoxed)
	}
	lpStaticMust(t, m.UpdateDAndXB())
	lpStaticMust(t, m.ApplyFlips())
	if !reflect.DeepEqual(*m.X, []float64{-7, 1, 1, 0}) || !reflect.DeepEqual(m.D, []float64{8, -7, -14, 0}) {
		t.Fatal(*m.X, m.D)
	}
	// Actual nonunit B inverse converts the signed column sum -6 to -3.
	if len(m.AWave) != 1 || m.AWave[0] != -3 {
		t.Fatal(m.AWave)
	}
	lhs, err := m.A.DotRow(0, *m.X)
	lpStaticMust(t, err)
	if lhs != -20 {
		t.Fatal(lhs)
	}
}

func TestLPFloatDualFlipReachedStorageErrors(t *testing.T) {
	type scenario struct {
		name string
		run  func(*FloatDualCoreSolver) error
	}
	cases := []scenario{
		{"column-link", func(m *FloatDualCoreSolver) error { m.A.Columns[2][0].Offset = 99; return m.ProcessFlipped() }},
		{"basis-prefix", func(m *FloatDualCoreSolver) error { *m.Basis = nil; return m.UpdateXBAfterBoundFlips() }},
		{"x-prefix", func(m *FloatDualCoreSolver) error { *m.X = nil; return m.UpdateXBAfterBoundFlips() }},
		{"cost-before-wave", func(m *FloatDualCoreSolver) error { m.AWave = nil; return m.UpdateDAndXB() }},
		{"flip-upper-read", func(m *FloatDualCoreSolver) error { *m.UpperBounds = nil; (*m.X)[2] = 1; return m.ApplyFlips() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := lpDualInitialFixture(t, 2, 3)
			m.P = 0
			m.ThetaD = 2
			m.FlippedBoxed = map[uint32]bool{2: true}
			(*m.ColumnTypes)[2] = ColumnBoxed
			(*m.UpperBounds)[2] = 3
			lpStaticMust(t, m.A.Set(0, 2, 2))
			wantErr := error(ErrBounds)
			if c.name == "flip-upper-read" && !sourceDebugEnabled {
				wantErr = nil
			}
			if err := c.run(m); err != wantErr {
				t.Fatal(err)
			}
		})
	}
	// Breakpoint-set construction has published its valid prefix on failure;
	// the previous flip set survives because ratio_test clears it afterwards.
	m := lpDualInitialFixture(t, 1, 2)
	m.P = 0
	(*m.ColumnTypes)[0], (*m.ColumnTypes)[1] = ColumnLowBound, ColumnLowBound
	(*m.X)[0] = -1
	m.PivotRow.Data[1] = -1
	*m.NBasis = []uint32{1, 9}
	m.FlippedBoxed = map[uint32]bool{7: true}
	_, err := m.RatioTest()
	if err != ErrBounds || !reflect.DeepEqual(m.BreakpointSet, map[uint32]bool{1: true}) || !reflect.DeepEqual(m.FlippedBoxed, map[uint32]bool{7: true}) {
		t.Fatal(err, m.BreakpointSet, m.FlippedBoxed)
	}
}
