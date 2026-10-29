package sat

// Source: src/sat/sat_big.cpp:28-70,174-216.

import "testing"

func satBIGRedundantFixture(t *testing.T) *Solver {
	t.Helper()
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 3; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	a, b, c := MakeLiteral(0, false), MakeLiteral(1, false), MakeLiteral(2, false)
	for _, clause := range [][]Literal{{a.Negated(), b}, {b.Negated(), c}, {a.Negated(), c}} {
		satOK(t, s.MkClause(clause))
	}
	return s
}

func TestSATBIGSolverExtractionAndTransitiveReduction(t *testing.T) {
	s := satBIGRedundantFixture(t)
	b := newSATBIG(&s.decision.rng)
	satOK(t, b.initSolver(s, false))
	a, c := MakeLiteral(0, false), MakeLiteral(2, false)
	if !b.reaches(a, c) || !b.connected(a, c) {
		t.Fatal("solver binary implication graph missed chain")
	}
	n, err := b.reduceTransitive(s.Lifecycle)
	satOK(t, err)
	if n != 1 {
		t.Fatal("redundant binaries", n)
	}
	pairs, err := s.appendBinaryClauses(nil, false)
	satOK(t, err)
	if len(pairs) != 2 {
		t.Fatal("remaining binaries", pairs)
	}
	for mask := 0; mask < 8; mask++ {
		original := (!((mask & 1) != 0) || (mask&2) != 0) && (!((mask & 2) != 0) || (mask&4) != 0) && (!((mask & 1) != 0) || (mask&4) != 0)
		reduced := true
		for _, pair := range pairs {
			value := func(l Literal) bool { return ((mask>>l.variable())&1 != 0) != l.sign() }
			reduced = reduced && (value(pair.Literal1()) || value(pair.Literal2()))
		}
		if original != reduced {
			t.Fatal("transitive reduction changed truth table", mask, original, reduced, pairs)
		}
	}
}

func TestSATBIGLearnedExtractionMode(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false, true)
		satOK(t, err)
	}
	a, b := MakeLiteral(0, false), MakeLiteral(1, false)
	satOK(t, s.Lifecycle.mkBinClause(a.Negated(), b, true))
	hard := newSATBIG(&s.decision.rng)
	satOK(t, hard.initSolver(s, false))
	if hard.reaches(a, b) {
		t.Fatal("non-learned graph included learned edge")
	}
	all := newSATBIG(&s.decision.rng)
	satOK(t, all.initSolver(s, true))
	if !all.reaches(a, b) || !all.Learned() {
		t.Fatal("learned graph omitted edge")
	}
}
