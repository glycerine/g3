package lp

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func lpModelReportSolvedPrimal(t *testing.T) *FloatPrimalSimplex {
	t.Helper()
	m := lpFloatPrimal(t)
	t.Cleanup(func() { lpStaticMust(t, m.Close()) })
	m.Settings.SimplexStrategy = StrategyLU
	m.Settings.UseScaling = true
	m.Settings.DebugOut = nil
	for j, name := range []string{"x", "y"} {
		m.GiveSymbolicNameToColumn(name, uint32(j))
		m.Columns[uint32(j)].SetLowBound(0)
		m.Columns[uint32(j)].Cost = float64(3 - j)
		m.SetRowColumnCoefficient(0, uint32(j), 1)
		m.SetRowColumnCoefficient(uint32(j+1), uint32(j), 1)
	}
	for i, rhs := range []float64{4, 2, 3} {
		lpStaticMust(t, m.AddConstraint(RelationLessOrEqual, rhs, uint32(i)))
	}
	m.GiveSymbolicNameToColumn("fixed", 99)
	m.Columns[99].SetFixedValue(1.5)
	m.Columns[99].Cost = 4
	// Source reports the name map, including aliases retained after renaming.
	m.GiveSymbolicNameToColumn("alias", 0)
	lpStaticMust(t, m.FindMaximalSolution())
	if m.Status != StatusOptimal {
		t.Fatal(m.Status)
	}
	return m
}

func TestLPFloatModelReportSolvedOriginalValues(t *testing.T) {
	m := lpModelReportSolvedPrimal(t)
	// Names print in libstdc++ unordered_map order (alias fixed y x for
	// inserts x y fixed alias, from g++ 13.3; AUDIT.md util_lp_06-F1).
	// The vertex (2,2) uniquely maximizes 3*x+2*y subject to x+y<=4,
	// x<=2, y<=3 and nonnegative bounds. The fixed term adds six.
	want := "objective = 16\ncolumn values\nalias = 2\nfixed = 1.5\ny = 2\nx = 2\n"
	w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: len(want)}}
	lpStaticMust(t, m.PrintModel(w))
	if w.output.String() != want || w.flushes != 5 {
		t.Fatal(w.output.String(), w.flushes)
	}
}

func TestLPFloatModelReportEveryByteAndEndl(t *testing.T) {
	m := lpModelReportSolvedPrimal(t)
	report := "objective = 16\ncolumn values\nalias = 2\nfixed = 1.5\ny = 2\nx = 2\n"
	ends := []int{len("objective = 16\n"), len("objective = 16\ncolumn values\nalias = 2\n"), len("objective = 16\ncolumn values\nalias = 2\nfixed = 1.5\n"), len("objective = 16\ncolumn values\nalias = 2\nfixed = 1.5\ny = 2\n"), len(report)}
	refused := errors.New("model report refused")
	for _, writeErr := range []error{nil, refused} {
		for limit := 0; limit < len(report); limit++ {
			w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: limit, err: writeErr}}
			want := writeErr
			if want == nil {
				want = io.ErrShortWrite
			}
			flushes := 0
			for _, end := range ends {
				if end <= limit {
					flushes++
				}
			}
			if err := m.PrintModel(w); err != want || w.output.String() != report[:limit] || w.flushes != flushes {
				t.Fatal(limit, err, w.output.String(), w.flushes, flushes)
			}
		}
	}
	for i, end := range ends {
		w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: len(report)}, failAt: i + 1}
		if err := m.PrintModel(w); err != lpErrResidualWrite || w.output.String() != report[:end] || w.flushes != i+1 {
			t.Fatal(i, err, w.output.String(), w.flushes)
		}
	}
}

func TestLPFloatModelReportGetterFailurePrefixes(t *testing.T) {
	// The objective insertion precedes the virtual getter, even when it fails.
	m := lpFloatPrimal(t)
	m.Columns[0] = nil
	w := &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: 1000}}
	if err := m.PrintModel(w); err != ErrBounds || w.output.String() != "objective = " || w.flushes != 0 {
		t.Fatal(err, w.output.String(), w.flushes)
	}
	// A name can refer to a missing column without affecting the objective.
	// The source getter reports zero without inserting column info.
	m = lpFloatPrimal(t)
	m.NamesToColumns.Put("missing", 123)
	prefix := "objective = 0\ncolumn values\nmissing = "
	w = &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: len(prefix) - 1}}
	if err := m.PrintModel(w); err != io.ErrShortWrite || len(m.Columns) != 0 {
		t.Fatal(err, m.Columns)
	}
	w = &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: 1000}}
	if err := m.PrintModel(w); err != nil || w.output.String() != prefix+"0\n" || w.flushes != 2 || len(m.Columns) != 0 {
		t.Fatal(err, w.output.String(), w.flushes)
	}
	// An unnamed empty model has the objective endl only, not a heading flush.
	m = lpFloatPrimal(t)
	w = &lpValidationPrefixWriter{larConstraintPrefixWriter: larConstraintPrefixWriter{remaining: 1000}}
	lpStaticMust(t, m.PrintModel(w))
	if w.output.String() != "objective = 0\ncolumn values\n" || w.flushes != 1 {
		t.Fatal(w.output.String(), w.flushes)
	}
}

// TestLPFloatModelReportLibstdcxxNameOrder: AUDIT.md util_lp_06-F1.
// mps_reader fills m_columns in read order (XONE YTWO ZTHREE ALPHA BETA
// GAMMA); fill_solver_on_columns walks m_columns and names the solver's
// columns in that libstdc++ order, and print_model walks
// m_names_to_columns. Both orders come from a g++ 13.3 program inserting
// the same names into std::unordered_map<std::string, unsigned>.
func TestLPFloatModelReportLibstdcxxNameOrder(t *testing.T) {
	reader := NewFloatMPSReader(nil)
	for _, name := range []string{"XONE", "YTWO", "ZTHREE", "ALPHA", "BETA", "GAMMA"} {
		reader.columns.Set(name, &floatMPSColumn{name: name, index: uint32(reader.columns.Len())})
	}
	if got, want := reader.ColumnNames(), []string{"ALPHA", "ZTHREE", "GAMMA", "YTWO", "BETA", "XONE"}; !reflect.DeepEqual(got, want) {
		t.Fatal("m_columns order", got, want)
	}
	m := lpFloatPrimal(t)
	t.Cleanup(func() { lpStaticMust(t, m.Close()) })
	for name, col := range reader.columns.All() {
		m.GiveSymbolicNameToColumn(name, col.index)
	}
	if got, want := m.NamesToColumns.Keys(), []string{"XONE", "BETA", "YTWO", "GAMMA", "ZTHREE", "ALPHA"}; !reflect.DeepEqual(got, want) {
		t.Fatal("m_names_to_columns order", got, want)
	}
	var out strings.Builder
	lpStaticMust(t, m.PrintModel(&out))
	if want := "objective = 0\ncolumn values\nXONE = 0\nBETA = 0\nYTWO = 0\nGAMMA = 0\nZTHREE = 0\nALPHA = 0\n"; out.String() != want {
		t.Fatalf("report\n%s\nwant\n%s", out.String(), want)
	}
}
