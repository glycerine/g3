package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func lpMPSPair(col, row string, v float64) string {
	return fmt.Sprintf("    %-8s  %-8s  %.12g\n", col, row, v)
}
func lpMPSBoundLine(typ, data string) string {
	return fmt.Sprintf(" %-2s %-8s  %s\n", typ, "BND", data)
}
func lpMPSRead(t *testing.T, text string) *FloatMPSReader {
	t.Helper()
	m := NewFloatMPSReader(strings.NewReader(text))
	m.Messages = &bytes.Buffer{}
	if err := m.Read(); err != nil || !m.IsOK() {
		t.Fatal(err, m.IsOK(), m.line, m.LineNumber, m.Messages)
	}
	return m
}
func TestLPFloatMPSIndependentOptima(t *testing.T) {
	rng := rand.New(rand.NewSource(8981170))
	for world := 0; world < 120; world++ {
		a := make([][]float64, 3)
		b := make([]float64, 3)
		c := []float64{float64(1 + rng.Intn(9)), float64(1 + rng.Intn(9)), 0, 0, 0}
		var input strings.Builder
		input.WriteString("* comment\nNAME MODEL\n \t\nROWS\n N COST\n L R0\n L R1\n L R2\nCOLUMNS\n")
		for i := range a {
			a[i] = []float64{float64(1 + rng.Intn(8)), float64(1 + rng.Intn(8)), 0, 0, 0}
			a[i][i+2] = 1
			b[i] = float64(1 + rng.Intn(24))
		}
		for j, name := range []string{"x", "y"} {
			input.WriteString(lpMPSPair(name, "COST", c[j]))
			for i := range a {
				input.WriteString(lpMPSPair(name, fmt.Sprint("R", i), a[i][j]))
			}
		}
		input.WriteString("RHS\n")
		for i, v := range b {
			input.WriteString(lpMPSPair("RHS1", fmt.Sprint("R", i), v))
		}
		input.WriteString("ENDATA\n")
		reader := lpMPSRead(t, input.String())
		if reader.Name != "MODEL" || !reflect.DeepEqual(reader.ColumnNames(), []string{"y", "x"}) { // libstdc++ m_columns order (util_lp_06-F1)
			t.Fatal(reader.Name, reader.ColumnNames())
		}
		solver, err := reader.CreatePrimalSolver()
		lpStaticMust(t, err)
		t.Cleanup(func() { solver.Close() })
		solver.Settings.SimplexStrategy = StrategyLU
		solver.Settings.DebugOut = nil
		solver.Settings.MessageOut = nil
		lpStaticMust(t, solver.FindMaximalSolution())
		cost, err := solver.GetCurrentCost()
		lpStaticMust(t, err)
		want := lpVertexMaximum(a, b, c)
		if solver.Status != StatusOptimal || math.Abs(cost-want) > 1e-7 {
			t.Fatal(world, solver.Status, cost, want)
		}
		solution := map[string]float64{}
		for _, name := range reader.ColumnNames() {
			v, err := solver.GetColumnValue(lpTestFloatNamedColumn(t, solver.NamesToColumns, name))
			lpStaticMust(t, err)
			solution[name] = v
			if v < -1e-8 {
				t.Fatal(world, name, v)
			}
		}
		for i := range b {
			if a[i][0]*solution["x"]+a[i][1]*solution["y"] > b[i]+1e-7 {
				t.Fatal(world, i, solution)
			}
		}
		if ok, err := solver.SolutionIsFeasible(solution); !ok || err != nil {
			t.Fatal(world, ok, err)
		}
	}
}
func TestLPFloatMPSFileBoundsRangesIntegration(t *testing.T) {
	reader := OpenFloatMPSReader(filepath.Join("test_vectors", "mps_small.mps"))
	defer reader.Close()
	reader.Messages = &bytes.Buffer{}
	if err := reader.Read(); err != nil || !reader.IsOK() {
		t.Fatal(err, reader.Messages)
	}
	solver, err := reader.CreatePrimalSolver()
	lpStaticMust(t, err)
	defer solver.Close()
	solver.Settings.SimplexStrategy = StrategyLU
	solver.Settings.DebugOut = nil
	solver.Settings.MessageOut = nil
	lpStaticMust(t, solver.FindMaximalSolution())
	cost, err := solver.GetCurrentCost()
	lpStaticMust(t, err)
	if solver.Status != StatusOptimal || math.Abs(cost-24) > 1e-8 {
		t.Fatal(solver.Status, cost)
	}
	solution := map[string]float64{}
	for _, name := range reader.ColumnNames() {
		v, err := solver.GetColumnValue(lpTestFloatNamedColumn(t, solver.NamesToColumns, name))
		lpStaticMust(t, err)
		solution[name] = v
	}
	if solution["f"] != 2 || math.Abs(solution["x"]-2) > 1e-8 || math.Abs(solution["y"]-4) > 1e-8 {
		t.Fatal(solution)
	}
	if ok, err := solver.SolutionIsFeasible(solution); !ok || err != nil {
		t.Fatal(ok, err)
	}
	lpStaticMust(t, reader.Close())
	missing := OpenFloatMPSReader(filepath.Join(t.TempDir(), "missing.mps"))
	missing.Messages = &bytes.Buffer{}
	if !missing.IsOK() {
		t.Fatal("open failure observed too early")
	}
	lpStaticMust(t, missing.Read())
	if missing.IsOK() || missing.Messages.(*bytes.Buffer).String() != "setting m_is_OK to false\n" {
		t.Fatal(missing.IsOK(), missing.Messages)
	}
}
func TestLPFloatMPSAllBoundsAndRanges(t *testing.T) {
	for _, typ := range []string{"UP", "UI", "LIMITMAX", "LO", "LI", "FR", "FX", "PL", "MI", "BV"} {
		text := "NAME B\nROWS\n N COST\n L R\nCOLUMNS\n" + lpMPSPair("x", "COST", 1) + lpMPSPair("x", "R", 1) + "RHS\n" + lpMPSPair("RHS1", "R", 8) + "BOUNDS\n" + lpMPSBoundLine(typ, "x 3") + "ENDATA\n"
		if typ == "LIMITMAX" && sourceDebugEnabled {
			m := NewFloatMPSReader(strings.NewReader(text))
			m.Messages = &bytes.Buffer{}
			if err := m.Read(); err != ErrAssertion {
				t.Fatal(err)
			}
			continue
		}
		m := lpMPSRead(t, text)
		col, _ := m.columns.Get2("x")
		b := col.bound
		if b == nil {
			t.Fatal(typ)
		}
		switch typ {
		case "UP", "UI", "LIMITMAX":
			if !b.upperSet || b.upper != 3 || !b.lowSet {
				t.Fatal(typ, b)
			}
		case "LO", "LI":
			if b.low != 3 || !b.lowSet {
				t.Fatal(typ, b)
			}
		case "FR":
			if !b.free {
				t.Fatal(typ)
			}
		case "FX":
			if !b.fixedSet || b.fixed != 3 {
				t.Fatal(typ)
			}
		case "PL":
			if !b.lowSet || b.low != 0 {
				t.Fatal(typ)
			}
		case "MI":
			if !b.upperSet || b.upper != 0 || !b.lowSet {
				t.Fatal("source MI retains default low", b)
			}
		case "BV":
			if b.upper != 1 || !b.upperSet {
				t.Fatal(typ)
			}
		}
		solver, err := m.CreatePrimalSolver()
		lpStaticMust(t, err)
		defer solver.Close()
		ci := solver.Columns[0]
		if typ == "FR" && (!ci.IsFree() || ci.LowSet || ci.UpperSet) {
			t.Fatal(ci)
		}
	}
	for _, kind := range []string{"G", "L", "E"} {
		for _, span := range []float64{-3, 0, 3} {
			for _, boundsFirst := range []bool{false, true} {
				head := "NAME R\nROWS\n N COST\n " + kind + " R\nCOLUMNS\n" + lpMPSPair("x", "COST", 1) + lpMPSPair("x", "R", 2) + "RHS\n" + lpMPSPair("RHS1", "R", 8)
				bounds := "BOUNDS\n" + lpMPSBoundLine("UP", "x 9")
				ranges := "RANGES\n" + lpMPSPair("RNG", "R", span)
				if boundsFirst {
					head += bounds + ranges
				} else {
					head += ranges + bounds
				}
				head += "ENDATA\n"
				m := lpMPSRead(t, head)
				r, _ := m.rows.Get2("R")
				other, _ := m.rows.Get2("R_range")
				wantType, wantOther, wantRHS := mpsGreater, mpsLess, 8+math.Abs(span)
				if kind == "L" {
					wantType, wantOther, wantRHS = mpsLess, mpsGreater, 8-math.Abs(span)
				}
				if kind == "E" {
					wantRHS = 8 + span
					if span <= 0 {
						wantType, wantOther = mpsLess, mpsGreater
					}
				}
				if r.typ != wantType || other.typ != wantOther || other.rhs != wantRHS || other.index != 1 || other.coefficients.Get("x") != 2 {
					t.Fatal(kind, span, r, other)
				}
				solver, err := m.CreatePrimalSolver()
				lpStaticMust(t, err)
				defer solver.Close()
				if len(solver.Constraints) != 2 {
					t.Fatal(solver.Constraints)
				}
			}
		}
	}
}
func TestLPFloatMPSFixedFieldsAndPublication(t *testing.T) {
	m := NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.rows.Set("R A", newFloatMPSRow("R A", 0, mpsLess))
	m.rows.Set("R B", newFloatMPSRow("R B", 1, mpsGreater))
	m.columns.Set("X A", &floatMPSColumn{name: "X A"})
	pair := fmt.Sprintf("%-8s%14s   %-8s%14s", "R A", "2", "R B", "3")
	m.line = "              " + pair
	lpStaticMust(t, m.rowPairs(pair, "X A", 0, mpsCoefficient))
	r, _ := m.rows.Get2("R A")
	s, _ := m.rows.Get2("R B")
	if r.coefficients.Get("X A") != 2 || s.coefficients.Get("X A") != 3 {
		t.Fatal(r, s)
	}
	lpStaticMust(t, m.rowPairs(pair, "", 0, mpsRHS))
	if r.rhs != 2 || s.rhs != 3 {
		t.Fatal(r, s)
	}
	lpStaticMust(t, m.rowPairs(m.line, "", 1, mpsRange))
	if r.span != 2 || s.span != 3 {
		t.Fatal(r, s)
	}
	m.line = " UP BND       " + fmt.Sprintf("%-8s%6s%8s", "X A", "", "4")
	lpStaticMust(t, m.createBound())
	col, _ := m.columns.Get2("X A")
	if col.bound.upper != 4 {
		t.Fatal(col.bound)
	}
	// Duplicates overwrite coefficients; a blank column field continues the name.
	text := "NAME D\nROWS\n N COST\n L R\nCOLUMNS\n" + lpMPSPair("x", "COST", 1) + lpMPSPair("", "COST", 3) + lpMPSPair("", "R", 2) + "    MARK      'MARKER' 'INTORG'\nRHS\n" + lpMPSPair("RHS1", "R", 7) + "ENDATA\n"
	parsed := lpMPSRead(t, text)
	cost, _ := parsed.rows.Get2("COST")
	if cost.coefficients.Get("x") != 3 || !reflect.DeepEqual(parsed.ColumnNames(), []string{"MARK", "x"}) { // libstdc++ m_columns order (util_lp_06-F1)
		t.Fatal(parsed.ColumnNames())
	}
	// Repeated range expansion replaces the same name with the current source index.
	lpStaticMust(t, m.expandRange(r))
	replacement, _ := m.rows.Get2("R A_range")
	if replacement.index != uint32(m.rows.Len()) {
		t.Fatal(replacement.index, m.rows.Len())
	}
}
func TestLPFloatMPSMalformedAndStreamPrefixes(t *testing.T) {
	m := NewFloatMPSReader(strings.NewReader(""))
	var out bytes.Buffer
	m.Messages = &out
	lpStaticMust(t, m.readLine())
	if m.IsOK() || m.LineNumber != 2 || out.String() != "setting m_is_OK to false\ncannot read from file\n" {
		t.Fatal(m.IsOK(), m.LineNumber, out.String())
	}
	stalled := NewFloatMPSReader(strings.NewReader("NAME S\nJUNK\nROWS\n"))
	stalled.Messages = &out
	if err := stalled.Read(); err != ErrMPSStalled || !stalled.IsOK() || stalled.LineNumber != 2 {
		t.Fatal(err, stalled.LineNumber)
	}
	for _, data := range []string{"short", strings.Repeat(" ", 22)} {
		m = NewFloatMPSReader(nil)
		m.Messages = &bytes.Buffer{}
		m.line = data
		lpStaticMust(t, m.fixedRowPairs(data, "x", mpsRHS))
		if m.IsOK() {
			t.Fatal(data)
		}
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.line = "short"
	lpStaticMust(t, m.fixedRowPairs("", "", mpsRange))
	if m.IsOK() {
		t.Fatal("short range")
	}
	if err := m.rowPairs("", "", 0, mpsCoefficient); err != ErrBounds {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(strings.NewReader("tiny\n"))
	m.Messages = &bytes.Buffer{}
	lpStaticMust(t, m.readColumns())
	if m.IsOK() {
		t.Fatal("short columns")
	}
	m = NewFloatMPSReader(strings.NewReader("tiny\n"))
	m.Messages = &bytes.Buffer{}
	lpStaticMust(t, m.readRHS())
	if m.IsOK() {
		t.Fatal("short RHS")
	}
	m = NewFloatMPSReader(strings.NewReader(""))
	m.Messages = &bytes.Buffer{}
	m.line = "BOUNDS"
	if err := m.readBounds(); err != ErrBounds {
		t.Fatal(err)
	}
	for _, typ := range []string{"UP", "LO", "FX"} {
		m = NewFloatMPSReader(nil)
		m.Messages = &bytes.Buffer{}
		m.line = lpMPSBoundLine(typ, "x")
		lpStaticMust(t, m.updateBound(&floatMPSBound{}, []string{"x"}))
		if m.IsOK() {
			t.Fatal(typ)
		}
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.line = lpMPSBoundLine("ZZ", "x 1")
	if err := m.createBound(); err != nil { // Missing column takes fallback before bound type.
		t.Fatal(err)
	}
	if m.IsOK() {
		t.Fatal("missing bound column")
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.line = lpMPSBoundLine("ZZ", "x 1")
	if err := m.updateBound(&floatMPSBound{}, []string{"x", "1"}); err != ErrMPSUnhandled || m.IsOK() {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.line = strings.Repeat(" ", 14)
	if err := m.createBound(); err != ErrMPSUnhandled || m.IsOK() {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.line = "short"
	want := ErrBounds
	if sourceDebugEnabled {
		want = ErrAssertion
	}
	if err := m.createBound(); err != want {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	if _, err := m.relation(mpsCost); err != ErrMPSUnhandled || m.IsOK() {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	if err := m.expandRange(newFloatMPSRow("C", 0, mpsCost)); err != ErrMPSUnhandled || m.IsOK() {
		t.Fatal(err)
	}
	// Numeric conversion is the native Go boundary; flags publish before refusal.
	m = NewFloatMPSReader(nil)
	m.Messages = &bytes.Buffer{}
	m.line = lpMPSBoundLine("UP", "x broken")
	b := &floatMPSBound{}
	if err := m.updateBound(b, []string{"x", "broken"}); err == nil || !b.upperSet || b.upper != 0 {
		t.Fatal(err, b)
	}
	if v, err := mpsNumber("1e999 trailing"); err != nil || !math.IsInf(v, 1) {
		t.Fatal(v, err)
	}
	if _, err := mpsNumber(""); err != ErrBounds {
		t.Fatal(err)
	}
	wantErr := errors.New("MPS writer refused")
	for n := 0; n < 3; n++ {
		m = NewFloatMPSReader(nil)
		w := &lpValidationWriter{allowed: n, err: wantErr}
		m.Messages = w
		m.line = "bad"
		if err := m.cannotUnderstand(); err != wantErr {
			t.Fatal(n, err)
		}
		if m.IsOK() != (n == 0) {
			t.Fatal("flag prefix", n, m.IsOK())
		}
	}
	m = NewFloatMPSReader(nil)
	m.Messages = nil
	if err := m.Read(); err != ErrBounds || !m.IsOK() {
		t.Fatal(err)
	}
	if err := m.SetMessageStream(nil); sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	lpStaticMust(t, m.SetMessageStream(io.Discard))
}
func TestLPFloatMPSFileReadFailure(t *testing.T) {
	// A real closed file fails through bufio after the source failure diagnostics.
	f, err := os.CreateTemp(t.TempDir(), "mps")
	lpStaticMust(t, err)
	lpStaticMust(t, f.Close())
	m := NewFloatMPSReader(f)
	m.Messages = &bytes.Buffer{}
	if err := m.readLine(); err == nil || m.IsOK() || m.LineNumber != 1 {
		t.Fatal(err, m.IsOK(), m.LineNumber)
	}
}

// Writer refusal occurs at actual reached diagnostics, with each earlier
// write accepted. No solver or parser callback replaces source computation.
func TestLPFloatMPSDiagnosticRefusalCampaigns(t *testing.T) {
	base := "NAME T\nROWS\n N COST\n L R\nCOLUMNS\n" + lpMPSPair("x", "COST", 1) + lpMPSPair("x", "R", 1) + "RHS\n" + lpMPSPair("RHS1", "R", 8)
	inputs := []string{
		"", "NAME T\nROWS\nCOLUMNS\ntiny\n",
		"NAME T\nROWS\nCOLUMNS\nRHS\ntiny\n",
		base + "BOUNDS\n" + lpMPSBoundLine("ZZ", "x 1") + "ENDATA\n",
		base + "BOUNDS\n" + lpMPSBoundLine("UP", "") + "ENDATA\n",
		base + "BOUNDS\n" + lpMPSBoundLine("UP", "missing 1") + "ENDATA\n",
		base + "RANGES\n" + lpMPSPair("RNG", "COST", 1) + "ENDATA\n",
		"NAME T\nROWS\n N C0\n N COST\nCOLUMNS\n" + lpMPSPair("x", "COST", 1) + "RHS\nENDATA\n",
		"NAME T\nROWS\n N COST\nCOLUMNS\n" + lpMPSPair("x", "MISSING", 1) + "RHS\nENDATA\n",
	}
	wantErr := errors.New("MPS diagnostic refusal")
	for fixture, input := range inputs {
		reachedEnd := false
		for accepted := 0; accepted < 30; accepted++ {
			m := NewFloatMPSReader(strings.NewReader(input))
			w := &lpValidationWriter{allowed: accepted, err: wantErr}
			m.Messages = w
			err := m.Read()
			if err == nil && m.IsOK() {
				solver, createErr := m.CreatePrimalSolver()
				if solver != nil {
					solver.Close()
				}
				err = createErr
			}
			if err == wantErr {
				continue
			}
			if w.allowed != accepted {
				reachedEnd = true
				break
			}
			t.Fatal("fixture reached no diagnostics", fixture, err)
		}
		if !reachedEnd {
			t.Fatal("no completed diagnostic sequence", fixture)
		}
	}
}

func TestLPFloatMPSSourceDomainPrefixes(t *testing.T) {
	m := NewFloatMPSReader(nil)
	m.Messages = io.Discard
	m.line = ""
	lpStaticMust(t, m.addRow())
	m.line = "  "
	if err := m.addRow(); err != ErrBounds {
		t.Fatal(err)
	}
	m.line = " Z ignored"
	lpStaticMust(t, m.addRow())
	if m.rows.Len() != 0 {
		t.Fatal(m.rows.Len())
	}
	m.line = " N COST"
	lpStaticMust(t, m.addRow())
	// Empty trailing RHS/column data reaches the source unsigned-size guard.
	if err := m.rowPairs("", "x", 0, mpsRHS); err != ErrBounds {
		t.Fatal(err)
	}
	row := newFloatMPSRow("R", 0, mpsLess)
	if err := m.assignRow(row, "x", "not-a-number", mpsCoefficient); err == nil || row.coefficients.Len() != 0 {
		t.Fatal(err)
	}
	// Range clone publication precedes the source missing-column assertion.
	row.coefficients.Set("missing", 2)
	row.span = 1
	err := m.expandRange(row)
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	if _, ok := m.rows.Get2("R_range"); !ok {
		t.Fatal("range owner not published")
	}
	// A missing solver column publishes source operator[] only in release.
	m = NewFloatMPSReader(nil)
	m.Messages = io.Discard
	m.rows.Set("R", row)
	solver, err := m.CreatePrimalSolver()
	if solver == nil {
		t.Fatal("partial solver missing")
	}
	defer solver.Close()
	if sourceDebugEnabled {
		if err != ErrAssertion || m.columns.Len() != 0 {
			t.Fatal(err)
		}
	} else if err != ErrBounds || m.columns.Len() != 1 {
		t.Fatal(err)
	}
	m.columns.Set("missing", nil)
	if _, err := m.CreatePrimalSolver(); err != ErrBounds {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = io.Discard
	m.columns.Set("x", nil)
	if _, err := m.CreatePrimalSolver(); err != ErrBounds {
		t.Fatal(err)
	}
	// Duplicate row indices preserve add_constraint's debug assertion prefix.
	m = NewFloatMPSReader(nil)
	m.Messages = io.Discard
	m.rows.Set("R0", newFloatMPSRow("R0", 0, mpsLess))
	m.rows.Set("R1", newFloatMPSRow("R1", 0, mpsGreater))
	solver, err = m.CreatePrimalSolver()
	if solver != nil {
		solver.Close()
	}
	if sourceDebugEnabled && err != ErrAssertion || !sourceDebugEnabled && err != nil {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = io.Discard
	m.line = ""
	if err := m.updateBound(&floatMPSBound{}, nil); err != ErrBounds {
		t.Fatal(err)
	}
	m.line = "              "
	if err := m.rowPairs("RNG unknown 2", "", 1, mpsRange); err != nil || m.IsOK() {
		t.Fatal(err)
	}
	m = NewFloatMPSReader(nil)
	m.Messages = io.Discard
	m.line = "short"
	lpStaticMust(t, m.rowPairs("RNG unknown 2", "", 1, mpsRange))
	if m.IsOK() {
		t.Fatal("range fallback short-line failure missing")
	}
}

func TestLPFloatMPSUnrangedRelationsAndNumericPrefixes(t *testing.T) {
	text := "NAME EQGE\nROWS\n N COST\n E E0\n G G0\nCOLUMNS\n" +
		lpMPSPair("x", "COST", 3) + lpMPSPair("x", "E0", 1) + lpMPSPair("x", "G0", 1) +
		lpMPSPair("y", "COST", 2) + lpMPSPair("y", "G0", 1) + "RHS\n" +
		lpMPSPair("RHS1", "E0", 2) + lpMPSPair("RHS1", "G0", 3) + "BOUNDS\n" + lpMPSBoundLine("UP", "y 5") + "ENDATA\n"
	reader := lpMPSRead(t, text)
	solver, err := reader.CreatePrimalSolver()
	lpStaticMust(t, err)
	defer solver.Close()
	if solver.Constraints[0].Relation != RelationEqual || solver.Constraints[1].Relation != RelationGreaterOrEqual {
		t.Fatal(solver.Constraints)
	}
	solver.Settings.SimplexStrategy = StrategyLU
	solver.Settings.DebugOut = nil
	solver.Settings.MessageOut = nil
	lpStaticMust(t, solver.FindMaximalSolution())
	cost, err := solver.GetCurrentCost()
	lpStaticMust(t, err)
	if solver.Status != StatusOptimal || math.Abs(cost-16) > 1e-8 {
		t.Fatal(solver.Status, cost)
	}
	// Malformed native numeric input publishes earlier source coefficients only.
	reader = NewFloatMPSReader(nil)
	reader.Messages = io.Discard
	r := newFloatMPSRow("R", 0, mpsLess)
	reader.rows.Set("R", r)
	if err := reader.rowPairs("R 2 R invalid", "x", 0, mpsCoefficient); err == nil || r.coefficients.Get("x") != 2 {
		t.Fatal(err)
	}
	reader.line = fmt.Sprintf("%-8s%14s", "R", "invalid")
	if err := reader.fixedRowPairs(reader.line, "x", mpsCoefficient); err == nil || r.coefficients.Get("x") != 2 {
		t.Fatal(err)
	}
	for _, span := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		reader = NewFloatMPSReader(nil)
		reader.Messages = io.Discard
		r = newFloatMPSRow("R", 0, mpsEqual)
		r.span = span
		lpStaticMust(t, reader.expandRange(r))
		other, _ := reader.rows.Get2("R_range")
		if math.IsNaN(span) {
			if r.typ != mpsLess || other.typ != mpsGreater || !math.IsNaN(other.rhs) {
				t.Fatal(r, other)
			}
		} else if other.rhs != span {
			t.Fatal(span, other.rhs)
		}
	}
}
