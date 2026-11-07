package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTSequenceConflictExplanationSkipsCommonExpansion(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m := astTestManager(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			family, err := m.SequenceFamilyID()
			astTestMust(t, err)
			theory, err := NewSMTSequenceTheory(family)
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			astTestMust(t, state.RegisterTheory(theory))
			a, err := m.MkString("a")
			astTestMust(t, err)
			b, err := m.MkString("b")
			astTestMust(t, err)
			sort, err := m.GetSort(a)
			astTestMust(t, err)
			var words []ast.ASTHandle
			for _, name := range []string{"shared-expansion", "shared-left", "shared-right"} {
				word, err := m.MkConst(astTestSymbol(t, m, name), sort)
				astTestMust(t, err)
				words = append(words, word)
			}
			alias, err := m.MkSequenceConcat(words[1], words[2])
			astTestMust(t, err)
			aliasEquality, err := m.MkEq(words[0], alias)
			astTestMust(t, err)
			ab, err := m.MkSequenceConcat(a, b)
			astTestMust(t, err)
			left, err := m.MkSequenceConcat(words[0], ab)
			astTestMust(t, err)
			prefix, err := m.MkSequenceConcat(words[0], a)
			astTestMust(t, err)
			right, err := m.MkSequenceConcat(prefix, b)
			astTestMust(t, err)
			equality, err := m.MkEq(left, right)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(aliasEquality, true))
			astTestMust(t, state.Internalize(equality, true))
			aliasLiteral, err := state.Literal(aliasEquality)
			astTestMust(t, err)
			wordLiteral, err := state.Literal(equality)
			astTestMust(t, err)
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, state.PushScope())
				astTestMust(t, state.AssignLiteral(aliasLiteral, NewSMTBooleanAxiom()))
				dependency := theory.dependencies.Leaf(smtSequenceAssumption{literal: aliasLiteral})
				astTestMust(t, theory.solutions.Update(words[0], alias, dependency))
				astTestMust(t, state.AssignLiteral(wordLiteral.Negated(), NewSMTBooleanAxiom()))
				_, err = state.PropagateCore()
				astTestMust(t, err)
				progress, err := theory.checkReducedDisequations()
				astTestMust(t, err)
				conflict, found := state.Conflict()
				owner, _ := conflict.Justification()
				ext, ok := owner.(*SMTTheoryConflictJustification)
				if !progress || !found || !ok || len(ext.Literals) != 1 || ext.Literals[0] != wordLiteral.Negated() {
					t.Fatal("associativity conflict retained the unnecessary shared expansion premise", owner)
				}
				astTestMust(t, state.PopScopes(1))
				if state.Inconsistent() || !theory.solutions.IsRoot(words[0]) {
					t.Fatal("explanation conflict or scoped solution survived pop")
				}
			}
		})
	}
}

func TestSMTSequenceUnitExplanationRetainsWordAndElementPremises(t *testing.T) {
	m := astTestManager(t)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	elementSort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "explanation-element-sort"), nil)
	astTestMust(t, err)
	parameter, err := m.NewParameterAST(elementSort)
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 0, []ast.ASTParameter{parameter})
	astTestMust(t, err)
	astTestMust(t, m.DelParameter(&parameter))
	var elements []ast.ASTHandle
	for _, name := range []string{"explanation-a", "explanation-b", "explanation-c"} {
		value, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		astTestPin(t, m, value)
		elements = append(elements, value)
	}
	var units []ast.ASTHandle
	for _, element := range elements[:2] {
		unit, err := m.MkSequenceUnit(element)
		astTestMust(t, err)
		astTestPin(t, m, unit)
		units = append(units, unit)
	}
	sequenceSort, err := m.GetSort(units[0])
	astTestMust(t, err)
	var words []ast.ASTHandle
	for _, name := range []string{"explanation-x", "explanation-y"} {
		word, err := m.MkConst(astTestSymbol(t, m, name), sequenceSort)
		astTestMust(t, err)
		astTestPin(t, m, word)
		words = append(words, word)
	}
	leaf := func(variable int32) *smtSequenceDependency {
		return theory.dependencies.Leaf(smtSequenceAssumption{literal: NewSMTLiteral(variable, false)})
	}
	seed := leaf(10)
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, theory.solutions.PushScope())
		astTestMust(t, theory.solutions.Update(words[0], units[0], leaf(2)))
		astTestMust(t, theory.solutions.Update(words[1], units[1], leaf(3)))
		astTestMust(t, theory.solutions.Update(elements[0], elements[2], leaf(4)))
		astTestMust(t, theory.solutions.Update(elements[1], elements[2], leaf(5)))
		explained, dependency, err := theory.explainWordEquality(words[0], words[1], seed)
		astTestMust(t, err)
		assumptions := theory.dependencies.Linearize(dependency)
		want := []int32{10, 2, 3, 4, 5}
		if !explained || len(assumptions) != len(want) {
			t.Fatal("unit recursion omitted word-to-unit or element-equality premises", explained, assumptions)
		}
		for i, variable := range want {
			if assumptions[i].literal != NewSMTLiteral(variable, false) {
				t.Fatal("unit explanation premise order", assumptions)
			}
		}
		astTestMust(t, theory.solutions.PopScopes(1))
		explained, dependency, err = theory.explainWordEquality(words[0], words[1], seed)
		astTestMust(t, err)
		if explained || dependency != seed {
			t.Fatal("explanation reused cells or solutions from a popped scope")
		}
	}
}

func TestSMTSequenceExplanationEmptyLeavesPartialFailureAndPop(t *testing.T) {
	m := astTestManager(t)
	family, err := m.SequenceFamilyID()
	astTestMust(t, err)
	theory, err := NewSMTSequenceTheory(family)
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, state.RegisterTheory(theory))
	empty, err := m.MkString("")
	astTestMust(t, err)
	astTestPin(t, m, empty)
	sort, err := m.GetSort(empty)
	astTestMust(t, err)
	var words []ast.ASTHandle
	for _, name := range []string{"empty-explanation", "left-explanation", "right-explanation"} {
		word, err := m.MkConst(astTestSymbol(t, m, name), sort)
		astTestMust(t, err)
		astTestPin(t, m, word)
		words = append(words, word)
	}
	prefix, err := m.MkSequenceConcat(words[0], words[1])
	astTestMust(t, err)
	astTestPin(t, m, prefix)
	suffix, err := m.MkSequenceConcat(words[1], words[0])
	astTestMust(t, err)
	astTestPin(t, m, suffix)
	seed := theory.dependencies.Leaf(smtSequenceAssumption{literal: NewSMTLiteral(2, false)})
	removed := theory.dependencies.Leaf(smtSequenceAssumption{literal: NewSMTLiteral(3, false)})
	for attempt := 0; attempt < 2; attempt++ {
		astTestMust(t, theory.solutions.PushScope())
		astTestMust(t, theory.solutions.Update(words[0], empty, removed))
		for _, pair := range [][2]ast.ASTHandle{{prefix, words[1]}, {words[1], prefix}, {suffix, words[1]}, {words[1], suffix}, {prefix, words[2]}} {
			explained, dependency, err := theory.explainWordEquality(pair[0], pair[1], seed)
			astTestMust(t, err)
			assumptions := theory.dependencies.Linearize(dependency)
			if explained != (pair[1] != words[2]) || len(assumptions) != 2 || assumptions[0].literal != NewSMTLiteral(2, false) || assumptions[1].literal != NewSMTLiteral(3, false) {
				t.Fatal("empty-leaf matching or partial failure lost dependency order", explained, assumptions)
			}
		}
		explained, dependency, err := theory.explainWordEquality(words[0], words[0], seed)
		astTestMust(t, err)
		if !explained || dependency != seed {
			t.Fatal("identical expressions used an unnecessary solution premise")
		}
		astTestMust(t, theory.solutions.PopScopes(1))
		explained, _, err = theory.explainWordEquality(prefix, words[1], nil)
		astTestMust(t, err)
		if explained {
			t.Fatal("empty solution explanation survived pop")
		}
	}
}

func TestSMTSequenceSingletonEqualityEnqueuesElementConsequence(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			m := astTestManager(t)
			if proofs {
				astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			}
			family, err := m.SequenceFamilyID()
			astTestMust(t, err)
			theory, err := NewSMTSequenceTheory(family)
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			astTestMust(t, state.RegisterTheory(theory))
			sort, err := m.MkUninterpretedSort(astTestSymbol(t, m, "singleton-elements"), nil)
			astTestMust(t, err)
			var elements, units []ast.ASTHandle
			for _, name := range []string{"singleton-left", "singleton-right"} {
				element, err := m.MkConst(astTestSymbol(t, m, name), sort)
				astTestMust(t, err)
				unit, err := m.MkSequenceUnit(element)
				astTestMust(t, err)
				astTestMust(t, state.Internalize(unit, false))
				elements = append(elements, element)
				units = append(units, unit)
			}
			equality, err := m.MkEq(units[0], units[1])
			astTestMust(t, err)
			astTestMust(t, state.Internalize(equality, true))
			literal, err := state.Literal(equality)
			astTestMust(t, err)
			for attempt := 0; attempt < 2; attempt++ {
				astTestMust(t, state.PushScope())
				astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
				_, err = state.PropagateCore()
				astTestMust(t, err)
				left, right := state.context.enodes.get(elements[0]), state.context.enodes.get(elements[1])
				if left.root == right.root {
					t.Fatal("element equality preceded sequence reduction")
				}
				progress, err := theory.solveWordEquations()
				astTestMust(t, err)
				if !progress || left.root == right.root || state.pendingHead == len(state.pending) {
					t.Fatal("unit injectivity did not enqueue an element consequence", progress)
				}
				request := state.pending[len(state.pending)-1]
				owner, _ := request.justification.Justification()
				ext, ok := owner.(*SMTExtTheoryEqualityPropagationJustification)
				if !ok || len(ext.Literals) != 1 || ext.Literals[0] != literal {
					t.Fatal("unit element consequence lost its asserted word premise", owner)
				}
				_, err = state.PropagateCore()
				astTestMust(t, err)
				if state.Inconsistent() || left.root != right.root {
					t.Fatal("element consequence failed in core propagation")
				}
				astTestMust(t, state.PopScopes(1))
				if left.root == right.root {
					t.Fatal("unit element equality survived scope pop")
				}
			}
		})
	}
}
