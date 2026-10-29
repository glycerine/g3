package sat

// Source: src/sat/dimacs.h; src/sat/dimacs.cpp.

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestParseDIMACSSourceGrammarAndVariableNumbering(t *testing.T) {
	solver, _, _ := satSolverCheckFixture(t)
	input := "\t c leading comment\r\np cnf 4 5\n-1 3 0\n-2 +3 0\n-3 4 0\n3 -4 0\n1 2 -3 0\nc ignored tail\n"
	satOK(t, ParseDIMACS(strings.NewReader(input), solver))
	if got, err := solver.sourceNumVars(); err != nil || got != 5 {
		t.Fatal("source parser must retain unused variable zero", got, err)
	}
	if got, err := solver.numClauses(); err != nil || got != 5 {
		t.Fatal("clause count", got, err)
	}
	status, err := solver.Check(nil)
	if err != nil || status != lbool.True {
		t.Fatal("parsed source problem", status, err)
	}
	model, err := solver.borrowModel()
	satOK(t, err)
	clauses := [][]Literal{
		{MakeLiteral(1, true), MakeLiteral(3, false)},
		{MakeLiteral(2, true), MakeLiteral(3, false)},
		{MakeLiteral(3, true), MakeLiteral(4, false)},
		{MakeLiteral(3, false), MakeLiteral(4, true)},
		{MakeLiteral(1, false), MakeLiteral(2, false), MakeLiteral(3, true)},
	}
	for i, clause := range clauses {
		satisfied := false
		for _, literal := range clause {
			value, e := model.Read(literal.variable())
			satOK(t, e)
			if literal.sign() {
				value = value.Negated()
			}
			satisfied = satisfied || value == lbool.True
		}
		if !satisfied {
			t.Fatal("model does not satisfy parsed clause", i)
		}
	}
}

func TestParseDIMACSEmptyClauseAndHeaderCountsIgnored(t *testing.T) {
	solver, _, _ := satSolverCheckFixture(t)
	satOK(t, ParseDIMACS(strings.NewReader("p cnf 999 0\n0\n"), solver))
	if got, err := solver.sourceNumVars(); err != nil || got != 0 {
		t.Fatal("header allocated variables", got, err)
	}
	status, err := solver.Check(nil)
	if err != nil || status != lbool.False {
		t.Fatal("empty clause", status, err)
	}
}

func TestParseDIMACSMalformedInputPreservesReachedPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		char  string
		vars  uint32
	}{
		{"letter", "1 0\n2 x", "120", 3},
		{"sign at eof", "1 0\n-", "-1", 2},
		{"non-source whitespace", "1 0\x002 0", "0", 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			solver, _, _ := satSolverCheckFixture(t)
			err := ParseDIMACS(strings.NewReader(test.input), solver)
			if !errors.Is(err, ErrDIMACSUnexpectedChar) || !strings.Contains(err.Error(), test.char) {
				t.Fatal("parse error", err)
			}
			if got, e := solver.numClauses(); e != nil || got != 1 {
				t.Fatal("published clause prefix", got, e)
			}
			if got, e := solver.sourceNumVars(); e != nil || got != test.vars {
				t.Fatal("published variable prefix", got, e)
			}
		})
	}
}

func TestParseDIMACSOwnerAndClosedSolver(t *testing.T) {
	if err := ParseDIMACS(strings.NewReader(""), nil); !errors.Is(err, ErrSATOwner) {
		t.Fatal(err)
	}
	solver, _, _ := satSolverCheckFixture(t)
	satOK(t, solver.Close())
	if err := ParseDIMACS(strings.NewReader(""), solver); err == nil {
		t.Fatal("closed solver accepted")
	}
}
