package nlsat

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverDisplayMathematicaClauseCollectionExcludesLearned(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, Assumption{}))
	learnedBool, err := s.MkBoolVar()
	assignmentMust(t, err)
	_, err = s.clauses.mkClause([]sat.Literal{sat.MakeLiteral(learnedBool, false)}, true, nil)
	assignmentMust(t, err)

	var out bytes.Buffer
	assignmentMust(t, s.DisplayMathematica(&out))
	want := "{\n ((0 < 1)),\n (x0 > 0)}\n"
	if out.String() != want {
		t.Fatalf("Mathematica collection %q, want %q", out.String(), want)
	}
	if strings.Contains(out.String(), "b") {
		t.Fatal("learned clause leaked into original collection", out.String())
	}
	out.Reset()
	assignmentMust(t, s.DisplayMathematicaClause(&out, s.clauses.learned[0]))
	if !strings.HasPrefix(out.String(), "(b") {
		t.Fatal("learned clause direct display", out.String())
	}
}

func TestSolverDisplayMathematicaFailurePrefix(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{3, 2, 0}, evaluatorTerm{-1, 1, 0})
	literal, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	w := &solverDisplayLimitWriter{remaining: 9}
	err = s.DisplayMathematicaLiteral(w, literal)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("Mathematica writer failure lost", err)
	}
	if w.text.String() != "3*x0^2 - " {
		t.Fatalf("Mathematica failure prefix %q", w.text.String())
	}
}
