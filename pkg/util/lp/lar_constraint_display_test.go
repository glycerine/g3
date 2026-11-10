package lp

// Source: src/util/lp/lar_solver.h:1309-1337,1359-1362;
// column_namer.h:73-97.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func lpLARDisplayFixture(t *testing.T, h *lpPairHarness, profile string) (*LARSolver, string) {
	t.Helper()
	s, err := NewLARSolver(h.m)
	h.must(err)
	for _, ext := range []uint32{17, 31, 47} {
		_, err = s.AddVar(ext)
		h.must(err)
	}
	input := NewSparseRationalVector(h.m)
	rhs, constant := "7/3", "0"
	var coefficients []string
	var expected string
	switch profile {
	case "variable":
		expected = "v17 <= 7/3\n"
	case "signs":
		coefficients = []string{"-1", "1", "-2/3", "0"}
		expected = " - v17 + v31 - 2/3v47 - 0v17 <= 7/3\n"
	case "first-negative":
		coefficients = []string{"-2/3", "-1"}
		expected = "-2/3v17 - v31 <= 7/3\n"
	case "positive":
		coefficients = []string{"2/3", "1"}
		expected = "2/3v17 + v31 <= 7/3\n"
	case "zero":
		coefficients = []string{"0", "1"}
		expected = "0v17 + v31 <= 7/3\n"
	case "term-negative", "term-positive":
		coefficients = []string{"-1", "1"}
		constant = "-5"
		if profile == "term-positive" {
			constant = "5"
		}
		// lar_term's unordered_map iterates column 1 before column 0.
		expected = "v31 - v17 + " + constant + " <= 7/3\n"
	case "empty-general":
		expected = " <= 7/3\n"
	case "empty-term":
		constant = "-5"
		expected = " + -5 <= 7/3\n"
	case "term-large":
		coefficients = []string{"-1", "1"}
		constant = mustLPRat(t, "-"+strings.Repeat("2718281828", 20)+"/19").RatString()
		expected = "v31 - v17 + " + constant + " <= 7/3\n"
	case "large":
		a := mustLPRat(t, strings.Repeat("1234567890", 20)+"/7").RatString()
		b := mustLPRat(t, strings.Repeat("9876543210", 20)+"/13").RatString()
		coefficients = []string{a, "-" + b}
		rhs = "-" + a
		expected = a + "v17 - " + b + "v31 <= " + rhs + "\n"
	default:
		t.Fatal(profile)
	}
	for i, value := range coefficients {
		h.must(input.Append(uint32(i%3), &h.pair(value, "0").X))
	}
	var c LARStoredConstraint
	if profile == "variable" {
		c, err = NewLARVarConstraint(h.m, 0, LConstraintLE, &h.pair(rhs, "0").X)
	} else if strings.HasPrefix(profile, "term-") || profile == "empty-term" {
		term, e := NewLARTermFromCoefficients(h.m, input.Data, &h.pair(constant, "0").X)
		h.must(e)
		s.Terms = append(s.Terms, term)
		c, err = NewLARTermConstraint(h.m, term, LConstraintLE, &h.pair(rhs, "0").X)
	} else {
		c, err = NewLARConstraint(h.m, input.Data, LConstraintLE, &h.pair(rhs, "0").X)
	}
	h.must(err)
	s.Constraints = append(s.Constraints, c)
	h.must(input.Close())
	return s, expected
}

func TestLARConstraintDisplaySourceText(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, profile := range []string{"variable", "signs", "first-negative", "positive", "zero", "term-negative", "term-positive", "empty-general", "empty-term", "large", "term-large"} {
		for _, kind := range []LConstraintKind{LConstraintLE, LConstraintLT, LConstraintGE, LConstraintGT, LConstraintEQ} {
			t.Run(fmt.Sprintf("%s/%d", profile, kind), func(t *testing.T) {
				h := newLPPairHarness(t)
				s, expected := lpLARDisplayFixture(t, h, profile)
				s.Constraints[0].Base().Kind = kind
				symbol, err := LConstraintKindString(kind)
				h.must(err)
				expected = strings.Replace(expected, " <= ", " "+symbol+" ", 1)
				var out bytes.Buffer
				h.must(s.PrintConstraint(0, &out))
				if out.String() != expected || s.validationCoefficients != nil || s.rationalWork != nil {
					t.Fatal("source constraint spelling/local cleanup", out.String(), expected)
				}
				h.must(s.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

func TestLARConstraintDisplayEveryWriterPrefix(t *testing.T) {
	for _, profile := range []string{"signs", "term-negative"} {
		h0 := newLPPairHarness(t)
		s0, text := lpLARDisplayFixture(t, h0, profile)
		h0.must(s0.Close())
		lpExactVectorNamedCensus(t, h0, nil)
		for cut := 0; cut < len(text); cut++ {
			t.Run(fmt.Sprintf("%s/%d", profile, cut), func(t *testing.T) {
				h := newLPPairHarness(t)
				s, expected := lpLARDisplayFixture(t, h, profile)
				out := &larConstraintPrefixWriter{remaining: cut, err: io.ErrClosedPipe}
				if err := s.WriteConstraint(s.Constraints[0], out); err != io.ErrClosedPipe || out.output.String() != expected[:cut] {
					t.Fatal("accepted source output prefix", cut, err, out.output.String(), expected[:cut])
				}
				if s.PrintConstraint(0, io.Discard) != ErrExactVectorPending {
					t.Fatal("stopped diagnostic retry")
				}
				h.must(s.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
		}
	}
}

type lpLARConstraintFlushWriter struct {
	output bytes.Buffer
	err    error
	flush  int
}

func (w *lpLARConstraintFlushWriter) Write(p []byte) (int, error) { return w.output.Write(p) }
func (w *lpLARConstraintFlushWriter) Flush() error {
	w.flush++
	return w.err
}

func lpLARDisplayListFixture(t *testing.T, h *lpPairHarness) *LARSolver {
	t.Helper()
	s, err := NewLARSolver(h.m)
	h.must(err)
	_, err = s.AddVar(17)
	h.must(err)
	_, err = s.AddVarBound(0, LConstraintGE, &h.pair("-7/3", "0").X)
	h.must(err)
	_, err = s.AddVarBound(0, LConstraintLT, &h.pair("11/2", "0").X)
	h.must(err)
	_, err = s.AddTerm([]RationalSparseEntry{{Index: 0, Value: h.pair("-1", "0").X}}, &h.pair("5/2", "0").X)
	h.must(err)
	_, err = s.AddTerm([]RationalSparseEntry{{Index: 0, Value: h.pair("3", "0").X}}, &h.pair("0", "0").X)
	h.must(err)
	// print_terms uses current terms, whereas print_implied_bound deliberately
	// uses OriginalTerms. Distinguish them without changing either term's shape.
	h.must(s.Terms[0].SetValue(&h.pair("99", "0").X))
	return s
}

func TestLARDisplayListsOrderAndFlushes(t *testing.T) {
	h := newLPPairHarness(t)
	s := lpLARDisplayListFixture(t, h)
	out := &lpLARConstraintFlushWriter{}
	h.must(s.PrintConstraints(out))
	h.must(s.PrintTerms(out))
	want := "v17 >= -7/3\nv17 < 11/2\n99 +  - v17\n3v17\n"
	if out.output.String() != want || out.flush != 2 {
		t.Fatal("ordered source lists", out.output.String(), out.flush)
	}
	if s.validationCoefficients != nil || s.rationalWork != nil {
		t.Fatal("successful display retained lexical owners")
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARDisplayListsEveryWriterPrefix(t *testing.T) {
	for _, terms := range []bool{false, true} {
		want := "v17 >= -7/3\nv17 < 11/2\n"
		if terms {
			want = "99 +  - v17\n3v17\n"
		}
		for cut := 0; cut < len(want); cut++ {
			h := newLPPairHarness(t)
			s := lpLARDisplayListFixture(t, h)
			out := &larConstraintPrefixWriter{remaining: cut, err: io.ErrClosedPipe}
			var err error
			if terms {
				err = s.PrintTerms(out)
			} else {
				err = s.PrintConstraints(out)
			}
			if !errors.Is(err, io.ErrClosedPipe) || !s.stopped {
				t.Fatal("writer refusal", terms, cut, err, s.stopped)
			}
			// The shared writer retains exactly the accepted output prefix.
			if out.output.String() != want[:cut] {
				t.Fatal("refusal prefix", terms, cut, out.output.String(), want[:cut])
			}
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		}
	}
}

func TestLARDisplayListsEmptyAndFlushRefusal(t *testing.T) {
	for _, terms := range []bool{false, true} {
		h := newLPPairHarness(t)
		s, err := NewLARSolver(h.m)
		h.must(err)
		out := &lpLARConstraintFlushWriter{}
		h.must(s.PrintConstraints(out))
		h.must(s.PrintTerms(out))
		if out.output.Len() != 0 || out.flush != 0 {
			t.Fatal("empty lists produced output")
		}
		if terms {
			err = s.PrintTerms(nil)
		} else {
			err = s.PrintConstraints(nil)
		}
		if !errors.Is(err, ErrBounds) || !s.stopped {
			t.Fatal("nil writer", terms, err)
		}
		h.must(s.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
	h := newLPPairHarness(t)
	s := lpLARDisplayListFixture(t, h)
	out := &lpLARConstraintFlushWriter{err: io.ErrClosedPipe}
	if err := s.PrintConstraints(out); !errors.Is(err, io.ErrClosedPipe) || out.output.String() != "v17 >= -7/3\n" || out.flush != 1 {
		t.Fatal("first endl refusal", err, out.output.String(), out.flush)
	}
	h.must(s.Close())
	lpExactVectorNamedCensus(t, h, nil)
}

func TestLARConstraintDisplayReachedPrefixes(t *testing.T) {
	for _, phase := range []string{"missing", "nil-writer", "nil-constraint", "stopped-constraint", "unknown", "flush", "short-write"} {
		t.Run(phase, func(t *testing.T) {
			h := newLPPairHarness(t)
			s, expected := lpLARDisplayFixture(t, h, "variable")
			var out bytes.Buffer
			want := error(nil)
			var err error
			switch phase {
			case "missing":
				err = s.PrintConstraint(^uint32(0), &out)
				expected = "constraint 4294967295 is not found\n"
			case "nil-writer":
				err = s.PrintConstraint(0, nil)
				want, expected = ErrBounds, ""
			case "nil-constraint":
				err = s.WriteConstraint(nil, &out)
				want, expected = ErrExactVectorOwner, ""
			case "stopped-constraint":
				s.Constraints[0].Base().stopped = true
				err = s.WriteConstraint(s.Constraints[0], &out)
				want, expected = ErrExactVectorPending, ""
			case "unknown":
				s.Constraints[0].Base().Kind = LConstraintKind(255)
				err = s.PrintConstraint(0, &out)
				expected = "v17  7/3\n"
				if sourceDebugEnabled {
					want, expected = ErrAssertion, "v17 "
				}
			case "flush":
				w := &lpLARConstraintFlushWriter{err: io.ErrClosedPipe}
				err = s.PrintConstraint(0, w)
				if w.flush != 1 || w.output.String() != expected {
					t.Fatal("source endl prefix/flush", w.flush, w.output.String())
				}
				out.WriteString(w.output.String())
				want = io.ErrClosedPipe
			case "short-write":
				w := &larConstraintPrefixWriter{remaining: 2}
				err = s.PrintConstraint(0, w)
				out.WriteString(w.output.String())
				want, expected = io.ErrShortWrite, "v1"
			}
			if !errors.Is(err, want) || out.String() != expected {
				t.Fatal("diagnostic stage", phase, err, want, out.String(), expected)
			}
			h.must(s.Close())
			lpExactVectorNamedCensus(t, h, nil)
		})
	}
}

func TestLARConstraintDisplayPhysicalOwnership(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}
	for _, profile := range []string{"large", "term-negative", "term-large"} {
		for _, direct := range []bool{false, true} {
			done, failures := false, 0
			for ordinal := int64(0); ordinal < 1200 && !done; ordinal++ {
				t.Run(fmt.Sprintf("%s/%v/%d", profile, direct, ordinal), func(t *testing.T) {
					var h *lpPairHarness
					if direct {
						h = lpNewDirectSparseHarness(t)
					} else {
						h = newLPPairHarness(t)
					}
					s, expected := lpLARDisplayFixture(t, h, profile)
					var out bytes.Buffer
					h.budget.FailAfter(ordinal)
					err := s.PrintConstraint(0, &out)
					h.budget.FailAfter(-1)
					if err == nil {
						done = true
						if out.String() != expected {
							t.Fatal("source physical diagnostic", out.String(), expected)
						}
					} else {
						failures++
						if !lpExactPriorityPhysicalRefusal(err) || !strings.HasPrefix(expected, out.String()) || s.PrintConstraint(0, io.Discard) != ErrExactVectorPending {
							t.Fatal("physical diagnostic prefix", err, out.String())
						}
					}
					h.must(s.Close())
					lpExactVectorNamedCensus(t, h, nil)
				})
			}
			if !done || direct && profile != "term-negative" && failures == 0 {
				t.Fatal("diagnostic physical campaign", profile, direct, failures)
			}
		}
	}
}
