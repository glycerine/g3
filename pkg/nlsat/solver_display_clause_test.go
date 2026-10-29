package nlsat

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

type solverDisplayLimitWriter struct {
	remaining int
	text      bytes.Buffer
}

func (w *solverDisplayLimitWriter) Write(value []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	n := len(value)
	if n > w.remaining {
		n = w.remaining
	}
	_, _ = w.text.Write(value[:n])
	w.remaining -= n
	if n != len(value) {
		return n, io.ErrClosedPipe
	}
	return n, nil
}

func TestSolverDisplayClauseAssumptionPrefixAndOwnership(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal.Negated(), literal}, AttachedAssumption(&AssumptionAttachment{})))
	clause := s.clauses.originals[len(s.clauses.originals)-1]
	var out bytes.Buffer
	assignmentMust(t, s.DisplayClause(&out, clause))
	if !strings.HasPrefix(out.String(), " |- ") || !strings.Contains(out.String(), "x0 < 0") || !strings.Contains(out.String(), "!(x0 < 0)") {
		t.Fatal("assumption clause display", out.String())
	}
	if err = s.DisplayClause(io.Discard, &Clause{}); !errors.Is(err, ErrAtomCacheOwner) {
		t.Fatal("foreign clause accepted", err)
	}
	if err = s.DisplayClause(nil, clause); !errors.Is(err, arena.ErrConfig) {
		t.Fatal("nil output accepted", err)
	}
}

func TestSolverDisplayWriterFailureRetainsPrefix(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{3, 2, 0}, evaluatorTerm{-1, 1, 0}, evaluatorTerm{5, 0, 0})
	literal, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	w := &solverDisplayLimitWriter{remaining: 7}
	err = s.DisplayLiteral(w, literal)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer failure was lost", err)
	}
	if w.text.String() != "3 x0^2 " {
		t.Fatalf("failure prefix %q", w.text.String())
	}
}
