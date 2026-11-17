package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h add_edge conflict dump.

import (
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestSMTDenseTheoryConflictDump(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "refused"}[refused], func(t *testing.T) {
			t.Chdir(t.TempDir())
			oldID := smtLemmaID
			t.Cleanup(func() { smtLemmaID = oldID })
			smtLemmaID = 0

			m, _ := smtModelGeneratorFixture(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTDenseDiffTheory(family, smtDenseMI)
			astTestMust(t, err)
			astTestMust(t, theory.ConfigureArithmetic(SMTArithState{DumpLemmas: true}))
			astTestMust(t, state.RegisterTheory(theory))

			var literals []SMTLiteral
			for _, name := range []string{"dense-dump-p", "dense-dump-q"} {
				expression := smtRelevancyBool(t, m, name)
				astTestMust(t, state.Internalize(expression, true))
				literal, err := state.Literal(expression)
				astTestMust(t, err)
				astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
				literals = append(literals, literal)
			}

			theory.denseMatrix().addVariable()
			theory.denseMatrix().addVariable()
			astTestMust(t, theory.ext.setK(testQR(new(big.Rat))))
			astTestMust(t, theory.addEdge(0, 1, literals[0]))
			if refused {
				astTestMust(t, os.Mkdir("lemma_0.smt2", 0700))
			}
			astTestMust(t, theory.ext.setK(testQR(big.NewRat(-1, 1))))
			astTestMust(t, theory.addEdge(1, 0, literals[1]))
			if !state.Inconsistent() || smtLemmaID != 1 {
				t.Fatal("dense conflict did not publish and dump", state.Inconsistent(), smtLemmaID)
			}
			if refused {
				return
			}
			text, err := os.ReadFile("lemma_0.smt2")
			astTestMust(t, err)
			for _, want := range []string{"(assert dense-dump-p)\n", "(assert dense-dump-q)\n"} {
				if strings.Count(string(text), want) != 1 {
					t.Fatal(string(text), want)
				}
			}
		})
	}
}
