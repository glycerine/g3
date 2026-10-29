package nlsat

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverDisplayInequalityRootAndBooleanAtoms(t *testing.T) {
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
		t.Fatal("inequality atom was not retained")
	}
	var out bytes.Buffer
	assignmentMust(t, s.DisplayArithmeticAtom(&out, atom))
	if want := "(x0 + 2)(x1)^2 > 0"; out.String() != want {
		t.Fatalf("inequality display %q, want %q", out.String(), want)
	}
	out.Reset()
	assignmentMust(t, s.DisplayLiteral(&out, literal.Negated()))
	if want := "!((x0 + 2)(x1)^2 > 0)"; out.String() != want {
		t.Fatalf("negative inequality display %q, want %q", out.String(), want)
	}

	rootLiteral, err := s.MkRoot(AtomRootLE, 1, 2, q, false)
	assignmentMust(t, err)
	root, found, err := s.AtomForBooleanVariable(rootLiteral.Variable())
	assignmentMust(t, err)
	if !found {
		t.Fatal("root atom was not retained")
	}
	out.Reset()
	assignmentMust(t, s.DisplayArithmeticAtom(&out, root))
	if want := "x1 <= root[2](x1)"; out.String() != want {
		t.Fatalf("root display %q, want %q", out.String(), want)
	}

	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	out.Reset()
	assignmentMust(t, s.DisplayBooleanAtom(&out, plain))
	if want := "b" + strconv.FormatUint(uint64(plain), 10); out.String() != want {
		t.Fatalf("Boolean display %q, want %q", out.String(), want)
	}
	out.Reset()
	assignmentMust(t, s.DisplayAbstractLiterals(&out, []sat.Literal{FalseLiteral, sat.MakeLiteral(plain, true), literal}))
	if want := "!true or !b" + strconv.FormatUint(uint64(plain), 10) + " or b" + strconv.FormatUint(uint64(literal.Variable()), 10); out.String() != want {
		t.Fatalf("abstract display %q, want %q", out.String(), want)
	}
	out.Reset()
	assignmentMust(t, s.DisplayDisjunction(&out, []sat.Literal{literal, sat.MakeLiteral(plain, false)}))
	if want := "(x0 + 2)(x1)^2 > 0 or b" + strconv.FormatUint(uint64(plain), 10); out.String() != want {
		t.Fatalf("disjunction display %q, want %q", out.String(), want)
	}
	out.Reset()
	assignmentMust(t, s.DisplayLiterals(&out, []sat.Literal{sat.MakeLiteral(plain, false), FalseLiteral}))
	if want := "b" + strconv.FormatUint(uint64(plain), 10) + ";  !true;  "; out.String() != want {
		t.Fatalf("literal-list display %q, want %q", out.String(), want)
	}
}
