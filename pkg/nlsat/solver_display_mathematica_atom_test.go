package nlsat

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverDisplayMathematicaAtomsAndLiterals(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{2, 0, 0})
	q := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1})
	literal, err := s.MkInequality(AtomEQ, []polynomial.Polynomial{p, q}, []bool{false, true}, false)
	assignmentMust(t, err)
	atom, found, err := s.AtomForBooleanVariable(literal.Variable())
	assignmentMust(t, err)
	if !found {
		t.Fatal("inequality atom missing")
	}
	var out bytes.Buffer
	assignmentMust(t, s.DisplayMathematicaAtom(&out, atom))
	if want := "(x0 + 2)*((x1^2)) == 0"; out.String() != want {
		t.Fatalf("Mathematica inequality %q, want %q", out.String(), want)
	}
	out.Reset()
	assignmentMust(t, s.DisplayMathematicaLiteral(&out, literal.Negated()))
	if want := "!((x0 + 2)*((x1^2)) == 0)"; out.String() != want {
		t.Fatalf("Mathematica negative literal %q, want %q", out.String(), want)
	}

	rootPolynomial := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0}, evaluatorTerm{1, 0, 2})
	rootLiteral, err := s.MkRoot(AtomRootGE, 1, 3, rootPolynomial, false)
	assignmentMust(t, err)
	root, found, err := s.AtomForBooleanVariable(rootLiteral.Variable())
	assignmentMust(t, err)
	if !found {
		t.Fatal("root atom missing")
	}
	out.Reset()
	assignmentMust(t, s.DisplayMathematicaAtom(&out, root))
	if want := "x1 >= Root[#1^2 + x0 &, 3]"; out.String() != want {
		t.Fatalf("Mathematica root %q, want %q", out.String(), want)
	}

	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	out.Reset()
	assignmentMust(t, s.DisplayMathematicaLiterals(&out, []sat.Literal{TrueLiteral, sat.MakeLiteral(plain, true), rootLiteral}))
	if want := "(0 < 1) || !b" + strconv.FormatUint(uint64(plain), 10) + " || x1 >= Root[#1^2 + x0 &, 3]"; out.String() != want {
		t.Fatalf("Mathematica literals %q, want %q", out.String(), want)
	}
}
