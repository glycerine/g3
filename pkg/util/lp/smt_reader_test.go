package lp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestLPSMTReaderIndependentConstraints(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for world := 0; world < 100; world++ {
		for i, kind := range []LConstraintKind{LConstraintEQ, LConstraintGE, LConstraintLE, LConstraintGT, LConstraintLT} {
			op := []string{"=", ">=", "<=", ">", "<"}[i]
			a, b, constant, negated := world+1, world+2, world*101, world+10
			input := fmt.Sprintf(":formula (and (%s (+ (* %d x) (* (~ %d) y) (* 0 x) %d (~ %d)) ignored_rhs))", op, a, b, constant, negated)
			h := newLPPairHarness(t)
			r := NewSMTReader(h.m, strings.NewReader(input))
			r.Messages = io.Discard
			h.must(r.Read())
			if !r.IsOK() || r.LineNumber != 1 || len(r.Constraints) != 1 {
				t.Fatal(world, i, r.IsOK(), r.LineNumber)
			}
			c := r.Constraints[0]
			if c.Kind != kind || strings.Join(c.Names, ",") != "x,y,x" || c.Coefficient(-1) != nil || c.Coefficient(3) != nil {
				t.Fatal(c.Kind, c.Names)
			}
			for j, want := range []int64{int64(negated - constant), int64(a), -int64(b), 0} {
				lpSparseRationalCheck(t, h, &c.Values.Data[j], big.NewRat(want, 1))
			}
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLPSMTReaderSyntaxAndState(t *testing.T) {
	for _, tc := range []struct {
		input  string
		count  int
		ok     bool
		report string
	}{
		{":formula (and (<= (* 1 x) 0))\n:formula (and (>= (* 2 y) 9))", 2, true, ""},
		{":formula (and (<= (* 1 x) 0) trailing) ignored", 0, false, ""},
		{":formula (and (<= (* 1 x) 0)", 1, true, ""},
		{":formula (and", 0, true, ""},
		{" :formula (and)\n", 0, false, "unexpected top element \nsetting error\n"},
		{":formula\n", 0, false, "empty formula\nunexpected top element \nsetting error\n"},
		{":formula (and\t)\n", 0, false, "unexpected top element and\t\nsetting error\n"},
		{":formula (and (<= (+ (pow 2 x) (* 1 y)) 0))", 1, false, "unexpected input pow\nsetting error\n"},
	} {
		h := newLPPairHarness(t)
		var out bytes.Buffer
		r := NewSMTReader(h.m, strings.NewReader(tc.input))
		r.Messages = &out
		err := r.Read()
		if strings.Contains(tc.input, "trailing") {
			if err == nil {
				t.Fatal("invalid extra constraint accepted")
			}
		} else if err != nil || len(r.Constraints) != tc.count || r.IsOK() != tc.ok || out.String() != tc.report {
			t.Fatal(tc.input, err, len(r.Constraints), r.IsOK(), out.String())
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLPSMTReaderIntegerAndAssertionDomains(t *testing.T) {
	for _, tc := range []struct {
		expr   string
		native bool
		value  int64
	}{
		{"(* (~ -2147483648) x)", true, 2147483648},
		{"(* (- 2) x)", true, -2},
		{"(* (~ 2 3) x)", true, -2},
		{"(* 2 x ignored)", true, 2},
		{"(* -0 x)", true, 0},
		{"(* 2)", false, 0},
	} {
		h := newLPPairHarness(t)
		r := NewSMTReader(h.m, strings.NewReader(":formula (and (<= "+tc.expr+" 0))"))
		r.Messages = io.Discard
		err := r.Read()
		wantSuccess := tc.native && (!sourceDebugEnabled || strings.Contains(tc.expr, "-2147483648"))
		if wantSuccess {
			h.must(err)
			lpSparseRationalCheck(t, h, r.Constraints[0].Coefficient(0), big.NewRat(tc.value, 1))
		} else if err == nil {
			t.Fatal(tc.expr, "invalid domain accepted")
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	r := NewSMTReader(h.m, strings.NewReader(":formula (and (<= (+ 2147483647 2147483647 (~ 5) (* 1 x)) 999))"))
	r.Messages = io.Discard
	h.must(r.Read())
	lpSparseRationalCheck(t, h, &r.Constraints[0].Values.Data[0], big.NewRat(-4294967289, 1))
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPSMTReaderActualFactoryAndNames(t *testing.T) {
	h := newLPPairHarness(t)
	r := NewSMTReader(h.m, strings.NewReader(":formula (and (<= (+ (* 1 x) (* 2 y) (* 3 x)) 0) (>= (* 4 z) 0))"))
	r.Messages = io.Discard
	h.must(r.Read())
	s, err := r.CreateLARSolver()
	if sourceDebugEnabled {
		if err != ErrAssertion || s == nil || len(r.Names) != 2 || r.population == nil {
			t.Fatal(err, r.Names)
		}
	} else {
		h.must(err)
		if len(r.Names) != 3 || r.Names["x"] != 0 || r.Names["y"] != 1 || r.Names["z"] != 2 || s.NumberOfConstraints() != 0 {
			t.Fatal(r.Names, s.NumberOfConstraints())
		}
		status, e := s.Solve()
		h.must(e)
		if status != StatusOptimal {
			t.Fatal(status)
		}
	}
	h.must(s.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPSMTReaderFileAndDiagnosticFailures(t *testing.T) {
	h := newLPPairHarness(t)
	name := lpDriverListFile(t, ":formula (and (<= (* 1 x) 0))")
	r := OpenSMTReader(h.m, name)
	r.Messages = io.Discard
	h.must(r.Read())
	if r.file != nil || len(r.Constraints) != 1 {
		t.Fatal("file not closed before constraints")
	}
	h.must(r.Read())
	if r.IsOK() {
		t.Fatal("closed file read again")
	}
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
	missing := name + ".missing"
	want := "cannot open file " + missing + "\nsetting error\n"
	for accepted := 0; accepted <= len(want); accepted++ {
		h := newLPPairHarness(t)
		w := &lpResidualFailWriter{remaining: accepted}
		r := OpenSMTReader(h.m, missing)
		r.Messages = lpResidualWriterOnly{writer: w}
		err := r.Read()
		if w.String() != want[:accepted] || accepted < len(want) && err != lpErrResidualWrite || accepted == len(want) && err != nil {
			t.Fatal(accepted, err, w.String())
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	for flush := 1; flush <= 2; flush++ {
		h := newLPPairHarness(t)
		w := &lpMPSDriverActionWriter{failFlush: flush}
		r := OpenSMTReader(h.m, missing)
		r.Messages = w
		if err := r.Read(); err != lpErrResidualWrite || w.flushes != flush {
			t.Fatal(err, w.flushes)
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h = newLPPairHarness(t)
	r = OpenSMTReader(h.m, lpDriverListFile(t, ":formula\n"))
	w := &lpMPSDriverActionWriter{}
	w.action = func(text string) error {
		if text == "empty formula\n" {
			return r.file.Close()
		}
		return nil
	}
	r.Messages = w
	if err := r.Read(); !errors.Is(err, os.ErrClosed) || r.LineNumber != 1 {
		t.Fatal(err, r.LineNumber)
	}
	if err := r.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatal(err)
	}
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPSMTReaderPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	input := ":formula (and (<= (+ " + strings.Repeat("(* (~ -2147483648) x) ", 24) + "2147483647 2147483647 (~ 5)) 999))"
	for _, phase := range []string{"read", "prepared-factory"} {
		for _, direct := range []bool{false, true} {
			success, failures := false, 0
			for ordinal := int64(0); ordinal < 4000 && !success; ordinal++ {
				if !t.Run(fmt.Sprintf("%s/direct%t/%d", phase, direct, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					r := NewSMTReader(h.m, strings.NewReader(input))
					r.Messages = io.Discard
					if phase == "prepared-factory" {
						h.must(r.Read())
						// Public source formula coefficients can be mutated before
						// factory construction. This is a prepared rational-domain
						// fixture, not a claim that atoi parses 1000-digit input.
						v := h.pair(strings.Repeat("1234567890", 100)+"7/13", "0")
						h.must(h.m.Copy(r.Constraints[0].Coefficient(0), &v.X))
					}
					var s *LARSolver
					var err error
					h.budget.FailAfter(ordinal)
					if phase == "read" {
						err = r.Read()
					} else {
						s, err = r.CreateLARSolver()
					}
					h.budget.FailAfter(-1)
					if err == nil || phase == "prepared-factory" && sourceDebugEnabled && err == ErrAssertion {
						success = true
						if phase == "read" {
							if len(r.Constraints) != 1 || len(r.Constraints[0].Names) != 24 {
								t.Fatal("incomplete parsed constraint")
							}
							lpSparseRationalCheck(t, h, &r.Constraints[0].Values.Data[0], big.NewRat(-4294967289, 1))
						} else if s == nil || len(r.Names) != 1 {
							t.Fatal("factory boundary", s, r.Names)
						}
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
			if !success || direct && failures == 0 {
				t.Fatal(phase, direct, success, failures)
			}
		}
	}
}

func TestLPSMTReaderDiagnosticPrefixesAndGuards(t *testing.T) {
	for _, tc := range []struct {
		input, report string
		terminal      error
	}{
		{":formula (or)", "unexpected top element or\nsetting error\n", nil},
		{":formula (and (!= (* 1 x) 0))", "kind != is not supported \nsetting error\n", ErrBounds},
		{":formula (and (<= (+ (pow 2 x)) 0))", "unexpected input pow\nsetting error\n", nil},
	} {
		for accepted := 0; accepted <= len(tc.report); accepted++ {
			h := newLPPairHarness(t)
			w := &lpResidualFailWriter{remaining: accepted}
			r := NewSMTReader(h.m, strings.NewReader(tc.input))
			r.Messages = lpResidualWriterOnly{writer: w}
			err := r.Read()
			wantErr := lpErrResidualWrite
			if accepted == len(tc.report) {
				wantErr = tc.terminal
			}
			if err != wantErr || w.String() != tc.report[:accepted] {
				t.Fatal(tc.input, accepted, err, w.String())
			}
			h.must(r.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
	r := NewSMTReader(nil, strings.NewReader(":formula (and)"))
	r.Messages = io.Discard
	if r.Read() != ErrExactVectorOwner {
		t.Fatal("missing owner accepted")
	}
	if _, err := r.CreateLARSolver(); err != ErrExactVectorOwner {
		t.Fatal(err)
	}
	lpStaticMust(t, r.Close())
	h := newLPPairHarness(t)
	r = NewSMTReader(h.m, strings.NewReader(":formula (and)"))
	r.Messages = io.Discard
	h.must(r.Read())
	s, err := r.CreateLARSolver()
	h.must(err)
	h.must(s.Close())
	h.must(r.Close())
	if r.Read() != ErrExactVectorPending {
		t.Fatal("closed reader read")
	}
	lpExactVectorNamedCensus(t, h, nil)
	h = newLPPairHarness(t)
	r = NewSMTReader(h.m, strings.NewReader(":formula (or)"))
	r.Messages = io.Discard
	h.must(r.Read())
	if r.IsOK() {
		t.Fatal("invalid top-level formula accepted")
	}
	// Source factory neither gates on IsOK nor parses implicitly.
	s, err = r.CreateLARSolver()
	h.must(err)
	h.must(s.Close())
	h.must(r.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLPSMTReaderOlderCallerComparison(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, factory := range []bool{false, true} {
		matched := false
		for ordinal := int64(0); ordinal < 40 && !matched; ordinal++ {
			h := newLPPairHarness(t)
			r := NewSMTReader(h.m, strings.NewReader(":formula (and (<= (+ (* (~ -2147483648) x) 2147483647 2147483647) 0))"))
			r.Messages = io.Discard
			if factory {
				h.must(r.Read())
			}
			v := h.pair(strings.Repeat("9876543210", 100)+"/7", strings.Repeat("1234567890", 100)+"/13")
			var caller exactEtaCalls
			before, err := caller.begin(h.m)
			h.must(err)
			h.budget.FailAfter(ordinal)
			_, err = h.m.Compare(&v.X, &v.Y)
			h.budget.FailAfter(-1)
			err = caller.after(h.m, before, err)
			older := h.m.ComparisonInvocationState()
			var s *LARSolver
			if older.Active {
				matched = true
				if !lpExactPriorityPhysicalRefusal(err) {
					t.Fatal(err)
				}
				if factory {
					s, _ = r.CreateLARSolver()
				} else {
					_ = r.Read()
				}
				if r.pending != nil && r.pending.calls.comparison != 0 || r.pendingCopy != nil && r.pendingCopy.calls.comparison != 0 || s != nil && s.calls.comparison != 0 {
					t.Fatal("reader adopted caller receipt")
				}
			}
			if s != nil {
				h.must(s.Close())
			}
			h.must(r.Close())
			if older.Active && h.m.ComparisonInvocationState() != older {
				t.Fatal("reader cleanup consumed caller receipt")
			}
			h.must(caller.cleanupRaw(h.m))
			h.must(caller.cleanupRoots(h.m))
			lpExactVectorNamedCensus(t, h, nil)
		}
		if !matched {
			t.Fatal("no actual older comparison", factory)
		}
	}
}

func TestLPSMTReaderErasedAssertionsAndMalformedTokens(t *testing.T) {
	for _, tc := range []struct {
		input       string
		nativeNames string
		malformed   bool
	}{
		{":formula (and lone", "", true},
		{":formula (and (<= (* 2 9) 0))", "9", false},
		{":formula (and (<= (* 2 (x a)) 0))", "x", false},
		{":formula (and (<= bare 0))", "", false},
	} {
		h := newLPPairHarness(t)
		r := NewSMTReader(h.m, strings.NewReader(tc.input))
		r.Messages = io.Discard
		err := r.Read()
		if sourceDebugEnabled || tc.malformed {
			if err == nil {
				t.Fatal("assertion/bad token accepted", tc.input)
			}
		} else {
			h.must(err)
			if len(r.Constraints) != 1 || strings.Join(r.Constraints[0].Names, ",") != tc.nativeNames {
				t.Fatal(tc.input, r.Constraints)
			}
		}
		h.must(r.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

// AUDIT.md util_lp_06-F2: numerals go through atoi, which keeps the longest
// integer prefix ("3.5" is 3, "2x" is 2).
func TestSMTReaderAtoiNumerals(t *testing.T) {
	h := newLPPairHarness(t)
	r := NewSMTReader(h.m, strings.NewReader(":formula (and (<= (+ (* 3.5 x) 2x) 0))\n"))
	r.Messages = io.Discard
	h.must(r.Read())
	if !r.IsOK() || len(r.Constraints) != 1 {
		t.Fatal(r.IsOK(), len(r.Constraints))
	}
	c := r.Constraints[0]
	if strings.Join(c.Names, ",") != "x" {
		t.Fatal(c.Names)
	}
	lpSparseRationalCheck(t, h, &c.Values.Data[0], big.NewRat(-2, 1))
	lpSparseRationalCheck(t, h, &c.Values.Data[1], big.NewRat(3, 1))
	h.must(r.Close())
	// atoi truncates strtol's 2147483648 to int; is_integer holds for both.
	for _, tc := range []struct {
		expr  string
		value int64
	}{{"(* 2147483648 x)", -2147483648}, {"(* 2junk x)", 2}} {
		h := newLPPairHarness(t)
		r := NewSMTReader(h.m, strings.NewReader(":formula (and (<= "+tc.expr+" 0))"))
		r.Messages = io.Discard
		h.must(r.Read())
		lpSparseRationalCheck(t, h, r.Constraints[0].Coefficient(0), big.NewRat(tc.value, 1))
		h.must(r.Close())
	}
	for _, tc := range []struct {
		text string
		want int32
	}{{" 42", 42}, {"-7z", -7}, {"+5", 5}, {"x", 0}, {"4294967297", 1}, {"99999999999999999999", -1}, {"-99999999999999999999", 0}} {
		if got := smtAtoi(tc.text); got != tc.want {
			t.Fatal(tc.text, got, tc.want)
		}
	}
}
