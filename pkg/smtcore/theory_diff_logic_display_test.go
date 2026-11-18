package smtcore

// Source: src/smt/theory_diff_logic_def.h:38-52,920-925 and
// src/smt/diff_logic.h:846-922.

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTDifferenceDisplayAtomsGraphsAssignmentsAndGuards(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		f := newSMTRealDiffFixture(t)
		bound := f.bound(t, smtArithOpLE, f.x, f.y, 1, 2)
		astTestMust(t, f.state.Internalize(bound, true))
		literal, err := f.state.Literal(bound)
		astTestMust(t, err)
		astTestMust(t, f.state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		astTestMust(t, f.theory.Assign(literal.Var(), true))
		astTestMust(t, f.theory.Propagate())

		var output bytes.Buffer
		astTestMust(t, f.theory.Display(&output))
		const want = "(not p1) (<= (+ real-diff-x (* (- 1.0) real-diff-y)) (/ 1.0 2.0)) p1 (<= (- $1 $2) 1/2) 0\n$0 := 0\n$1 := 0\n$2 := 0\n"
		if output.String() != want {
			t.Fatalf("real difference display\ngot:\n%s\nwant:\n%s", output.String(), want)
		}
		if err = (&SMTRealDiffTheory{}).Display(&output); !errors.Is(err, ast.ErrASTHandle) {
			t.Fatal("uninitialized theory", err)
		}
	})
}
