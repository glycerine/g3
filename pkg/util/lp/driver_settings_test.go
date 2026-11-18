package lp

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func lpDriverSettings(t *testing.T) *Settings {
	t.Helper()
	s, err := NewSettings()
	lpStaticMust(t, err)
	s.ReportFrequency, s.PercentOfEnteringToCheck = 17, 2
	s.PartialPivotingConstant, s.DensityThreshold, s.HarrisFeasibilityTolerance = 13, 0.4, 0.2
	s.MaxIterationsWithoutImprovements = 3
	s.SimplexStrategy, s.PrintStatistics = StrategyUndecided, false
	s.RandomSeed(19)
	return s
}

func lpDriverAllSettings(t *testing.T) *DriverArguments {
	return lpDriverArgs(t, "--rep_frq", "5", "--percent_for_enter", "13", "--partial_pivot", "41", "--density", "25", "--maxng", "23", "--harris_toler", "0.00125", "--random_seed", "17", "--simplex_strategy", "1")
}

func TestLPDriverSettingsDefaultsAndOverrides(t *testing.T) {
	for _, profile := range []string{"default", "mpq", "empty", "all", "wrapped", "nan"} {
		s := lpDriverSettings(t)
		a := lpDriverArgs(t)
		switch profile {
		case "mpq":
			a = lpDriverArgs(t, "--mpq")
		case "empty":
			a = lpDriverArgs(t, "--mpq", "--rep_frq", "", "--density", "", "--harris_toler", "")
		case "all":
			a = lpDriverAllSettings(t)
		case "wrapped":
			a = lpDriverArgs(t, "--rep_frq", "-1", "--density", "-1", "--simplex_strategy", "-1")
		case "nan":
			a = lpDriverArgs(t, "--harris_toler", "NaN")
		}
		var out bytes.Buffer
		lpStaticMust(t, ApplyDriverSettings(a, s, &out))
		if !s.PrintStatistics {
			t.Fatal(profile, "driver did not enable statistics")
		}
		switch profile {
		case "all":
			if s.SimplexStrategy != StrategyTableauCosts || s.ReportFrequency != 5 || s.PercentOfEnteringToCheck != 13 || s.PartialPivotingConstant != 41 || s.DensityThreshold != 0.25 || s.MaxIterationsWithoutImprovements != 23 || s.HarrisFeasibilityTolerance != 0.00125 || out.String() != "setting partial pivot constant to 41\nsetting density to 0.25\nsetting harris_feasibility_tolerance to 0.00125\n" {
				t.Fatal(profile, out.String(), s)
			}
			state := uint32(17)
			for i := 0; i < 10; i++ {
				state = state*214013 + 2531011
				if s.RandomNext() != (state>>16)&0x7fff {
					t.Fatal("driver random-seed publication", i)
				}
			}
		case "wrapped":
			if s.ReportFrequency != -1 || s.SimplexStrategy != SimplexStrategy(-1) || s.DensityThreshold != float64(^uint32(0))/100 || out.String() != "setting density to 4.29497e+07\n" {
				t.Fatal(profile, out.String(), s.DensityThreshold)
			}
		case "nan":
			if s.HarrisFeasibilityTolerance == s.HarrisFeasibilityTolerance || out.String() != "setting harris_feasibility_tolerance to nan\n" {
				t.Fatal(profile, out.String())
			}
		default:
			frequency := int32(1000)
			if profile != "default" {
				frequency = 80
			}
			if s.SimplexStrategy != StrategyLU || s.ReportFrequency != frequency || s.PercentOfEnteringToCheck != 2 || s.PartialPivotingConstant != 13 || s.DensityThreshold != 0.4 || s.HarrisFeasibilityTolerance != 0.2 || s.MaxIterationsWithoutImprovements != 3 || out.Len() != 0 {
				t.Fatal(profile, s)
			}
		}
	}
}

func TestLPDriverSettingsDiagnosticPublication(t *testing.T) {
	lines := []string{"setting partial pivot constant to 41\n", "setting density to 0.25\n", "setting harris_feasibility_tolerance to 0.00125\n"}
	want := strings.Join(lines, "")
	first, second := len(lines[0]), len(lines[0])+len(lines[1])
	for accepted := 0; accepted <= len(want); accepted++ {
		s := lpDriverSettings(t)
		w := &lpResidualFailWriter{remaining: accepted}
		err := ApplyDriverSettings(lpDriverAllSettings(t), s, lpResidualWriterOnly{writer: w})
		if accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil || w.String() != want[:accepted] {
			t.Fatal(accepted, err, w.String())
		}
		partial, density, harris, maxng, strategy := int32(13), 0.4, 0.2, uint32(3), StrategyLU
		if accepted >= first {
			partial = 41
		}
		if accepted >= second {
			density, maxng = 0.25, 23
		}
		if accepted == len(want) {
			harris, strategy = 0.00125, StrategyTableauCosts
		}
		if s.PartialPivotingConstant != partial || s.DensityThreshold != density || s.HarrisFeasibilityTolerance != harris || s.MaxIterationsWithoutImprovements != maxng || s.SimplexStrategy != strategy || s.ReportFrequency != 5 || s.PercentOfEnteringToCheck != 13 || !s.PrintStatistics {
			t.Fatal("diagnostic assignment prefix", accepted, s)
		}
	}
	for line := 1; line <= 3; line++ {
		s := lpDriverSettings(t)
		w := &lpDriverFlushWriter{failAt: line}
		if err := ApplyDriverSettings(lpDriverAllSettings(t), s, w); err != lpErrResidualWrite || w.flushes != line || w.String() != strings.Join(lines[:line], "") {
			t.Fatal("endl prefix", line, err)
		}
		if line == 1 && s.PartialPivotingConstant != 13 || line == 2 && s.DensityThreshold != 0.4 || line == 3 && s.HarrisFeasibilityTolerance != 0.2 {
			t.Fatal("assignment preceded endl flush", line)
		}
	}
	for _, option := range []string{"--rep_frq", "--percent_for_enter", "--partial_pivot", "--density", "--maxng", "--harris_toler", "--random_seed", "--simplex_strategy"} {
		s := lpDriverSettings(t)
		a := lpDriverAllSettings(t)
		a.setValue(option, "12suffix")
		if err := ApplyDriverSettings(a, s, nil); err == nil || s.SimplexStrategy != StrategyLU || option == "--rep_frq" && (s.ReportFrequency != 17 || s.PrintStatistics) || option != "--rep_frq" && (s.ReportFrequency != 5 || !s.PrintStatistics) {
			t.Fatal("numeric parser boundary", option, err)
		}
	}
	if ApplyDriverSettings(nil, lpDriverSettings(t), nil) != ErrBounds || ApplyDriverSettings(lpDriverArgs(t), nil, nil) != ErrBounds {
		t.Fatal("nil settings/arguments")
	}
}

func TestLPDriverSetupActualMPSMinMax(t *testing.T) {
	for world := 0; world < 30; world++ {
		cx, cy, u, v, rhs := int64(1+world%7), int64(1+world%9), int64(3+world%5), int64(5+world%4), int64(3+world%8)
		// Independent continuous knapsack optimum for x+y<=rhs, with box
		// bounds. Spend capacity on the more valuable coordinate first.
		remaining := rhs
		x, y := int64(0), int64(0)
		if cx >= cy {
			x = min(u, remaining)
			remaining -= x
			y = min(v, remaining)
		} else {
			y = min(v, remaining)
			remaining -= y
			x = min(u, remaining)
		}
		maximum := cx*x + cy*y
		input := "NAME DRIVER\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "COST", fmt.Sprint(cx)) + lpExactMPSPair("y", "COST", fmt.Sprint(cy)) + lpExactMPSPair("x", "R", "1") + lpExactMPSPair("y", "R", "1") + "RHS\n" + lpExactMPSPair("RHS1", "R", fmt.Sprint(rhs)) + "BOUNDS\n" + lpMPSBoundLine("UP", fmt.Sprint("x ", u)) + lpMPSBoundLine("UP", fmt.Sprint("y ", v)) + "ENDATA\n"
		for _, minimum := range []bool{false, true} {
			a := lpDriverArgs(t, "lp-test", "--max_iters", "1000", "--time_limit", "60", "--density", "25", "--partial_pivot", "12", "--harris_toler", "0.00001", "--random_seed", "41", "--rep_frq", "0")
			time, maximumIterations, err := a.TimeAndIterationLimits()
			lpStaticMust(t, err)
			want := maximum
			if minimum {
				want = 0
			}
			h := newLPPairHarness(t)
			r := lpReadExactMPS(t, h, input)
			s, err := r.CreatePrimalSolver()
			h.must(err)
			h.must(s.SetupFromDriver(maximumIterations, time, minimum, a, nil))
			s.Settings.MessageOut, s.Settings.DebugOut = nil, nil
			h.must(s.FindMaximalSolution())
			out := lpScalarBoundValue(h, "97")
			h.must(s.GetCurrentCost(out))
			if minimum {
				h.must(h.m.Neg(out))
			}
			lpSparseRationalCheck(t, h, out, big.NewRat(want, 1))
			if s.Status != StatusOptimal || s.Settings.MaxTotalIterations != 1000 || s.Settings.TimeLimit != 60 || !s.Settings.PrintStatistics || s.Settings.DensityThreshold != 0.25 {
				t.Fatal("exact MPS driver", world, minimum, s.Status)
			}
			h.must(s.Close())
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
			for _, dual := range []bool{false, true} {
				reader := lpMPSRead(t, input)
				if dual {
					m, err := reader.CreateDualSolver()
					lpStaticMust(t, err)
					lpStaticMust(t, m.SetupFromDriver(maximumIterations, time, minimum, a, nil))
					m.Settings.MessageOut, m.Settings.DebugOut = nil, nil
					lpStaticMust(t, m.FindMaximalSolution())
					value, err := m.GetCurrentCost()
					lpStaticMust(t, err)
					if minimum {
						value = -value
					}
					if m.Status != StatusOptimal || value != float64(want) {
						t.Fatal("floating dual MPS driver", world, minimum, value, want, m.Status)
					}
				} else {
					m, err := reader.CreatePrimalSolver()
					lpStaticMust(t, err)
					lpStaticMust(t, m.SetupFromDriver(maximumIterations, time, minimum, a, nil))
					m.Settings.MessageOut, m.Settings.DebugOut = nil, nil
					lpStaticMust(t, m.FindMaximalSolution())
					value, err := m.GetCurrentCost()
					lpStaticMust(t, err)
					if minimum {
						value = -value
					}
					if m.Status != StatusOptimal || value != float64(want) {
						t.Fatal("floating primal MPS driver", world, minimum, value, want, m.Status)
					}
				}
			}
		}
	}
}

func TestLPDriverSetupLimitsAndFailurePrefixes(t *testing.T) {
	a := lpDriverAllSettings(t)
	for _, profile := range []string{"zero", "diagnostic", "closed", "nil-settings", "flip-failure"} {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		h.must(m.SetCostForColumn(0, lpScalarBoundValue(h, "7/13")))
		m.Settings = lpDriverSettings(t)
		m.Settings.MaxTotalIterations, m.Settings.TimeLimit = 31, 37
		settings := m.Settings
		var out *lpDriverFlushWriter
		maxIterations, time := uint32(71), uint32(9)
		minimum, want := true, error(nil)
		switch profile {
		case "zero":
			maxIterations, time, minimum = 0, 0, false
		case "diagnostic":
			out = &lpDriverFlushWriter{failAt: 1}
			want = lpErrResidualWrite
		case "closed":
			h.must(m.Close())
			want = ErrExactVectorPending
		case "nil-settings":
			m.Settings = nil
			want = ErrBounds
		case "flip-failure":
			m.Columns.Set(1, nil)
			want = ErrBounds
		}
		var err error
		if out != nil {
			err = m.SetupFromDriver(maxIterations, time, minimum, a, out)
		} else {
			err = m.SetupFromDriver(maxIterations, time, minimum, a, nil)
		}
		if err != want {
			t.Fatal(profile, err, want)
		}
		m.Settings = settings
		if profile == "zero" {
			if settings.MaxTotalIterations != 31 || settings.TimeLimit != 37 {
				t.Fatal("zero limits overwrite retained settings")
			}
		} else if profile == "diagnostic" || profile == "flip-failure" {
			if settings.MaxTotalIterations != 71 || settings.TimeLimit != 9 {
				t.Fatal("limits must precede flip and diagnostic failure")
			}
			// flip_costs walks the columns in libstdc++ order (1 before 0),
			// so a null column 1 stops it before column 0 flips.
			c, _ := m.Columns.Get2(0)
			if profile == "flip-failure" {
				lpSparseRationalCheck(t, h, &c.Cost, big.NewRat(7, 13))
			} else {
				lpSparseRationalCheck(t, h, &c.Cost, big.NewRat(-7, 13))
			}
			if profile == "diagnostic" && (settings.PartialPivotingConstant != 13 || settings.ReportFrequency != 5 || !settings.PrintStatistics) || profile == "flip-failure" && (settings.SimplexStrategy != StrategyUndecided || settings.ReportFrequency != 17 || settings.PrintStatistics) {
				t.Fatal("setup publication prefix", profile)
			}
			if m.SetupFromDriver(0, 0, false, a, nil) != ErrExactVectorPending {
				t.Fatal("failed setup retry")
			}
		}
		h.must(m.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for _, profile := range []string{"zero", "diagnostic", "nil-settings", "flip-failure"} {
		m, err := NewFloatLPSolver()
		lpStaticMust(t, err)
		m.GetOrCreateColumnInfo(0).Cost = 7
		m.Settings = lpDriverSettings(t)
		m.Settings.MaxTotalIterations, m.Settings.TimeLimit = 31, 37
		maxIterations, time, minimum := uint32(71), uint32(9), true
		want := error(nil)
		if profile == "zero" {
			maxIterations, time, minimum = 0, 0, false
		} else if profile == "nil-settings" {
			m.Settings, want = nil, ErrBounds
		} else if profile == "flip-failure" {
			m.Columns[1], want = nil, ErrBounds
		}
		if profile == "diagnostic" {
			err = m.SetupFromDriver(maxIterations, time, minimum, a, &lpDriverFlushWriter{failAt: 1})
			want = lpErrResidualWrite
		} else {
			err = m.SetupFromDriver(maxIterations, time, minimum, a, nil)
		}
		if err != want {
			t.Fatal(profile, err, want)
		}
		if profile == "zero" && (m.Settings.MaxTotalIterations != 31 || m.Settings.TimeLimit != 37 || m.Columns[0].Cost != 7) || (profile == "diagnostic" || profile == "flip-failure") && (m.Settings.MaxTotalIterations != 71 || m.Settings.TimeLimit != 9 || m.Columns[0].Cost != map[bool]float64{false: -7, true: 7}[profile == "flip-failure"]) {
			// flip_costs visits column 1 before column 0 (libstdc++ order).
			t.Fatal("float setup publication prefix", profile)
		}
	}
}

func TestLPDriverSetupPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		success, failures, prefix := false, 0, false
		for ordinal := int64(0); ordinal < 2000 && !success; ordinal++ {
			if !t.Run(fmt.Sprintf("direct%t/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				m := lpRationalInputModel(t, h)
				m.Settings = lpDriverSettings(t)
				// flip_costs visits column 1 before column 0 (libstdc++
				// order), so the large cost sits in the later column 0.
				h.must(m.SetCostForColumn(0, lpScalarBoundValue(h, strings.Repeat("1234567890", 100)+"7/13")))
				h.must(m.SetCostForColumn(1, lpScalarBoundValue(h, "7")))
				a := lpDriverAllSettings(t)
				h.budget.FailAfter(ordinal)
				err := m.SetupFromDriver(71, 9, true, a, nil)
				h.budget.FailAfter(-1)
				if m.Settings.MaxTotalIterations != 71 || m.Settings.TimeLimit != 9 {
					t.Fatal("limits before cost allocation")
				}
				if err == nil {
					success = true
					if m.Settings.SimplexStrategy != StrategyTableauCosts {
						t.Fatal("settings after complete cost flip")
					}
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) || m.SetupFromDriver(0, 0, false, a, nil) != ErrExactVectorPending || m.Settings.SimplexStrategy != StrategyUndecided || m.Settings.ReportFrequency != 17 || m.Settings.PrintStatistics {
						t.Fatal("cost flip failure prefix", err)
					}
					c, _ := m.Columns.Get2(1)
					value, inspectErr := h.m.Int64(&c.Cost)
					prefix = prefix || inspectErr == nil && value == -7
				}
				h.must(m.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}) {
				t.FailNow()
			}
		}
		if !success || direct && (failures == 0 || !prefix) {
			t.Fatal("setup physical campaign", direct, success, failures, prefix)
		}
	}
}

func TestLPDriverSetupPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		m := lpRationalInputModel(t, h)
		h.must(m.SetCostForColumn(0, lpScalarBoundValue(h, strings.Repeat("1234567890", 100)+"7/13")))
		v := h.pairExactLarge(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
		var caller exactEtaCalls
		before, err := caller.begin(h.m)
		h.must(err)
		h.budget.FailAfter(ordinal)
		_, err = h.m.Compare(&v.X, &v.Y)
		h.budget.FailAfter(-1)
		err = caller.after(h.m, before, err)
		older := h.m.ComparisonInvocationState()
		if older.Active {
			matched = true
			if !lpExactPriorityPhysicalRefusal(err) {
				t.Fatal(err)
			}
			h.must(m.SetupFromDriver(71, 9, true, lpDriverAllSettings(t), nil))
			if m.calls.comparison != 0 {
				t.Fatal("setup adopted caller comparison")
			}
			for _, c := range m.Columns.All() {
				if c.calls.comparison != 0 {
					t.Fatal("column adopted caller comparison")
				}
			}
		}
		h.must(m.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("setup consumed caller comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}
