package lp

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
)

func TestLPRationalMPSCommonModelIndependentPopulation(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 100; world++ {
		h := newLPPairHarness(t)
		coeff := new(big.Rat).SetFrac(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), uint(81+world)), big.NewInt(3)), big.NewInt(17))
		rhs := big.NewRat(int64(world-47), 19)
		span := big.NewRat(int64(1+world%13), 23)
		var input strings.Builder
		input.WriteString("NAME MODEL\nROWS\n N COST\n L L\n G G\n E E\nCOLUMNS\n")
		for _, col := range []string{"default", "free", "boxed", "fixed"} {
			for _, row := range []string{"COST", "L", "G", "E"} {
				input.WriteString(lpExactMPSPair(col, row, coeff.RatString()))
			}
		}
		input.WriteString("RHS\n")
		for _, row := range []string{"L", "G", "E"} {
			input.WriteString(lpExactMPSPair("RHS1", row, rhs.RatString()))
		}
		input.WriteString("RANGES\n" + lpExactMPSPair("RNG", "L", span.RatString()))
		input.WriteString("BOUNDS\n" + lpMPSBoundLine("FR", "free") + lpMPSBoundLine("LO", "boxed -3/7") + lpMPSBoundLine("UP", "boxed 11/13") + lpMPSBoundLine("FX", "fixed 5/29") + "ENDATA\n")
		r := lpReadExactMPS(t, h, input.String())
		m := lpRationalInputModel(t, h)
		h.must(r.fillRationalSolver(m))
		if m.Columns.Len() != 4 || m.Constraints.Len() != 4 || m.AValues.Len() != 4 || m.ProblemIsEmpty() {
			t.Fatal("populated extent")
		}
		for j, name := range []string{"default", "free", "boxed", "fixed"} {
			c, ok := m.Columns.Get2(uint32(j))
			if !ok || c.Name != name || c.Index != InvalidColumnIndex || lpTestNamedColumn(t, m.NamesToColumns, name) != uint32(j) {
				t.Fatal("symbolic column", j)
			}
			lpSparseRationalCheck(t, h, &c.Cost, coeff)
			wantType := []ColumnType{ColumnLowBound, ColumnFree, ColumnBoxed, ColumnFixed}[j]
			if c.ColumnType() != wantType {
				t.Fatal(name, c.ColumnType(), wantType)
			}
			if j == 0 {
				lpSparseRationalCheck(t, h, &c.Low, new(big.Rat))
			}
			if j == 2 {
				lpSparseRationalCheck(t, h, &c.Low, big.NewRat(-3, 7))
				lpSparseRationalCheck(t, h, &c.Upper, big.NewRat(11, 13))
			}
			if j == 3 {
				lpSparseRationalCheck(t, h, &c.FixedValue, big.NewRat(5, 29))
			}
		}
		for i := uint32(0); i < 4; i++ {
			c, ok := m.Constraints.Get2(i)
			wantRelation := []LPRelation{RelationLessOrEqual, RelationGreaterOrEqual, RelationEqual, RelationGreaterOrEqual}[i]
			if !ok || c.Relation != wantRelation {
				t.Fatal("relation", i, c)
			}
			want := new(big.Rat).Set(rhs)
			if i == 3 {
				want.Sub(want, span)
			}
			lpSparseRationalCheck(t, h, c.RightSide, want)
			row, ok := m.AValues.Get2(i)
			if !ok || row.Len() != 4 {
				t.Fatal("row coefficients")
			}
			for j := uint32(0); j < 4; j++ {
				v, ok := row.Get2(j)
				if !ok {
					t.Fatal("coefficient index")
				}
				lpSparseRationalCheck(t, h, v, coeff)
			}
		}
		h.must(m.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalMPSCommonModelPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, direct := range []bool{false, true} {
		for _, phase := range []string{"cost", "row", "bounds"} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 600 && !done; ordinal++ {
				if !t.Run(fmt.Sprintf("direct%t/%s/%d", direct, phase, ordinal), func(t *testing.T) {
					h := newLPPairHarness(t)
					if direct {
						h = lpNewDirectSparseHarness(t)
					}
					large := strings.Repeat("1234567890", 20) + "7/13"
					input := "NAME MODEL\nROWS\n N COST\n L R\nCOLUMNS\n"
					if phase == "cost" {
						input += lpExactMPSPair("x", "COST", large)
					} else {
						input += lpExactMPSPair("x", "R", large)
					}
					input += "RHS\n" + lpExactMPSPair("RHS1", "R", large)
					if phase == "bounds" {
						input += "BOUNDS\n" + lpMPSBoundLine("LO", "x -"+large) + lpMPSBoundLine("UP", "x "+large) + lpMPSBoundLine("FX", "x "+large)
					}
					r := lpReadExactMPS(t, h, input+"ENDATA\n")
					m := lpRationalInputModel(t, h)
					h.budget.FailAfter(ordinal)
					err := r.fillRationalSolver(m)
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) || r.fillRationalSolver(m) != ErrExactVectorPending {
							t.Fatal("population refusal/retry", err)
						}
					}
					// Child copies can borrow the reader's by-value loop coefficient.
					h.must(m.Close())
					h.must(r.Close())
					lpExactVectorNamedCensus(t, h, nil)
				}) {
					t.FailNow()
				}
			}
			if !done || direct && failures == 0 {
				t.Fatal("physical population campaign incomplete", direct, phase, failures)
			}
		}
	}
}

func TestLPRationalMPSCommonModelReachedGuards(t *testing.T) {
	for phase := 0; phase < 8; phase++ {
		h := newLPPairHarness(t)
		r := lpReadExactMPS(t, h, "NAME MODEL\nROWS\n N COST\n L R\nCOLUMNS\n"+lpExactMPSPair("x", "R", "7/13")+"RHS\nENDATA\n")
		m := lpRationalInputModel(t, h)
		want := ErrBounds
		switch phase {
		case 0:
			m.manager = nil
			want = ErrExactVectorOwner
		case 1:
			m.stopped = true
			want = ErrExactVectorPending
		case 2:
			r.stopped = true
			want = ErrExactVectorPending
		case 3:
			row, _ := r.rows.Get2("R")
			row.typ = mpsCost
			want = ErrMPSUnhandled
		case 4:
			r.columns = newRationalMPSColumnMap()
			if sourceDebugEnabled {
				want = ErrAssertion
			}
		case 5:
			r.columns.Set("x", nil)
		case 6:
			// No row coefficient lookup reaches this nil entry: column pass does.
			r.columns.Set("extra", nil)
		case 7:
			m.Columns.Set(0, nil)
		}
		err := r.fillRationalSolver(m)
		if phase == 0 {
			m.manager = h.m
		}
		if err != want {
			t.Fatal(phase, err, want)
		}
		if phase == 3 && (!strings.Contains(r.Messages.(*bytes.Buffer).String(), "Unexpected rt") || r.IsOK()) {
			t.Fatal("unexpected relation diagnostics")
		}
		h.must(m.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPRationalMPSCommonModelDiagnosticPrefixes(t *testing.T) {
	refused := errors.New("exact population diagnostic refused")
	done := false
	for accepted := 0; accepted < 100 && !done; accepted++ {
		h := newLPPairHarness(t)
		r := lpReadExactMPS(t, h, "NAME MODEL\nROWS\n N COST\n L R\nCOLUMNS\n"+lpExactMPSPair("x", "R", "7/13")+"RHS\nENDATA\n")
		row, _ := r.rows.Get2("R")
		row.typ = mpsCost
		w := &lpValidationWriter{allowed: accepted, err: refused}
		r.Messages = w
		m := lpRationalInputModel(t, h)
		err := r.fillRationalSolver(m)
		if err != refused {
			if err != ErrMPSUnhandled || r.IsOK() {
				t.Fatal("ordinary unexpected relation", err, r.IsOK())
			}
			done = true
		}
		if m.Constraints.Len() != 0 || m.AValues.Len() != 0 || m.Columns.Len() != 0 {
			t.Fatal("unexpected relation mutated solver before diagnostics")
		}
		h.must(m.Close())
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !done {
		t.Fatal("diagnostic sequence did not finish")
	}
}

func TestLPRationalMPSCommonModelPreservesOlderComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	matched := false
	for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
		h := newLPPairHarness(t)
		large := strings.Repeat("1234567890", 20) + "7/13"
		r := lpReadExactMPS(t, h, "NAME MODEL\nROWS\n N COST\n L R\nCOLUMNS\n"+lpExactMPSPair("x", "COST", large)+lpExactMPSPair("x", "R", large)+"RHS\n"+lpExactMPSPair("RHS1", "R", large)+"ENDATA\n")
		m := lpRationalInputModel(t, h)
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
			_ = r.fillRationalSolver(m)
			if m.calls.comparison != 0 || r.calls.comparison != 0 {
				t.Fatal("model or reader adopted older comparison")
			}
			for _, col := range m.Columns.All() {
				if col.calls.comparison != 0 {
					t.Fatal("column adopted older comparison")
				}
			}
		}
		h.must(m.Close())
		h.must(r.Close())
		if older.Active && h.m.ComparisonInvocationState() != older {
			t.Fatal("model population consumed older comparison")
		}
		h.must(caller.cleanupRaw(h.m))
		h.must(caller.cleanupRoots(h.m))
		lpExactVectorNamedCensus(t, h, nil)
	}
	if !matched {
		t.Fatal("no actual older comparison")
	}
}
