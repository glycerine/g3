package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTSequenceEmptyWordSolutionsSurviveSuppressedCallback(t *testing.T) {
	for _, emptyLeft := range []bool{false, true} {
		name := "empty-right"
		if emptyLeft {
			name = "empty-left"
		}
		t.Run(name, func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.SequenceFamilyID()
			astTestMust(t, err)
			theory, err := NewSMTSequenceTheory(family)
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			astTestMust(t, state.RegisterTheory(theory))
			empty := astTestString(t, m, "")
			words := []ast.ASTHandle{
				astTestStringConst(t, m, "empty-word-left"),
				astTestStringConst(t, m, "empty-word-right"),
			}
			for _, expression := range append(words, empty) {
				astTestMust(t, state.Internalize(expression, false))
			}
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, state.PushScope())
				for _, word := range words {
					left, right := []ast.ASTHandle{word}, []ast.ASTHandle(nil)
					if emptyLeft {
						left, right = right, left
					}
					astTestMust(t, theory.equations.Push(theory.nextEquation, left, right, nil))
					theory.nextEquation++
				}
				for theory.equations.Len() != 0 {
					progress, err := theory.solveWordEquations()
					astTestMust(t, err)
					if !progress && theory.equations.Len() != 0 {
						t.Fatal("empty word equation stalled")
					}
				}
				consistent, err := state.PropagateCore()
				astTestMust(t, err)
				if !consistent {
					t.Fatal("empty word equation conflicted")
				}
				for _, word := range words {
					value, _, err := theory.solutions.Find(word)
					astTestMust(t, err)
					if value != empty || theory.solutions.IsRoot(word) || state.context.enodes.get(word).Root() != state.context.enodes.get(empty).Root() {
						t.Fatal("originating theory lost its empty solution", word, value)
					}
				}
				astTestMust(t, state.PopScopes(1))
				for _, word := range words {
					if !theory.solutions.IsRoot(word) || state.context.enodes.get(word).Root() == state.context.enodes.get(empty).Root() {
						t.Fatal("empty solution survived pop", word)
					}
				}
			}
		})
	}
}
