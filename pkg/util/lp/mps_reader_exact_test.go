package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func lpExactMPSPair(col, row, value string) string {
	return fmt.Sprintf("    %-8s  %-8s  %s\n", col, row, value)
}
func lpReadExactMPS(t *testing.T, h *lpPairHarness, text string) *RationalMPSReader {
	t.Helper()
	r := NewRationalMPSReader(h.m, strings.NewReader(text))
	r.Messages = &bytes.Buffer{}
	h.must(r.Read())
	if !r.IsOK() {
		t.Fatal(r.line, r.LineNumber, r.Messages)
	}
	return r
}
func TestLPRationalMPSIndependentExactModels(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 100; world++ {
		h := newLPPairHarness(t)
		var input strings.Builder
		input.WriteString("* comment\nNAME EXACT\n \t\nROWS\n N COST\n L L\n G G\n E E\nCOLUMNS\n")
		coeff := big.NewRat(int64(world*19-712), 17)
		// Values beyond binary64's exact integer range must remain exact.
		huge := new(big.Rat).SetInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(80+world)), big.NewInt(1)))
		for _, col := range []string{"x", "y"} {
			for _, row := range []string{"COST", "L", "G", "E"} {
				input.WriteString(lpExactMPSPair(col, row, coeff.RatString()))
			}
		}
		input.WriteString(lpExactMPSPair("x", "L", huge.RatString()))
		input.WriteString("RHS\n")
		rhs := big.NewRat(int64(world-41), 13)
		for _, row := range []string{"L", "G", "E"} {
			input.WriteString(lpExactMPSPair("RHS1", row, rhs.RatString()))
		}
		input.WriteString("RANGES\n")
		span := big.NewRat(int64(1+world%23), 19)
		if world%2 == 0 {
			span.Neg(span)
		}
		for _, row := range []string{"L", "G", "E"} {
			input.WriteString(lpExactMPSPair("RNG1", row, span.RatString()))
		}
		input.WriteString("ENDATA\n")
		r := lpReadExactMPS(t, h, input.String())
		if r.Name != "EXACT" || !reflect.DeepEqual(r.ColumnNames(), []string{"y", "x"}) || r.rows.Len() != 7 { // libstdc++ m_columns order
			t.Fatal(r.Name, r.ColumnNames(), r.rows.Len())
		}
		abs := new(big.Rat).Abs(span)
		for _, name := range []string{"L", "G", "E"} {
			row, _ := r.rows.Get2(name)
			other, _ := r.rows.Get2(name + "_range")
			lpSparseRationalCheck(t, h, row.rhs, rhs)
			lpSparseRationalCheck(t, h, row.span, span)
			want := new(big.Rat).Add(rhs, abs)
			wantType := mpsLess
			if name == "L" {
				want.Sub(rhs, abs)
				wantType = mpsGreater
			}
			if name == "E" {
				want.Add(rhs, span)
				if span.Sign() <= 0 {
					wantType = mpsGreater
				}
			}
			if other.typ != wantType || other.index < 3 {
				t.Fatal(name, other.typ, other.index)
			}
			lpSparseRationalCheck(t, h, other.rhs, want)
			for _, col := range []string{"x", "y"} {
				expected := coeff
				if name == "L" && col == "x" {
					expected = huge
				}
				a, _ := row.coefficients.Get2(col)
				b, _ := other.coefficients.Get2(col)
				if a == b {
					t.Fatal("range coefficients must own independent numeral copies")
				}
				lpSparseRationalCheck(t, h, a, expected)
				lpSparseRationalCheck(t, h, b, expected)
			}
		}
		h.must(r.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}
func TestLPRationalMPSBoundTypes(t *testing.T) {
	for _, typ := range []string{"LO", "LI", "UP", "UI", "FX", "FR", "PL", "MI", "BV", "LIMITMAX", "BAD"} {
		t.Run(typ, func(t *testing.T) {
			h := newLPPairHarness(t)
			text := "NAME B\nROWS\n N COST\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + "RHS\nBOUNDS\n" + lpMPSBoundLine(typ, "x 12345678901234567890123456789/17") + "ENDATA\n"
			r := NewRationalMPSReader(h.m, strings.NewReader(text))
			r.Messages = &bytes.Buffer{}
			err := r.Read()
			if typ == "LIMITMAX" && sourceDebugEnabled {
				if err != ErrAssertion {
					t.Fatal(err)
				}
			} else if typ == "BAD" {
				if err != ErrMPSUnhandled || r.IsOK() {
					t.Fatal(err, r.IsOK())
				}
			} else {
				h.must(err)
				col, _ := r.columns.Get2("x")
				b := col.bound
				if b == nil || !b.lowSet {
					t.Fatal("source default lower flag")
				}
				want := mustLPRat(t, "12345678901234567890123456789/17")
				switch typ {
				case "LO", "LI":
					lpSparseRationalCheck(t, h, b.low, want)
				case "UP", "UI", "LIMITMAX":
					if !b.upperSet {
						t.Fatal("upper flag")
					}
					lpSparseRationalCheck(t, h, b.upper, want)
				case "FX":
					if !b.fixedSet {
						t.Fatal("fixed flag")
					}
					lpSparseRationalCheck(t, h, b.fixed, want)
				case "FR":
					if !b.free {
						t.Fatal("free flag")
					}
				case "PL":
					lpSparseRationalCheck(t, h, b.low, new(big.Rat))
				case "MI":
					if !b.upperSet {
						t.Fatal("MI upper flag")
					}
					lpSparseRationalCheck(t, h, b.upper, new(big.Rat))
				case "BV":
					if !b.upperSet {
						t.Fatal("BV upper flag")
					}
					lpSparseRationalCheck(t, h, b.upper, big.NewRat(1, 1))
				}
			}
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}
func TestLPRationalMPSExactTokenSemantics(t *testing.T) {
	for _, token := range []string{"0.1", "-0.125", "3e-7", "1.5e3", "9007199254740993", "7/19", "1/0", "1/2e3", "1e4294967296"} {
		t.Run(token, func(t *testing.T) {
			h := newLPPairHarness(t)
			r := NewRationalMPSReader(h.m, strings.NewReader("NAME T\nROWS\n L R\nCOLUMNS\n"+lpExactMPSPair("x", "R", token)+"RHS\nENDATA\n"))
			r.Messages = &bytes.Buffer{}
			err := r.Read()
			switch token {
			case "1/0":
				if !errors.Is(err, numeral.ErrDivideByZero) {
					t.Fatal(err)
				}
			case "1/2e3":
				if !errors.Is(err, numeral.ErrMixedNotation) {
					t.Fatal(err)
				}
			case "1e4294967296":
				if !errors.Is(err, numeral.ErrExponentTooLarge) {
					t.Fatal(err)
				}
			default:
				h.must(err)
				row, _ := r.rows.Get2("R")
				v, _ := row.coefficients.Get2("x")
				want, ok := new(big.Rat).SetString(token)
				if !ok {
					t.Fatal(token)
				}
				lpSparseRationalCheck(t, h, v, want)
			}
			if err != nil {
				if !r.IsOK() || r.work == nil {
					t.Fatal("numeric exception must preserve source parser prefix")
				}
				if err := r.Read(); err != ErrExactVectorPending {
					t.Fatal("read after stopped conversion", err)
				}
			}
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}
func TestLPRationalMPSLARPopulationSourcePaths(t *testing.T) {
	for _, mode := range []string{"unbounded-cost-only", "free-cost-only", "bounded-cost-only", "row", "row-less", "row-equal", "old-cost-row", "not-ok"} {
		t.Run(mode, func(t *testing.T) {
			h := newLPPairHarness(t)
			var input strings.Builder
			input.WriteString("NAME P\nROWS\n")
			if mode == "old-cost-row" {
				input.WriteString(" N OLD\n")
			}
			input.WriteString(" N COST\n")
			if strings.HasPrefix(mode, "row") {
				kind := "G"
				if mode == "row-less" {
					kind = "L"
				}
				if mode == "row-equal" {
					kind = "E"
				}
				input.WriteString(" " + kind + " R\n")
			}
			input.WriteString("COLUMNS\n")
			for _, col := range []string{"x", "y"} {
				input.WriteString(lpExactMPSPair(col, "COST", "1"))
				if strings.HasPrefix(mode, "row") {
					input.WriteString(lpExactMPSPair(col, "R", "123456789012345678901/17"))
				}
			}
			input.WriteString("RHS\n")
			if strings.HasPrefix(mode, "row") {
				input.WriteString(lpExactMPSPair("RHS1", "R", "-7/13"))
			}
			if mode == "free-cost-only" || mode == "bounded-cost-only" {
				input.WriteString("BOUNDS\n")
				typ := "FR"
				if mode == "bounded-cost-only" {
					typ = "UP"
				}
				for _, col := range []string{"x", "y"} {
					input.WriteString(lpMPSBoundLine(typ, col+" 3/7"))
				}
			}
			input.WriteString("ENDATA\n")
			r := lpReadExactMPS(t, h, input.String())
			if mode == "not-ok" {
				h.must(r.fail())
			}
			s, err := r.CreateLARSolver()
			want := error(nil)
			if mode == "old-cost-row" {
				want = ErrMPSUnhandled
			} else if sourceDebugEnabled && (mode == "bounded-cost-only" || strings.HasPrefix(mode, "row")) {
				want = ErrAssertion
			}
			if err != want || s == nil {
				t.Fatal(err, want, s)
			}
			if s.NumberOfConstraints() != 0 {
				t.Fatal("source native no-op installed constraints")
			}
			expected := 1
			if mode == "free-cost-only" || strings.HasPrefix(mode, "row") || mode == "bounded-cost-only" && !sourceDebugEnabled {
				expected = 2
			}
			if len(s.ColumnsToExternal) != expected {
				t.Fatal("column-pass absent-bound early return", s.ColumnsToExternal, expected)
			}
			if err == nil {
				status, e := s.Solve()
				h.must(e)
				if status != StatusOptimal {
					t.Fatal(status)
				}
			} else if _, e := r.CreateLARSolver(); e != ErrExactVectorPending {
				t.Fatal(e)
			}
			h.must(s.Close())
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}
func TestLPRationalMPSFileAndEntryLifetime(t *testing.T) {
	h := newLPPairHarness(t)
	r := OpenRationalMPSReader(h.m, filepath.Join("test_vectors", "mps_small.mps"))
	r.Messages = &bytes.Buffer{}
	h.must(r.Read())
	if !r.IsOK() || r.file == nil {
		t.Fatal(r.Messages)
	}
	h.must(r.Close())
	if r.file != nil || r.Read() != ErrExactVectorPending {
		t.Fatal("closed file reader")
	}
	missing := OpenRationalMPSReader(h.m, filepath.Join(t.TempDir(), "absent.mps"))
	missing.Messages = &bytes.Buffer{}
	if !missing.IsOK() {
		t.Fatal("source delayed open failure")
	}
	h.must(missing.Read())
	if missing.IsOK() {
		t.Fatal("missing file")
	}
	h.must(missing.Close())
	nilOwner := NewRationalMPSReader(nil, strings.NewReader(""))
	if nilOwner.Read() != ErrExactVectorOwner {
		t.Fatal("nil owner")
	}
	h.must(nilOwner.Close())
	nilStream := NewRationalMPSReader(h.m, nil)
	nilStream.Messages = nil
	if nilStream.Read() != ErrBounds {
		t.Fatal("nil diagnostic stream")
	}
	h.must(nilStream.Close())
	stream := NewRationalMPSReader(h.m, nil)
	err := stream.SetMessageStream(nil)
	if err != sourceAssertionFailure() {
		t.Fatal(err)
	}
	h.must(stream.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPRationalMPSPhysicalReadOwnership(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, phase := range []string{"coefficient", "replace", "bound", "range", "range-less", "range-equal"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 1200 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("%v/%s/%d", direct, phase, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					large := strings.Repeat("1234567890", 5) + "7/13"
					coeff := "1"
					if phase == "coefficient" {
						coeff = large
					}
					rowKind := "G"
					if phase == "range-less" {
						rowKind = "L"
					}
					if phase == "range-equal" {
						rowKind = "E"
					}
					text := "NAME F\nROWS\n " + rowKind + " R\nCOLUMNS\n" + lpExactMPSPair("x", "R", coeff)
					if phase == "replace" {
						text += lpExactMPSPair("x", "R", large)
					}
					text += "RHS\n" + lpExactMPSPair("RHS1", "R", "-7/19")
					if phase == "bound" {
						text += "BOUNDS\n" + lpMPSBoundLine("UP", "x "+large)
					}
					if strings.HasPrefix(phase, "range") {
						if phase != "range" {
							large = "-" + large
						}
						text += "RANGES\n" + lpExactMPSPair("RNG1", "R", large)
					}
					text += "ENDATA\n"
					r := NewRationalMPSReader(h.m, strings.NewReader(text))
					r.Messages = io.Discard
					h.budget.FailAfter(ordinal)
					err := r.Read()
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
						if !r.IsOK() {
							t.Fatal("successful read not OK")
						}
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) {
							t.Fatal(err)
						}
						if r.Read() != ErrExactVectorPending {
							t.Fatal("retry after unfinished numeric parse")
						}
					}
					h.must(r.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("incomplete physical campaign", direct, phase, done, failures)
			}
		}
	}
}

func TestLPRationalMPSFixedFieldsAndRepeatedPublication(t *testing.T) {
	h := newLPPairHarness(t)
	r := NewRationalMPSReader(h.m, nil)
	r.Messages = io.Discard
	a, b := r.newRow("R A", 0, mpsLess), r.newRow("R B", 1, mpsGreater)
	r.rows.Set(a.name, a)
	r.rows.Set(b.name, b)
	r.columns.Set("X A", &rationalMPSColumn{name: "X A"})
	data := fmt.Sprintf("%-8s%14s   %-8s%14s", "R A", "2", "R B", "3")
	r.line = "              " + data
	for _, field := range []mpsRowField{mpsCoefficient, mpsRHS, mpsRange} {
		text, start := data, 0
		if field == mpsRange {
			text, start = r.line, 1
		}
		h.must(r.rowPairs(text, "X A", start, field))
	}
	// Source rational(char*) scans digits across trailing text. Unlike atof,
	// its first fixed-field suffix includes the second pair and yields23.
	lpSparseRationalCheck(t, h, a.coefficients.Get("X A"), big.NewRat(23, 1))
	lpSparseRationalCheck(t, h, b.coefficients.Get("X A"), big.NewRat(3, 1))
	lpSparseRationalCheck(t, h, a.rhs, big.NewRat(23, 1))
	lpSparseRationalCheck(t, h, a.span, big.NewRat(23, 1))
	r.line = " UP BND       " + fmt.Sprintf("%-8s%6s%8s", "X A", "", "4")
	h.must(r.createBound())
	col, _ := r.columns.Get2("X A")
	lpSparseRationalCheck(t, h, col.bound.upper, big.NewRat(4, 1))
	h.must(r.expandRange(a))
	other, _ := r.rows.Get2("R A_range")
	if other.index != uint32(r.rows.Len()) {
		t.Fatal(other.index, r.rows.Len())
	}
	h.must(r.Close())
	text := "NAME D\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + lpExactMPSPair("", "COST", "3") + lpExactMPSPair("", "R", "2") + "    MARK      'MARKER' 'INTORG'\nRHS\n" + lpExactMPSPair("RHS1", "R", "7") + "ENDATA\n"
	r = lpReadExactMPS(t, h, text)
	cost, _ := r.rows.Get2("COST")
	lpSparseRationalCheck(t, h, cost.coefficients.Get("x"), big.NewRat(3, 1))
	if !reflect.DeepEqual(r.ColumnNames(), []string{"MARK", "x"}) { // libstdc++ m_columns order
		t.Fatal(r.ColumnNames())
	}
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
func TestLPRationalMPSBoundsAndRangeSectionOrders(t *testing.T) {
	for _, kind := range []string{"L", "G", "E"} {
		for _, span := range []string{"-3/7", "0", "3/7"} {
			for _, boundsFirst := range []bool{false, true} {
				h := newLPPairHarness(t)
				text := "NAME R\nROWS\n " + kind + " R\nCOLUMNS\n" + lpExactMPSPair("x", "R", "2/5") + "RHS\n" + lpExactMPSPair("RHS1", "R", "8/11")
				bounds := "BOUNDS\n" + lpMPSBoundLine("UP", "x 9/13")
				ranges := "RANGES\n" + lpExactMPSPair("RNG1", "R", span)
				if boundsFirst {
					text += bounds + ranges
				} else {
					text += ranges + bounds
				}
				text += "ENDATA\n"
				r := lpReadExactMPS(t, h, text)
				other, _ := r.rows.Get2("R_range")
				delta := mustLPRat(t, span)
				rhs := big.NewRat(8, 11)
				if kind == "L" {
					rhs.Sub(rhs, new(big.Rat).Abs(delta))
				} else if kind == "G" {
					rhs.Add(rhs, new(big.Rat).Abs(delta))
				} else {
					rhs.Add(rhs, delta)
				}
				lpSparseRationalCheck(t, h, other.rhs, rhs)
				col, _ := r.columns.Get2("x")
				lpSparseRationalCheck(t, h, col.bound.upper, big.NewRat(9, 13))
				h.must(r.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}
		}
	}
}
func TestLPRationalMPSMalformedDiagnosticPrefixes(t *testing.T) {
	base := "NAME T\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + lpExactMPSPair("x", "R", "1") + "RHS\n" + lpExactMPSPair("RHS1", "R", "8")
	inputs := []string{"", "NAME T\nROWS\nCOLUMNS\ntiny\n", "NAME T\nROWS\nCOLUMNS\nRHS\ntiny\n", base + "BOUNDS\n" + lpMPSBoundLine("ZZ", "x 1") + "ENDATA\n", base + "BOUNDS\n" + lpMPSBoundLine("UP", "") + "ENDATA\n", base + "BOUNDS\n" + lpMPSBoundLine("UP", "missing 1") + "ENDATA\n", base + "RANGES\n" + lpExactMPSPair("RNG", "COST", "1") + "ENDATA\n", "NAME T\nROWS\n N OLD\n N COST\nCOLUMNS\n" + lpExactMPSPair("x", "COST", "1") + "RHS\nENDATA\n", "NAME T\nROWS\n N COST\nCOLUMNS\n" + lpExactMPSPair("x", "MISSING", "1") + "RHS\nENDATA\n"}
	refused := errors.New("exact MPS diagnostic refused")
	for fixture, text := range inputs {
		completed := false
		for accepted := 0; accepted < 30 && !completed; accepted++ {
			h := newLPPairHarness(t)
			r := NewRationalMPSReader(h.m, strings.NewReader(text))
			w := &lpValidationWriter{allowed: accepted, err: refused}
			r.Messages = w
			err := r.Read()
			if err == nil && r.IsOK() {
				s, e := r.CreateLARSolver()
				if s != nil {
					h.must(s.Close())
				}
				err = e
			}
			if err != refused {
				if w.allowed == accepted {
					t.Fatal("fixture reached no diagnostics", fixture, err)
				}
				completed = true
			}
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !completed {
			t.Fatal("diagnostic sequence did not terminate", fixture)
		}
	}
	h := newLPPairHarness(t)
	stalled := NewRationalMPSReader(h.m, strings.NewReader("NAME S\nJUNK\nROWS\n"))
	stalled.Messages = io.Discard
	if stalled.Read() != ErrMPSStalled || !stalled.IsOK() || stalled.LineNumber != 2 {
		t.Fatal("source nonadvancing header")
	}
	h.must(stalled.Close())
	for _, phase := range []string{"empty-tokens", "empty-row", "blank-row", "unknown-row", "short-bound", "empty-bound", "short-range", "missing-range", "missing-value", "empty-bound-type", "unsupported-relation", "range-missing-column", "nil-column"} {
		r := NewRationalMPSReader(h.m, nil)
		r.Messages = io.Discard
		var err, want error
		switch phase {
		case "empty-tokens":
			err = r.rowPairs("", "x", 0, mpsCoefficient)
			want = ErrBounds
		case "empty-row":
			err = r.addRow()
		case "blank-row":
			r.line = "  "
			err = r.addRow()
			want = ErrBounds
		case "unknown-row":
			r.line = " Z ignored"
			err = r.addRow()
			if r.rows.Len() != 0 {
				t.Fatal("unknown row installed")
			}
		case "short-bound":
			r.line = "short"
			err = r.createBound()
			want = ErrBounds
			if sourceDebugEnabled {
				want = ErrAssertion
			}
		case "empty-bound":
			r.line = strings.Repeat(" ", 14)
			err = r.createBound()
			want = ErrMPSUnhandled
		case "short-range":
			r.line = "short"
			err = r.fixedRowPairs("", "", mpsRange)
			if r.IsOK() {
				t.Fatal("short range")
			}
		case "missing-range":
			r.line = "short"
			err = r.rowPairs("RNG unknown 2", "", 1, mpsRange)
			if r.IsOK() {
				t.Fatal("missing range")
			}
		case "missing-value":
			r.line = lpMPSBoundLine("UP", "x")
			err = r.updateBound(r.newBound(), []string{"x"})
			if r.IsOK() {
				t.Fatal("missing value")
			}
		case "empty-bound-type":
			err = r.updateBound(r.newBound(), nil)
			want = ErrBounds
		case "unsupported-relation":
			_, err = r.larRelation(mpsCost)
			want = ErrMPSUnhandled
		case "range-missing-column":
			row := r.newRow("R", 0, mpsLess)
			v := r.newValue()
			h.must(r.setSmall(v, 2))
			row.coefficients.Set("missing", v)
			h.must(r.setSmall(row.span, 1))
			err = r.expandRange(row)
			if sourceDebugEnabled {
				want = ErrAssertion
			}
			if _, ok := r.rows.Get2("R_range"); !ok {
				t.Fatal("range prefix")
			}
		case "nil-column":
			r.columns.Set("x", nil)
			s, e := r.CreateLARSolver()
			err = e
			want = ErrBounds
			h.must(s.Close())
		}
		if err != want {
			t.Fatal(phase, err, want)
		}
		h.must(r.Close())
	}
	lpExactVectorNamedCensus(t, h, nil)
}
func TestLPRationalMPSPhysicalPopulationOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		done, failures := false, 0
		for ordinal := int64(0); ordinal < 200 && !done; ordinal++ {
			if !t.Run(fmt.Sprintf("%v/%d", direct, ordinal), func(t *testing.T) {
				var h *lpPairHarness
				if direct {
					h = lpNewDirectSparseHarness(t)
				} else {
					h = newLPPairHarness(t)
				}
				large := strings.Repeat("1234567890", 20) + "7/13"
				r := lpReadExactMPS(t, h, "NAME C\nROWS\n G R\nCOLUMNS\n"+lpExactMPSPair("x", "R", large)+lpExactMPSPair("y", "R", large)+"RHS\nENDATA\n")
				h.budget.FailAfter(ordinal)
				s, err := r.CreateLARSolver()
				h.budget.FailAfter(-1)
				if err == nil || sourceDebugEnabled && err == ErrAssertion {
					done = true
				} else {
					failures++
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
				}
				if s != nil {
					h.must(s.Close())
				}
				h.must(r.Close())
				lpExactVectorNamedCensus(t, h, nil)
			}) {
				t.FailNow()
			}
		}
		if !done || direct && failures == 0 {
			t.Fatal("incomplete population campaign", direct, done, failures)
		}
	}
}
func TestLPRationalMPSPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		r := NewRationalMPSReader(h.m, strings.NewReader("NAME O\nROWS\n G R\nCOLUMNS\n"+lpExactMPSPair("x", "R", "12345678901234567890123456789/17")+"RHS\nENDATA\n"))
		r.Messages = io.Discard
		v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
		var caller exactEtaCalls
		b, err := caller.begin(h.m)
		h.must(err)
		h.budget.FailAfter(ordinal)
		_, err = h.m.Compare(&v.X, &v.Y)
		h.budget.FailAfter(-1)
		err = caller.after(h.m, b, err)
		older := h.m.ComparisonInvocationState()
		if older.Active {
			matched = true
			if !lpExactPriorityPhysicalRefusal(err) {
				t.Fatal(err)
			}
			_ = r.Read()
			if r.calls.comparison != 0 {
				t.Fatal("adopted older caller comparison")
			}
		}
		h.must(r.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("consumed older caller comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}

func TestLPRationalMPSReadAndConversionFailurePrefixes(t *testing.T) {
	h := newLPPairHarness(t)
	f, err := os.CreateTemp(t.TempDir(), "mps")
	h.must(err)
	h.must(f.Close())
	r := NewRationalMPSReader(h.m, f)
	var out bytes.Buffer
	r.Messages = &out
	if r.Read() == nil || r.IsOK() || r.LineNumber != 1 || out.String() != "setting m_is_OK to false\ncannot read from file\n" {
		t.Fatal(r.IsOK(), r.LineNumber, out.String())
	}
	h.must(r.Close())
	r = NewRationalMPSReader(h.m, nil)
	r.Messages = io.Discard
	row := r.newRow("R", 0, mpsLess)
	r.rows.Set("R", row)
	if err = r.rowPairs("R 2 R 1/0", "x", 0, mpsCoefficient); !errors.Is(err, numeral.ErrDivideByZero) {
		t.Fatal(err)
	}
	lpSparseRationalCheck(t, h, row.coefficients.Get("x"), big.NewRat(2, 1))
	h.must(r.Close())
	r = NewRationalMPSReader(h.m, nil)
	r.Messages = io.Discard
	r.columns.Set("x", &rationalMPSColumn{name: "x"})
	r.line = lpMPSBoundLine("UP", "x 1/0")
	if err = r.createBound(); !errors.Is(err, numeral.ErrDivideByZero) {
		t.Fatal(err)
	}
	col, _ := r.columns.Get2("x")
	if !col.bound.upperSet {
		t.Fatal("flag precedes conversion")
	}
	lpSparseRationalCheck(t, h, col.bound.upper, new(big.Rat))
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

// Source: src/test/lp.cpp:2449-2507, create_lar_solver_from_file and the
// solve/--randomize_lar path of run_lar_solver, using a retained MPS fixture.
func TestLPRationalMPSUpstreamFileRandomizeDriver(t *testing.T) {
	h := newLPPairHarness(t)
	r := OpenRationalMPSReader(h.m, filepath.Join("test_vectors", "mps_small.mps"))
	r.Messages = io.Discard
	h.must(r.Read())
	s, err := r.CreateLARSolver()
	if err != sourceAssertionFailure() || s == nil {
		t.Fatal(err, s)
	}
	if !sourceDebugEnabled {
		status, err := s.Solve()
		h.must(err)
		if status != StatusOptimal || s.TotalIterations() != 1 || s.NumberOfConstraints() != 0 {
			t.Fatal("source file-driver solve", status, s.TotalIterations(), s.NumberOfConstraints())
		}
		vars := s.GetListOfAllVarIndices()
		if !reflect.DeepEqual(vars, []uint32{0, 1, 2}) {
			t.Fatal("driver uses internal heading indices", vars)
		}
		if len(vars) > 100 {
			vars = vars[:100]
		}
		h.must(s.RandomUpdate(vars))
		model := NewLARRationalModel(h.m)
		h.must(s.GetModel(model))
		values := make(map[string]bool)
		for _, j := range vars {
			text, err := h.m.String(model.Values[j])
			h.must(err)
			values[text] = true
		}
		if len(values) != 3 {
			t.Fatal("source randomization of the three free columns", values)
		}
		h.must(model.Close())
		vars[0] = 99
		if s.GetListOfAllVarIndices()[0] != 0 {
			t.Fatal("returned list aliases the solver")
		}
	}
	h.must(s.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARDriverQueryHeadingExtent(t *testing.T) {
	h := newLPPairHarness(t)
	s, err := NewLARSolver(h.m)
	h.must(err)
	if len(s.GetListOfAllVarIndices()) != 0 || s.TotalIterations() != 0 {
		t.Fatal("empty driver queries")
	}
	_, err = s.AddVar(17)
	h.must(err)
	term, err := s.AddTerm([]RationalSparseEntry{{Index: 0, Value: h.pair("1", "0").X}}, &h.pair("3", "0").X)
	h.must(err)
	_, err = s.AddVarBound(term, LConstraintGE, &h.pair("2", "0").X)
	h.must(err)
	if !reflect.DeepEqual(s.GetListOfAllVarIndices(), []uint32{0, 1}) {
		t.Fatal("term IDs must not be substituted into heading indices")
	}
	s.Core.RPrimal.TotalIterations = ^uint32(0)
	if s.TotalIterations() != ^uint32(0) {
		t.Fatal("unsigned source iteration counter")
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

// AUDIT.md util_lp_04-F2: the rational reader shares read_bounds.
func TestLPRationalMPSBoundsAtEOF(t *testing.T) {
	h := newLPPairHarness(t)
	text := "NAME T\nROWS\n N COST\n L R\nCOLUMNS\n" + lpExactMPSPair("X", "R", "1") + "RHS\n" + lpExactMPSPair("RHS", "R", "4") + "BOUNDS\n" + lpMPSBoundLine("UP", "X 4")
	r := NewRationalMPSReader(h.m, strings.NewReader(text))
	out := &bytes.Buffer{}
	r.Messages = out
	if err := r.Read(); err != nil || r.IsOK() {
		t.Fatal(err, r.IsOK())
	}
	if out.String() != "setting m_is_OK to false\ncannot read from file\n" {
		t.Fatalf("messages %q", out.String())
	}
}
