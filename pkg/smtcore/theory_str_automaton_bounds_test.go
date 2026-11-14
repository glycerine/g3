package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/automata"
)

func stringBoundsAutomaton(t *testing.T, m *ast.ASTManager, regex ast.ASTHandle) *automata.Automaton {
	t.Helper()
	owner, err := rewriter.NewStringRegexAutomaton(m, regex, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, owner.Close()) })
	graph, err := owner.Graph()
	astTestMust(t, err)
	graph.Compress()
	return graph
}

func TestSMTStringAutomatonBoundsConcreteRegexLanguages(t *testing.T) {
	m, _, _, _ := stringLibraryFixture(t)
	pin := func(value ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(t, err)
		astTestPin(t, m, value)
		return value
	}
	a, ab, bbb, empty := stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "ab"), stringRegexLiteral(t, m, "bbb"), stringRegexLiteral(t, m, "")
	for _, test := range []struct {
		name  string
		regex ast.ASTHandle
	}{
		{"finite", pin(m.MkRegexUnion(a, bbb))},
		{"epsilon-or-three", pin(m.MkRegexUnion(empty, bbb))},
		{"all-lengths", pin(m.MkRegexStar(a))},
		{"even-lengths", pin(m.MkRegexStar(ab))},
		{"positive-even", pin(m.MkRegexPlus(ab))},
		{"only-epsilon", empty},
	} {
		t.Run(test.name, func(t *testing.T) {
			graph := stringBoundsAutomaton(t, m, test.regex)
			accepts := func(n int64) bool {
				switch test.name {
				case "finite":
					return n == 1 || n == 3
				case "epsilon-or-three":
					return n == 0 || n == 3
				case "all-lengths":
					return n >= 0
				case "even-lengths":
					return n >= 0 && n%2 == 0
				case "positive-even":
					return n > 0 && n%2 == 0
				default:
					return n == 0
				}
			}
			for bound := uint32(0); bound <= 7; bound++ {
				at, next, err := refineStringAutomatonLowerBound(graph, bound)
				astTestMust(t, err)
				wantNext := int64(-1)
				for n := int64(bound) + 1; n < 20; n++ {
					if accepts(n) {
						wantNext = n
						break
					}
				}
				if at != accepts(int64(bound)) || next != wantNext {
					t.Fatal("lower-bound graph traversal disagrees with independent language lengths", bound, at, next, wantNext)
				}
				at, previous, err := refineStringAutomatonUpperBound(graph, bound)
				astTestMust(t, err)
				wantPrevious := int64(-1)
				for n := int64(0); n < int64(bound); n++ {
					if accepts(n) {
						wantPrevious = n
					}
				}
				if at != accepts(int64(bound)) || previous != wantPrevious {
					t.Fatal("upper-bound graph traversal disagrees with independent language lengths", bound, at, previous, wantPrevious)
				}
			}
		})
	}
}

func TestSMTStringAutomatonBoundsEpsilonCyclesEmptyAndNil(t *testing.T) {
	graph := automata.New(0, []uint32{3}, []automata.Move{automata.EpsilonMove(0, 1), automata.EpsilonMove(1, 0), automata.ConditionMove(1, 2, 12345), automata.EpsilonMove(2, 3)})
	for bound := uint32(0); bound < 4; bound++ {
		at, next, err := refineStringAutomatonLowerBound(graph, bound)
		astTestMust(t, err)
		wantNext := int64(-1)
		if bound == 0 {
			wantNext = 1
		}
		if at != (bound == 1) || next != wantNext {
			t.Fatal("epsilon closure counted as consuming a character", bound, at, next)
		}
		at, previous, err := refineStringAutomatonUpperBound(graph, bound)
		astTestMust(t, err)
		wantPrevious := int64(-1)
		if bound > 1 {
			wantPrevious = 1
		}
		if at != (bound == 1) || previous != wantPrevious {
			t.Fatal("epsilon cycle changed upper refinement", bound, at, previous)
		}
	}
	if at, next, err := refineStringAutomatonLowerBound(automata.Empty(), 3); err != nil || at || next != -1 {
		t.Fatal("empty lower-bound sentinel changed", at, next, err)
	}
	if at, previous, err := refineStringAutomatonUpperBound(automata.Empty(), 3); err != nil || at || previous != -1 {
		t.Fatal("empty upper-bound sentinel changed", at, previous, err)
	}
	if _, _, err := refineStringAutomatonLowerBound(nil, 0); err == nil {
		t.Fatal("nil lower-bound graph bypassed native precondition")
	}
	if _, _, err := refineStringAutomatonUpperBound(nil, 0); err == nil {
		t.Fatal("nil upper-bound graph bypassed native precondition")
	}
}

func TestSMTStringAutomatonInitialBoundsGuardsAndPersistentCache(t *testing.T) {
	for _, allowsZero := range []bool{false, true} {
		m, theory, state, rw := stringLibraryFixture(t)
		word := stringGroundingWord(t, theory, "aut-bound-word")
		regex := stringRegexLiteral(t, m, "abc")
		if allowsZero {
			var err error
			regex, err = m.MkRegexUnion(stringRegexLiteral(t, m, ""), regex)
			astTestMust(t, err)
			astTestPin(t, m, regex)
		}
		membership := stringRegexRegister(t, theory, word, regex)
		graph := stringBoundsAutomaton(t, m, regex)
		astTestMust(t, state.PushScope())
		astTestMust(t, theory.findStringAutomatonInitialBounds(membership, graph))
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		length, err := m.MkSequenceLength(word)
		astTestMust(t, err)
		for n := -1; n <= 5; n++ {
			for _, positive := range []bool{false, true} {
				numeral, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(n))
				astTestMust(t, err)
				bit, err := m.MkBoolVal(positive)
				astTestMust(t, err)
				want := !positive || n >= 3 || allowsZero && n == 0
				if got := stringLibraryAxiomsTruth(t, m, rw, []ast.ASTHandle{axiom}, map[ast.ASTHandle]ast.ASTHandle{membership: bit, length: numeral}); got != want {
					t.Fatal("initial automaton bound lost its polarity/zero guard", allowsZero, n, positive, got)
				}
			}
		}
		astTestMust(t, state.PopScopes(1))
		want := "3"
		if allowsZero {
			want = "0"
		}
		if value := theory.regexLastLower[word]; value == nil || value.String() != want {
			t.Fatal("last lower bound did not retain native word-keyed lifetime", value, want)
		}
	}
}

// smtcore_12-F1: both nonzero_lower_bound == -1 arms of
// find_automaton_initial_bounds are UNREACHABLE(); the release build takes
// the ordinary arm with the sentinel -1 as the bound and keeps solving.
func TestSMTStringAutomatonInitialBoundsNoNonzeroSolutionReleaseArm(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "aut-bound-empty-word")
	regex := stringRegexLiteral(t, m, "")
	membership, err := m.MkSequenceInRegex(word, regex)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(membership, false))
	before := len(theory.refs)
	if err := theory.findStringAutomatonInitialBounds(membership, automata.Epsilon()); err != nil || len(theory.refs) == before {
		t.Fatal("epsilon-only initial bound did not assert the release-build lemma", err)
	}
	if value := theory.regexLastLower[word]; value == nil || value.Sign() != 0 {
		t.Fatal("zero-bound publication was lost", value)
	}
	delete(theory.regexLastLower, word)
	before = len(theory.refs)
	if err := theory.findStringAutomatonInitialBounds(membership, automata.Empty()); err != nil || len(theory.refs) == before {
		t.Fatal("empty language did not assert the release-build lemma", err)
	}
	if value := theory.regexLastLower[word]; value == nil || value.Int64() != -1 {
		t.Fatal("empty language did not record the -1 sentinel bound", value)
	}
}
