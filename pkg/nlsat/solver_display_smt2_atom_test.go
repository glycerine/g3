package nlsat

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func TestSolverDisplaySMT2InequalityAndLiteral(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{2, 0, 0})
	q := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1})
	literal, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p, q}, []bool{false, true}, false)
	assignmentMust(t, err)
	atom, found, err := s.AtomForBooleanVariable(literal.Variable())
	assignmentMust(t, err)
	if !found {
		t.Fatal("SMT2 inequality atom missing")
	}
	var out bytes.Buffer
	assignmentMust(t, s.DisplaySMT2Atom(&out, atom))
	want := "(> (* (+ x0 2) (* x1 x1)) 0)"
	if out.String() != want {
		t.Fatalf("SMT2 inequality %q, want %q", out.String(), want)
	}
	out.Reset()
	assignmentMust(t, s.DisplaySMT2Literal(&out, literal.Negated()))
	if want = "(not " + want + ")"; out.String() != want {
		t.Fatalf("SMT2 negative literal %q, want %q", out.String(), want)
	}
}

func TestSolverDisplaySMT2RootStopAndPrefix(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	literal, err := s.MkRoot(AtomRootEQ, 0, 1, p, false)
	assignmentMust(t, err)
	atom, found, err := s.AtomForBooleanVariable(literal.Variable())
	assignmentMust(t, err)
	if !found {
		t.Fatal("SMT2 root atom missing")
	}
	var out bytes.Buffer
	if err = s.DisplaySMT2Atom(&out, atom); !errors.Is(err, ErrSMT2RootAtom) || out.Len() != 0 {
		t.Fatal("direct root stop", err, out.String())
	}
	if err = s.DisplaySMT2Literal(&out, literal.Negated()); !errors.Is(err, ErrSMT2RootAtom) || out.String() != "(not " {
		t.Fatal("negative root prefix", err, out.String())
	}
}
