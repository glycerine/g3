package lp

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLPGLPKReportLiteralFieldSelection(t *testing.T) {
	name := lpMPSDriverFile(t, "heading\nnotStatus: NOVEL\nStatus: OPTIMAL\nObjective: obj = -7.25 (MAXimum)\nObjective: obj = 99 (MAXimum)\ncost = 1\ncost = -8.5")
	if v, err := ReadDriverReportStatus(name, io.Discard); err != nil || v != "NOVEL" {
		t.Fatal(v, err)
	}
	if v, err := ReadGLPKReportCost(name, io.Discard); err != nil || v != -7.25 {
		t.Fatal(v, err)
	}
	if v, err := ReadLPReportCost(name, io.Discard); err != nil || v != -8.5 {
		t.Fatal(v, err)
	}
	name = lpMPSDriverFile(t, "\tStatus:\tOPTIMAL\r\ncost = 2\nprimal cost = 3\n")
	if v, err := ReadDriverReportStatus(name, io.Discard); err != nil || v != "OPTIMAL\r" {
		t.Fatal(v, err)
	}
	var out bytes.Buffer
	if _, err := ReadLPReportCost(name, &out); err != ErrDriverReport || out.String() != "unexpected cost string primal cost = 3\n" {
		t.Fatal(err, out.String())
	}
	// The first Objective match is parsed even if a later one is well formed.
	name = lpMPSDriverFile(t, "Objective bad\nObjective: x = 2 (MAXimum)\n")
	out.Reset()
	if _, err := ReadGLPKReportCost(name, &out); err != ErrDriverReport || out.String() != "unexpected Objective std::string Objective bad\n" {
		t.Fatal(err, out.String())
	}
	for _, number := range []string{"1e309", "-1e309", "NaN", "-0", "1.25e-200"} {
		name = lpMPSDriverFile(t, "Objective: x = "+number+" (MAXimum)\ncost = "+number)
		a, err := ReadGLPKReportCost(name, io.Discard)
		if err != nil {
			t.Fatal(number, err)
		}
		b, err := ReadLPReportCost(name, io.Discard)
		if err != nil {
			t.Fatal(number, err)
		}
		if math.Float64bits(a) != math.Float64bits(b) {
			t.Fatal(number, a, b)
		}
	}
}

func TestLPGLPKReportStatusCrossProduct(t *testing.T) {
	statuses := []string{"OPTIMAL", "INFEASIBLE", "UNBOUNDED", "UNDEFINED", "EMPTY", "UNKNOWN", "other"}
	for _, a := range statuses {
		for _, b := range statuses {
			glpk := lpMPSDriverFile(t, "Status: "+a+"\nObjective: x = 5 (MAXimum)\n")
			local := lpMPSDriverFile(t, "Status: "+b+"\ncost = 5\n")
			counts := DriverComparisonCounts{Successes: 11, Failures: 17, Inconclusives: 23}
			var out bytes.Buffer
			if err := CompareGLPKDriverReports(glpk, local, &counts, &out); err != nil {
				t.Fatal(a, b, err)
			}
			wantSuccess, wantFailure := uint32(11), uint32(17)
			wantReport := ""
			if a == b {
				if a == "OPTIMAL" {
					wantSuccess++
				}
			} else if a == "UNDEFINED" && (b == "INFEASIBLE" || b == "UNBOUNDED") {
				wantSuccess++
			} else {
				wantFailure++
				wantReport = "glpsol and lp_tst disagree: glpsol status is " + a + " but lp_tst status is " + b + "\n"
			}
			if counts.Successes != wantSuccess || counts.Failures != wantFailure || counts.Inconclusives != 23 || out.String() != wantReport {
				t.Fatal(a, b, counts, out.String())
			}
		}
	}
}

func TestLPGLPKReportIndependentCostComparisons(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 100; world++ {
		// Decimal integer worlds have independently obvious within/outside 1%.
		a := float64(1000 + world*100)
		b := a
		if world%3 == 1 {
			b += 1
		}
		close := world%3 != 2
		if !close {
			b *= 2
		}
		glpk := lpMPSDriverFile(t, fmt.Sprintf("Status: OPTIMAL\nObjective: obj = %.17g (MAXimum)\n", a))
		local := lpMPSDriverFile(t, fmt.Sprintf("Status: OPTIMAL\ncost = 1\ncost = %.17g\n", b))
		counts := DriverComparisonCounts{}
		var out bytes.Buffer
		if err := CompareGLPKDriverReports(glpk, local, &counts, &out); err != nil {
			t.Fatal(world, err)
		}
		if close {
			if counts.Successes != 1 || counts.Failures != 0 || out.Len() != 0 {
				t.Fatal(world, counts, out.String())
			}
		} else {
			want := fmt.Sprintf("glpsol cost is %g lp_tst cost is %g\n", a, b)
			if counts.Successes != 0 || counts.Failures != 1 || out.String() != want {
				t.Fatal(world, counts, out.String(), want)
			}
		}
	}
	for _, tc := range []struct {
		a, b  string
		close bool
	}{
		{"0", "0.0000009", true}, {"0", "0.000001", false}, {"NaN", "0", false},
		{"+Inf", "+Inf", false}, {"1e309", "0", true}, {"-0", "0", true},
	} {
		glpk := lpMPSDriverFile(t, "Status: OPTIMAL\nObjective: obj = "+tc.a+" (MAXimum)\n")
		local := lpMPSDriverFile(t, "Status: OPTIMAL\ncost = "+tc.b+"\n")
		c := DriverComparisonCounts{}
		if err := CompareGLPKDriverReports(glpk, local, &c, io.Discard); err != nil {
			t.Fatal(tc, err)
		}
		if (c.Successes == 1) != tc.close || (c.Failures == 1) == tc.close {
			t.Fatal(tc, c)
		}
	}
	// Native uint32 counter arithmetic matches source unsigned wrap.
	glpk := lpMPSDriverFile(t, "Status: UNDEFINED\n")
	local := lpMPSDriverFile(t, "Status: INFEASIBLE\n")
	c := DriverComparisonCounts{Successes: ^uint32(0)}
	if err := CompareGLPKDriverReports(glpk, local, &c, io.Discard); err != nil || c.Successes != 0 {
		t.Fatal(c, err)
	}
}

func TestLPGLPKReportDiagnosticsAndNativeReadErrors(t *testing.T) {
	for _, tc := range []struct{ text, field, report string }{
		{"status: OPTIMAL\n", "Status", "cannot find the status line in "},
		{"Status: OPTIMAL trailing\n", "Status", "unexpected Status string Status: OPTIMAL trailing\n"},
		{"", "Objective", "cannot find the Objective line in "},
		{"Objective 1\n", "Objective", "unexpected Objective std::string Objective 1\n"},
		{"Cost = 1\n", "cost", "cannot find the cost line in "},
		{"cost 1\n", "cost", "unexpected cost string cost 1\n"},
	} {
		name := lpMPSDriverFile(t, tc.text)
		report := tc.report
		if strings.HasSuffix(report, " in ") {
			report += name + "\n"
		}
		for at := -1; at < len(report); at++ {
			var out bytes.Buffer
			var writer io.Writer = &out
			var fail *lpResidualFailWriter
			if at >= 0 {
				fail = &lpResidualFailWriter{remaining: at}
				writer = lpResidualWriterOnly{fail}
			}
			var err error
			switch tc.field {
			case "Status":
				_, err = ReadDriverReportStatus(name, writer)
			case "Objective":
				_, err = ReadGLPKReportCost(name, writer)
			case "cost":
				_, err = ReadLPReportCost(name, writer)
			}
			if at < 0 {
				if err != ErrDriverReport || out.String() != report {
					t.Fatal(tc, err, out.String())
				}
			} else if err != lpErrResidualWrite || fail.String() != report[:at] {
				t.Fatal(tc, at, err, fail.String())
			}
		}
	}
	for _, field := range []string{"Status", "Objective", "cost"} {
		name := filepath.Join(t.TempDir(), "missing")
		var out bytes.Buffer
		if _, err := readDriverReportLine(name, field, false, &out); err != ErrDriverReport || out.String() != "cannot open "+name+"\n" {
			t.Fatal(field, err, out.String())
		}
		out.Reset()
		if _, err := readDriverReportLine(t.TempDir(), field, false, &out); err == nil || err == ErrDriverReport || out.Len() != 0 {
			t.Fatal("native directory read error", err, out.String())
		}
	}
	for _, field := range []string{"Objective", "cost"} {
		text := "Objective: x = invalid (MAXimum)\n"
		if field == "cost" {
			text = "cost = invalid\n"
		}
		name := lpMPSDriverFile(t, text)
		var err error
		var cost float64
		if field == "cost" {
			cost, err = ReadLPReportCost(name, io.Discard)
		} else {
			cost, err = ReadGLPKReportCost(name, io.Discard)
		}
		// The source reads the cost with atof (AUDIT.md util_lp_04-F1).
		if err != nil || cost != 0 {
			t.Fatal(cost, err)
		}
	}
}

func TestLPGLPKReportPublicationAndComparisonFailures(t *testing.T) {
	for _, cost := range []bool{false, true} {
		a, b := "OPTIMAL", "INFEASIBLE"
		report := "glpsol and lp_tst disagree: glpsol status is OPTIMAL but lp_tst status is INFEASIBLE\n"
		if cost {
			b = "OPTIMAL"
			report = "glpsol cost is 1 lp_tst cost is 2\n"
		}
		glpk := lpMPSDriverFile(t, "Status: "+a+"\nObjective: x = 1 (MAXimum)\n")
		local := lpMPSDriverFile(t, "Status: "+b+"\ncost = 2\n")
		for at := 0; at < len(report); at++ {
			c := DriverComparisonCounts{}
			w := &lpResidualFailWriter{remaining: at}
			if err := CompareGLPKDriverReports(glpk, local, &c, lpResidualWriterOnly{w}); err != lpErrResidualWrite || w.String() != report[:at] {
				t.Fatal(cost, at, err, w.String())
			}
			want := uint32(0)
			if cost {
				want = 1
			}
			if c.Failures != want || c.Successes != 0 {
				t.Fatal("counter prefix", cost, at, c)
			}
		}
		c := DriverComparisonCounts{}
		w := &lpMPSDriverActionWriter{failFlush: 1}
		if err := CompareGLPKDriverReports(glpk, local, &c, w); err == nil || w.flushes != 1 {
			t.Fatal(err, w.flushes)
		}
		want := uint32(0)
		if cost {
			want = 1
		}
		if c.Failures != want {
			t.Fatal(c)
		}
		c = DriverComparisonCounts{}
		w = &lpMPSDriverActionWriter{action: func(string) error {
			// The counter is borrowed and reread after each source report chain.
			c.Failures = 100
			return nil
		}}
		if err := CompareGLPKDriverReports(glpk, local, &c, w); err != nil {
			t.Fatal(err)
		}
		want = 101
		if cost {
			want = 100
		}
		if c.Failures != want {
			t.Fatal(cost, c)
		}
	}
	if err := CompareGLPKDriverReports("", "", nil, io.Discard); err != ErrBounds {
		t.Fatal(err)
	}
	// Every parser stage must stop before counters change.
	good := lpMPSDriverFile(t, "Status: OPTIMAL\nObjective: x = 1 (MAXimum)\ncost = 1\n")
	missing := good + ".missing"
	badCost := lpMPSDriverFile(t, "Status: OPTIMAL\n")
	for _, names := range [][2]string{{missing, good}, {good, missing}, {badCost, good}, {good, badCost}} {
		c := DriverComparisonCounts{Successes: 9, Failures: 7, Inconclusives: 5}
		if err := CompareGLPKDriverReports(names[0], names[1], &c, io.Discard); err != ErrDriverReport || c != (DriverComparisonCounts{9, 7, 5}) {
			t.Fatal(names, err, c)
		}
	}
	// Missing-file diagnostic report failure takes precedence over integer throw.
	w := &lpResidualFailWriter{}
	if _, err := ReadDriverReportStatus(missing, lpResidualWriterOnly{w}); err != lpErrResidualWrite {
		t.Fatal(err)
	}
	// Successful readonly file readers leave no descriptor ownership in the API.
	if err := os.Remove(good); err != nil {
		t.Fatal(err)
	}
}

func TestLPGLPKReportActualMPSDriverIntegration(t *testing.T) {
	for world := 0; world < 8; world++ {
		for _, mode := range []string{"float", "float-comparison", "exact"} {
			h := newLPPairHarness(t)
			name := lpMPSDriverFile(t, lpMPSDriverModel(int64(3+world), 1, 4, 5, 6))
			var output bytes.Buffer
			clock := &lpMPSDriverClock{}
			d := NewMPSFileDriver(name, lpMPSDriverArguments(t, mode == "exact", false, false, mode == "float-comparison"), h.m, MPSDriverConfig{Output: &output, Clock: clock.read})
			h.must(d.Run())
			if d.Status != StatusOptimal {
				t.Fatal(mode, d.Status)
			}
			local := lpMPSDriverFile(t, output.String())
			// Independent boxed knapsack optimum: four high-value items, two low.
			glpk := lpMPSDriverFile(t, fmt.Sprintf("Status: OPTIMAL\nObjective: obj = %d (MAXimum)\n", 4*(3+world)+2))
			counts := DriverComparisonCounts{}
			var report bytes.Buffer
			err := CompareGLPKDriverReports(glpk, local, &counts, &report)
			if mode == "float-comparison" {
				// Source's last substring match is its four-token primal-cost report.
				if err != ErrDriverReport || counts != (DriverComparisonCounts{}) || !strings.HasPrefix(report.String(), "unexpected cost string primal cost = ") {
					t.Fatal(mode, err, counts, report.String())
				}
			} else if err != nil || counts.Successes != 1 || counts.Failures != 0 || report.Len() != 0 {
				t.Fatal(mode, err, counts, report.String())
			}
			h.must(d.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPGLPKReportDiagnosticFlushFailures(t *testing.T) {
	for _, field := range []string{"Status", "Objective", "cost"} {
		name := lpMPSDriverFile(t, "")
		out := &lpMPSDriverActionWriter{failFlush: 1}
		var err error
		switch field {
		case "Status":
			_, err = ReadDriverReportStatus(name, out)
		case "Objective":
			_, err = ReadGLPKReportCost(name, out)
		case "cost":
			_, err = ReadLPReportCost(name, out)
		}
		if err == nil || err == ErrDriverReport || out.flushes != 1 {
			t.Fatal(field, err, out.flushes)
		}
	}
	// Nil output uses the ordinary standard stream; successful parsing is silent.
	name := lpMPSDriverFile(t, "Status: NOVEL\nObjective: x = 7 (MAXimum)\n")
	if v, err := ReadDriverReportStatus(name, nil); err != nil || v != "NOVEL" {
		t.Fatal(v, err)
	}
	if v, err := ReadGLPKReportCost(name, nil); err != nil || v != 7 {
		t.Fatal(v, err)
	}
}
