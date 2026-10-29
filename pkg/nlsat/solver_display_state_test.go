package nlsat

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSolverDisplayStateAndAssignments(t *testing.T) {
	s := solverFixture(t)
	for i := 0; i < 2; i++ {
		_, err := s.MkVar(false)
		assignmentMust(t, err)
	}
	plain, err := s.MkBoolVar()
	assignmentMust(t, err)
	assignmentMust(t, s.AddClause([]sat.Literal{sat.MakeLiteral(plain, false)}, Assumption{}))
	evaluatorSetFraction(t, s.evaluator, 0, -3, 2)
	values := make([]lbool.LBool, int(plain)+1)
	values[plain] = lbool.True
	assignmentMust(t, s.SetBooleanValues(values))

	var out bytes.Buffer
	assignmentMust(t, s.DisplayAssignment(&out))
	wantAssignment := "b" + strconv.FormatUint(uint64(plain), 10) + " -> true\nx0 -> -1.5\n"
	if out.String() != wantAssignment {
		t.Fatalf("assignment display %q, want %q", out.String(), wantAssignment)
	}
	out.Reset()
	assignmentMust(t, s.DisplayVars(&out))
	if out.String() != "0 -> x0\n1 -> x1\n" {
		t.Fatal("variable display", out.String())
	}
	out.Reset()
	assignmentMust(t, s.Display(&out))
	text := out.String()
	for _, fragment := range []string{"true\n", "b" + strconv.FormatUint(uint64(plain), 10) + "\n", "b" + strconv.FormatUint(uint64(plain), 10) + " -> true\n", "x0 -> -1.5\n"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("solver state %q lacks %q", text, fragment)
		}
	}
}
