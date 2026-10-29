package nlsat

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverDisplaySMT2LiteralArityAndClause(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	arithmetic, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	var out bytes.Buffer
	assignmentMust(t, s.DisplaySMT2Literals(&out, nil))
	if out.String() != "false" {
		t.Fatal("empty SMT2 literal vector", out.String())
	}
	out.Reset()
	literals := []sat.Literal{sat.MakeLiteral(plain, false), arithmetic.Negated()}
	assignmentMust(t, s.DisplaySMT2Literals(&out, literals))
	want := "(or b" + strconv.FormatUint(uint64(plain), 10) + " (not (< x0 0)))"
	if out.String() != want {
		t.Fatalf("SMT2 disjunction %q, want %q", out.String(), want)
	}
	clause, err := s.clauses.mkClause(literals, true, nil)
	assignmentMust(t, err)
	out.Reset()
	assignmentMust(t, s.DisplaySMT2Clause(&out, clause))
	if out.String() != want && out.String() != "(or (not (< x0 0)) b"+strconv.FormatUint(uint64(plain), 10)+")" {
		t.Fatal("SMT2 sorted clause", out.String())
	}
}

func TestSolverDisplaySMT2WriterFailure(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{3, 2, 0})
	literal, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	w := &solverDisplayLimitWriter{remaining: 8}
	err = s.DisplaySMT2Literal(w, literal)
	if !errors.Is(err, io.ErrClosedPipe) || w.text.String() != "(= (* 3x" {
		t.Fatal("SMT2 failure prefix", err, w.text.String())
	}
}
