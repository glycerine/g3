package nlsat

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSolverDisplaySMT2DeclarationsAndScript(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	_, err = s.MkVar(true)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	arithmetic, err := s.MkInequality(AtomGT, []polynomial.Polynomial{p}, []bool{false}, false)
	assignmentMust(t, err)
	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{sat.MakeLiteral(plain, false), arithmetic}, Assumption{}))

	var out bytes.Buffer
	assignmentMust(t, s.DisplaySMT2Declarations(&out))
	declarations := out.String()
	for _, want := range []string{"(declare-fun b0 () Bool)\n", "(declare-fun b" + strconv.FormatUint(uint64(plain), 10) + " () Bool)\n", "(declare-fun x0 () Real)\n", "(declare-fun x1 () Int)\n"} {
		if !strings.Contains(declarations, want) {
			t.Fatalf("SMT2 declarations %q lack %q", declarations, want)
		}
	}
	if strings.Contains(declarations, "b"+strconv.FormatUint(uint64(arithmetic.Variable()), 10)+" () Bool") {
		t.Fatal("arithmetic-backed Boolean was declared", declarations)
	}
	out.Reset()
	assignmentMust(t, s.DisplaySMT2(&out))
	script := out.String()
	if !strings.Contains(script, "(assert (and true\ntrue\n") || !strings.Contains(script, "(> x0 0)") || !strings.Contains(script, "))\n(check-sat)\n") {
		t.Fatal("SMT2 script", script)
	}
}

func TestSolverDisplaySMT2ScriptStopsAtRoot(t *testing.T) {
	s := solverFixture(t)
	_, err := s.MkVar(false)
	assignmentMust(t, err)
	p := evaluatorPolynomial(t, s.evaluator, evaluatorTerm{1, 1, 0})
	root, err := s.MkRoot(AtomRootLT, 0, 1, p, false)
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{root}, Assumption{}))
	var out bytes.Buffer
	err = s.DisplaySMT2(&out)
	if !errors.Is(err, ErrSMT2RootAtom) {
		t.Fatal("SMT2 script accepted root", err)
	}
	if !strings.HasSuffix(out.String(), "(assert (and true\ntrue\n") {
		t.Fatalf("SMT2 root-stop prefix %q", out.String())
	}
}
