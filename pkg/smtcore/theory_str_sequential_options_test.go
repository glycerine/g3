package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTStringSequentialOptionsCreationRefreshAndMore(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "seq-options-word")
	root, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.lengthIndicators[word][0]
	if theory.testers.lengthCounts[word] != 1 || theory.testers.lengthSubjects[first] != word {
		t.Fatal("first length counter/reverse registration missing")
	}
	fresh, before := theory.freshID, len(theory.basicTodo)
	root, err = theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || len(theory.basicTodo) != before+1 || theory.basicTodo[before].Owner() != first {
		t.Fatal("unassigned first tester not refreshed")
	}
	root, err = theory.generateStringSequentialOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	entries := theory.testers.lengthIndicators[word]
	if len(entries) != 2 || theory.testers.lengthCounts[word] != 2 {
		t.Fatal("more did not append second length window")
	}
	second := entries[1]
	fresh = theory.freshID
	root, err = theory.generateStringSequentialOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || theory.basicTodo[len(theory.basicTodo)-1].Owner() != second {
		t.Fatal("previous callback did not refresh next unassigned window")
	}
	// The earliest unresolved tester takes precedence over a later callback.
	root, err = theory.generateStringSequentialOptions(word, second, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || theory.basicTodo[len(theory.basicTodo)-1].Owner() != first {
		t.Fatal("later callback skipped unresolved first window")
	}
	more := astTestString(t, m, "more")
	astTestMust(t, state.Internalize(more, false))
	stringDependencyPrivateMerge(t, theory, first, more)
	root, err = theory.generateStringSequentialOptions(word, second, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(theory.testers.lengthIndicators[word]) != 3 || theory.testers.lengthCounts[word] != 3 {
		t.Fatal("assigned-more prefix did not advance to third window")
	}
}

func TestSMTStringSequentialOptionsConcreteCallbackAndPrivateValue(t *testing.T) {
	for _, assigned := range []bool{false, true} {
		t.Run(map[bool]string{false: "callback-before-merge", true: "private-value-precedes-callback"}[assigned], func(t *testing.T) {
			m, theory, state, rw := stringLibraryFixture(t)
			word := stringGroundingWord(t, theory, "seq-concrete")
			root, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
			astTestMust(t, err)
			astTestMust(t, m.DelRef(&root))
			length := theory.testers.lengthIndicators[word][0]
			callback := "2"
			if assigned {
				literal := astTestString(t, m, "2")
				astTestMust(t, state.Internalize(literal, false))
				stringDependencyPrivateMerge(t, theory, length, literal)
				callback = "more"
			}
			root, err = theory.generateStringSequentialOptions(word, length, callback)
			astTestMust(t, err)
			formula, err := m.RefGet(root)
			astTestMust(t, err)
			entries := theory.testers.valueIndicators[word][2]
			if len(entries) != 1 || theory.testers.lengthCounts[word] != 1 {
				t.Fatal("concrete length did not start value testing")
			}
			indicator := entries[0].variable
			for _, text := range []string{"aa", "ab"} {
				if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), length: astTestString(t, m, "2"), indicator: astTestString(t, m, "0")}); got != (text == "aa") {
					t.Fatal("concrete callback used wrong value length", text, got)
				}
			}
			astTestMust(t, m.DelRef(&root))
		})
	}
}

func TestSMTStringSequentialOptionsInactiveResetAndScopeRefresh(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "seq-scope")
	astTestMust(t, state.PushScope())
	root, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.lengthIndicators[word][0]
	astTestMust(t, state.PopScopes(1))
	if state.context.enodes.get(first) != nil {
		t.Fatal("test requires popped enode")
	}
	fresh := theory.freshID
	root, err = theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || state.context.enodes.get(first) == nil {
		t.Fatal("scope pop reset lifetime-active history")
	}
	delete(theory.internalVariables, first)
	theory.testers.lengthCounts[word] = 9
	root, err = theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	entries := theory.testers.lengthIndicators[word]
	if len(entries) != 1 || entries[0] == first || theory.testers.lengthCounts[word] != 1 || theory.testers.lengthSubjects[first] != word {
		t.Fatal("length reset failed to clear history while retaining reverse cache")
	}
	second := entries[0]
	delete(theory.testers.lengthCounts, word)
	root, err = theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if entries = theory.testers.lengthIndicators[word]; len(entries) != 1 || entries[0] == second {
		t.Fatal("absent count did not reset otherwise active history")
	}
}

func TestSMTStringSequentialOptionsPreviousFallbackAndNativeGuard(t *testing.T) {
	m, theory, _, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "seq-fallback")
	root, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	first := theory.testers.lengthIndicators[word][0]
	root, err = theory.generateStringSequentialOptions(word, first, "more")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	delete(theory.internalVariables, first)
	fresh := theory.freshID
	if _, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, ""); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unassigned noncallback previous tester bypassed native guard", err)
	}
	if theory.freshID != fresh {
		t.Fatal("guard failure created a tester")
	}
	// Fallback may use the immediately preceding callback even though that
	// previous entry was skipped by the active-membership scan.
	root, err = theory.generateStringSequentialOptions(word, first, "1")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(theory.testers.valueIndicators[word][1]) != 1 {
		t.Fatal("previous callback value not used after skipped history entry")
	}
}

func TestSMTStringSequentialOptionsRegexSubtermBlocksValues(t *testing.T) {
	for _, shape := range []string{"direct", "nested", "private-alias", "unrelated", "disabled"} {
		t.Run(shape, func(t *testing.T) {
			m, theory, state, _ := stringRegexStageFixture(t)
			word := stringGroundingWord(t, theory, "seq-regex-word")
			other := stringGroundingWord(t, theory, "seq-regex-other")
			root, err := theory.generateStringSequentialOptions(word, ast.ASTHandle{}, "")
			astTestMust(t, err)
			astTestMust(t, m.DelRef(&root))
			length := theory.testers.lengthIndicators[word][0]
			term := word
			if shape == "nested" || shape == "private-alias" {
				child := word
				if shape == "private-alias" {
					child = other
					stringDependencyPrivateMerge(t, theory, word, other)
				}
				term = stringDependencyConcat(t, theory, astTestString(t, m, "prefix"), child)
			} else if shape == "unrelated" {
				term = other
			}
			membership := stringRegexRegister(t, theory, term, stringRegexLiteral(t, m, "a"))
			// A false membership is still a constraint for this source guard.
			literal, err := state.Literal(membership)
			astTestMust(t, err)
			literal.Negate()
			astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
			if shape == "disabled" {
				context := theory.parameterOwner.Context
				params, err := context.NewValue()
				astTestMust(t, err)
				key := "str.regex_automata"
				astTestMust(t, context.SetBoolName(&params, &key, false))
				astTestMust(t, theory.parameterOwner.Update(&params))
				astTestMust(t, context.Del(&params))
			}
			root, err = theory.generateStringSequentialOptions(word, length, "1")
			astTestMust(t, err)
			blocked := shape != "unrelated" && shape != "disabled"
			if (root == (ast.ASTRef{})) != blocked || (len(theory.testers.valueIndicators[word][1]) == 0) != blocked {
				t.Fatal("regex term/subterm/class guard changed", shape)
			}
			if root != (ast.ASTRef{}) {
				astTestMust(t, m.DelRef(&root))
			}
		})
	}
}
