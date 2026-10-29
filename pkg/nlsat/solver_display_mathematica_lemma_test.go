package nlsat

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverDisplayMathematicaAssignmentAndLemma(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkInequality(AtomLT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	evaluatorSetFraction(t, s.evaluator, 0, -3, 2)
	evaluatorSetFraction(t, s.evaluator, 1, 5, 1)

	var out bytes.Buffer
	wrote, err := s.DisplayMathematicaAssignment(&out)
	assignmentMust(t, err)
	if !wrote || out.String() != "x0 == -3/2 && x1 == 5" {
		t.Fatal("Mathematica assignment", wrote, out.String())
	}
	out.Reset()
	assignmentMust(t, s.DisplayMathematicaLemma(&out, []sat.Literal{literal, FalseLiteral}, true))
	want := "Resolve[ForAll[{x0, x1}, !(x0 == -3/2 && x1 == 5) || x0 < 0 || !(0 < 1)], Reals]\n"
	if out.String() != want {
		t.Fatalf("Mathematica lemma %q, want %q", out.String(), want)
	}
}

func TestSolverDisplayMathematicaLemmaEmptyAssignment(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	var out bytes.Buffer
	assignmentMust(t, s.DisplayMathematicaLemma(&out, []sat.Literal{TrueLiteral}, true))
	want := "Resolve[ForAll[{x0}, !(0 < 1) || (0 < 1)], Reals]\n"
	if out.String() != want {
		t.Fatalf("empty assignment lemma %q, want %q", out.String(), want)
	}
}
