package nlsat

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func factorSimplificationFixture(t *testing.T, kind AtomKind, negative bool, even [2]bool, assumption Assumption) (*Solver, polynomial.Polynomial, polynomial.Polynomial, *Clause) {
	t.Helper()
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	q := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 0, 1})
	literal, err := s.MkInequality(kind, []polynomial.Polynomial{p, q}, even[:], negative)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{literal}, assumption))
	return s, p, q, s.clauses.originals[len(s.clauses.originals)-1]
}

func factorLiteralSignature(t *testing.T, s *Solver, p, q polynomial.Polynomial, literal sat.Literal) string {
	t.Helper()
	atom, err := s.clauses.literalAtom(literal)
	assignmentMust(t, err)
	inequality, ok := atom.(*InequalityAtom)
	if !ok || inequality.Size() != 1 {
		t.Fatalf("replacement literal is not a single-factor inequality: %#v", atom)
	}
	factor, err := inequality.Polynomial(0)
	assignmentMust(t, err)
	core, err := s.config.Polynomial.ArithmeticCore()
	assignmentMust(t, err)
	name := "?"
	if same, cause := core.Equal(factor, p); cause != nil {
		assignmentMust(t, cause)
	} else if same {
		name = "p"
	} else if same, cause = core.Equal(factor, q); cause != nil {
		assignmentMust(t, cause)
	} else if same {
		name = "q"
	}
	relation := map[AtomKind]string{AtomEQ: "eq", AtomLT: "lt", AtomGT: "gt"}[inequality.Kind()]
	even, err := inequality.IsEven(0)
	assignmentMust(t, err)
	if even {
		name += "2"
	}
	if literal.Sign() {
		name = "!" + name
	}
	return fmt.Sprintf("%s:%s", name, relation)
}

func factorClauseSignatures(t *testing.T, s *Solver, p, q polynomial.Polynomial) []string {
	t.Helper()
	var result []string
	for _, clause := range s.clauses.originals {
		if clause.Size() == 1 && clause.Literals()[0] == TrueLiteral {
			continue
		}
		literals := make([]string, 0, clause.Size())
		for _, literal := range clause.Literals() {
			literals = append(literals, factorLiteralSignature(t, s, p, q, literal))
		}
		sort.Strings(literals)
		result = append(result, strings.Join(literals, ","))
	}
	sort.Strings(result)
	return result
}

func factorWant(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatal("factor split clauses", got, want)
	}
}

func TestSimplifyUnitFactorsEqualityPolarities(t *testing.T) {
	for _, test := range []struct {
		negative bool
		want     []string
	}{
		{false, []string{"p:eq,q:eq"}},
		{true, []string{"!p:eq", "!q:eq"}},
	} {
		s, p, q, old := factorSimplificationFixture(t, AtomEQ, test.negative, [2]bool{}, Assumption{})
		count, err := s.SimplifyUnitFactors()
		assignmentMust(t, err)
		if count != 1 || s.clauses.owned[old] != nil {
			t.Fatal("equality split retirement", count, s.clauses.owned[old])
		}
		factorWant(t, factorClauseSignatures(t, s, p, q), test.want)
	}
}
