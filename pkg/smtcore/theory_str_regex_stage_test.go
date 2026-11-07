package smtcore

import (
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringRegexStageFixture(t *testing.T) (*ast.ASTManager, *SMTStringTheory, *SMTInternalizerState, *rewriter.TheoryRewriter) {
	t.Helper()
	m, parent, theory := stringExpressionSolverFixture(t)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	arithmetic, err := NewSMTMixedLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, parent.internalizer.RegisterTheory(arithmetic))
	return m, theory, parent.internalizer, parent.preprocessor
}

func stringRegexStageOption(t *testing.T, theory *SMTStringTheory, key string, value uint32) {
	t.Helper()
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	astTestMust(t, context.SetUIntName(&params, &key, value))
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
}

func stringRegexStageAssign(t *testing.T, theory *SMTStringTheory, member ast.ASTHandle, value lbool.LBool) {
	t.Helper()
	astTestMust(t, theory.state.Internalize(member, true))
	astTestMust(t, theory.state.markRelevant(member))
	literal, err := theory.state.Literal(member)
	astTestMust(t, err)
	if value == lbool.False {
		literal = literal.Negated()
	}
	astTestMust(t, theory.state.AssignLiteral(literal, NewSMTBooleanAxiom()))
}

func TestSMTStringRegexStageConstructionThenExactPathAndCache(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	word := stringContainmentWord(t, theory, "stage-exact-word")
	regex := stringRegexLiteral(t, m, "ab")
	member := stringRegexRegister(t, theory, word, regex)
	stringRegexStageAssign(t, theory, member, lbool.True)
	stringSetLength(t, m, state, word, "2")
	added, err := theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if !added || len(theory.regexStage.owned) != 1 || theory.regexStage.automata[regex] == nil {
		t.Fatal("first stage did not construct/cache the full positive automaton")
	}
	if _, done := theory.regexStage.pathDone[member]; done {
		t.Fatal("construction also generated a path in the same source branch")
	}
	added, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if !added || len(theory.stringChars[word]) != 2 || theory.regexStage.lengthAttempts[regex] != 1 {
		t.Fatal("cached exact path was not generated", added, len(theory.stringChars[word]), theory.regexStage.lengthAttempts[regex])
	}
	if _, done := theory.regexStage.pathDone[member]; !done {
		t.Fatal("successful path was not registered")
	}
	fresh, owned := theory.freshID, len(theory.regexStage.owned)
	added, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if added || theory.freshID != fresh || len(theory.regexStage.owned) != owned || theory.regexStage.lengthAttempts[regex] != 1 {
		t.Fatal("completed path repeated native scoped work")
	}
}

func TestSMTStringRegexStageZeroPathsBothPolarities(t *testing.T) {
	for _, tc := range []struct{ positive, epsilon, done bool }{{true, true, true}, {true, false, false}, {false, true, false}, {false, false, true}} {
		m, theory, state, _ := stringRegexStageFixture(t)
		word := stringContainmentWord(t, theory, "stage-zero-word")
		regex := stringRegexLiteral(t, m, "a")
		if tc.epsilon {
			var err error
			regex, err = m.MkRegexStar(regex)
			astTestMust(t, err)
			astTestPin(t, m, regex)
		}
		member := stringRegexRegister(t, theory, word, regex)
		assignment := lbool.True
		if !tc.positive {
			assignment = lbool.False
		}
		stringRegexStageAssign(t, theory, member, assignment)
		stringSetLength(t, m, state, word, "0")
		// Run construction then path dispatch without an intervening core
		// propagation pass that could discharge the conflicting cases first.
		// Every language still has a nonzero solution for initial bounds.
		_, err := theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		_, err = theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		_, done := theory.regexStage.pathDone[member]
		if done != tc.done || theory.regexStage.lengthAttempts[regex] != 1 || len(theory.stringChars[word]) != 0 {
			t.Fatal("native zero acceptance/polarity/conflict path changed", tc, done, theory.regexStage.lengthAttempts[regex])
		}
	}
}

func TestSMTStringRegexStageScopedLengthFormulaReuse(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	word := stringContainmentWord(t, theory, "stage-length-cache")
	star, err := m.MkRegexStar(stringRegexLiteral(t, m, "b"))
	astTestMust(t, err)
	astTestPin(t, m, star)
	regex, err := m.MkRegexConcat(stringRegexLiteral(t, m, "a"), star)
	astTestMust(t, err)
	astTestPin(t, m, regex)
	astTestMust(t, state.PushScope())
	member := stringRegexRegister(t, theory, word, regex)
	stringRegexStageAssign(t, theory, member, lbool.True)
	_, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	formula := theory.regexStage.lengthFormulas[member]
	variables := append([]ast.ASTHandle(nil), theory.regexStage.lengthVariables[member]...)
	if len(variables) != 3 {
		t.Fatal("concat/star length formula did not retain its native witnesses", len(variables))
	}
	owner, fresh := theory.regexStage.automata[regex], theory.freshID
	astTestMust(t, state.PopScopes(1))
	if _, done := theory.regexStage.lengthDone[member]; done {
		t.Fatal("scoped length-registration flag survived pop")
	}
	if theory.regexStage.lengthFormulas[member] != formula || theory.regexStage.automata[regex] != owner {
		t.Fatal("persistent formula/automaton cache was discarded")
	}
	stringRegexStageAssign(t, theory, member, lbool.True)
	_, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if theory.freshID != fresh || theory.regexStage.lengthFormulas[member] != formula {
		t.Fatal("scope replay replaced cached witnesses/formula")
	}
	for _, variable := range variables {
		if state.context.enodes.get(variable) == nil {
			t.Fatal("cached length variable was not refreshed")
		}
	}
}

func TestSMTStringRegexStageConstructionEffortAndUndefinedGate(t *testing.T) {
	m, theory, _, _ := stringRegexStageFixture(t)
	// Updating the concrete params owner keeps all source defaults for fields
	// not supplied by this call, so set both controls in one retained update.
	context := theory.parameterOwner.Context
	p, err := context.NewValue()
	astTestMust(t, err)
	k := "str.regex_automata_difficulty_threshold"
	astTestMust(t, context.SetUIntName(&p, &k, 0))
	k = "str.regex_automata_failed_automaton_threshold"
	astTestMust(t, context.SetUIntName(&p, &k, 2))
	astTestMust(t, theory.parameterOwner.Update(&p))
	astTestMust(t, context.Del(&p))
	word := stringContainmentWord(t, theory, "stage-effort-word")
	regex := stringRegexLiteral(t, m, "a")
	member := stringRegexRegister(t, theory, word, regex)
	added, err := theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if added || len(theory.regexStage.failures) != 0 || len(theory.regexStage.owned) != 0 {
		t.Fatal("undefined membership performed construction work")
	}
	stringRegexStageAssign(t, theory, member, lbool.False)
	for attempt := uint32(1); attempt <= 2; attempt++ {
		added, err = theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		if added || theory.regexStage.failures[member] != attempt || len(theory.regexStage.owned) != 0 {
			t.Fatal("construction effort threshold changed", attempt, added, theory.regexStage.failures[member])
		}
	}
	added, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if !added || theory.regexStage.automata[regex] == nil || theory.regexStage.failures[member] != 2 {
		t.Fatal("failure threshold did not trigger positive unbounded construction")
	}
}

func TestSMTStringRegexStageBoundConsequencesAndDisabledLowerCase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		upper  bool
		bound  int64
		accept func(int64) bool
	}{
		{"upper-gap", true, 2, func(n int64) bool { return n > 2 || n <= 1 }},
		{"upper-at", true, 3, func(n int64) bool { return n > 3 || n == 3 || n <= 1 }},
		{"lower-gap", false, 2, func(n int64) bool { return n < 2 || n >= 3 }},
		{"lower-at-last", false, 3, func(n int64) bool { return n < 3 || n == 3 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, theory, _, rw := stringRegexStageFixture(t)
			word := stringContainmentWord(t, theory, "stage-bound-word")
			regex, err := m.MkRegexUnion(stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "bbb"))
			astTestMust(t, err)
			astTestPin(t, m, regex)
			member := stringRegexRegister(t, theory, word, regex)
			owner, err := rewriter.NewStringRegexAutomaton(m, regex, nil)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, owner.Close()) })
			graph, err := owner.Graph()
			astTestMust(t, err)
			graph.Compress()
			length, err := m.MkSequenceLength(word)
			astTestMust(t, err)
			astTestPin(t, m, length)
			frame := smtStringRegexStage{theory: theory, locals: smtStringLengthPremises{theory: theory}}
			astTestMust(t, frame.refineBound(member, length, owner, big.NewInt(tc.bound), tc.upper))
			astTestMust(t, frame.locals.close())
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			for _, truth := range []bool{false, true} {
				var boolean ast.ASTHandle
				if truth {
					boolean, err = m.MkTrue()
				} else {
					boolean, err = m.MkFalse()
				}
				astTestMust(t, err)
				for n := int64(0); n <= 5; n++ {
					values := map[ast.ASTHandle]ast.ASTHandle{member: boolean, word: astTestString(t, m, strings.Repeat("x", int(n)))}
					if got := stringAxiomTruth(t, m, rw, axiom, values); got != (!truth || tc.accept(n)) {
						t.Fatal("guarded refinement changed", tc.name, truth, n, got)
					}
				}
			}
			before := len(theory.refs)
			frame = smtStringRegexStage{theory: theory, locals: smtStringLengthPremises{theory: theory}}
			astTestMust(t, frame.refineBound(member, length, owner, big.NewInt(1), false))
			astTestMust(t, frame.locals.close())
			if frame.added || len(theory.refs) != before {
				t.Fatal("disabled lower equality-or-next axiom became active")
			}
		})
	}
}

func TestSMTStringRegexStageIntersectionConflictPersistsAcrossPop(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	stringRegexStageOption(t, theory, "str.regex_automata_length_attempt_threshold", 0)
	word := stringContainmentWord(t, theory, "stage-conflict-word")
	astTestMust(t, state.PushScope())
	first := stringRegexRegister(t, theory, word, stringRegexLiteral(t, m, "a"))
	second := stringRegexRegister(t, theory, word, stringRegexLiteral(t, m, "b"))
	stringRegexStageAssign(t, theory, first, lbool.True)
	stringRegexStageAssign(t, theory, second, lbool.True)
	added, err := theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if !added || len(theory.regexStage.persisted) != 1 || len(theory.regexStage.owned) != 2 || len(theory.regexStage.products) != 1 {
		t.Fatal("empty product was not retained/persisted", added, len(theory.regexStage.persisted), len(theory.regexStage.owned), len(theory.regexStage.products))
	}
	conflict, err := m.RefGet(theory.regexStage.persisted[0])
	astTestMust(t, err)
	astTestMust(t, theory.Propagate())
	astTestMust(t, state.PopScopes(1))
	if len(theory.regexStage.persistedTodo) != 1 || !theory.CanPropagate() || theory.searchStarted {
		t.Fatal("pop did not queue persisted conflict before search")
	}
	astTestMust(t, theory.Propagate())
	if len(theory.regexStage.persistedTodo) != 0 {
		t.Fatal("replayed conflict remained pending")
	}
	last, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	if last != conflict {
		t.Fatal("scope replay did not assert the retained conflict clause")
	}
	if _, found := state.BoolVar(first); !found {
		t.Fatal("conflict replay did not reinternalize its membership atoms")
	}
}

func TestSMTStringRegexStageIntersectionRetainsNativeSelectionQuirks(t *testing.T) {
	for _, firstAssignment := range []lbool.LBool{lbool.False, lbool.Undef} {
		m, theory, _, _ := stringRegexStageFixture(t)
		stringRegexStageOption(t, theory, "str.regex_automata_length_attempt_threshold", 0)
		a, b := stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "b")
		seedWord := stringContainmentWord(t, theory, "stage-shared-cache-seed")
		seed := stringRegexRegister(t, theory, seedWord, a)
		stringRegexStageAssign(t, theory, seed, lbool.True)
		word := stringContainmentWord(t, theory, "stage-selection-word")
		first := stringRegexRegister(t, theory, word, a)
		second := stringRegexRegister(t, theory, word, b)
		if firstAssignment != lbool.Undef {
			stringRegexStageAssign(t, theory, first, firstAssignment)
		}
		stringRegexStageAssign(t, theory, second, lbool.True)
		_, err := theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		if len(theory.regexStage.persisted) != 1 {
			t.Fatal("native first-positive/unassigned selection behavior changed", firstAssignment, len(theory.regexStage.persisted))
		}
		conflict, err := m.RefGet(theory.regexStage.persisted[0])
		astTestMust(t, err)
		guard := second
		if firstAssignment == lbool.False {
			negative, err := m.MkNot(first)
			astTestMust(t, err)
			astTestPin(t, m, negative)
			guard, err = m.MkAndArray([]ast.ASTHandle{negative, second})
			astTestMust(t, err)
			astTestPin(t, m, guard)
		}
		want, err := m.MkNot(guard)
		astTestMust(t, err)
		astTestPin(t, m, want)
		if conflict != want {
			t.Fatal("source conflict premise selection changed", firstAssignment)
		}
	}
}

func TestSMTStringRegexStageFinalCheckRunsWithoutInventedContinue(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	word := stringContainmentWord(t, theory, "stage-final-check-word")
	astTestMust(t, theory.setUpAxioms(word))
	regex := stringRegexLiteral(t, m, "ab")
	member := stringRegexRegister(t, theory, word, regex)
	stringRegexStageAssign(t, theory, member, lbool.True)
	for attempt := 0; attempt < 8; attempt++ {
		_, err := state.PropagateCore()
		astTestMust(t, err)
		status, err := theory.FinalCheck()
		astTestMust(t, err)
		if theory.regexStage.automata[regex] != nil {
			if status != SMTFinalCheckContinue || len(theory.testers.lengthIndicators[word]) == 0 {
				t.Fatal("regex stage did not reach actual free-variable tester work before continuation", status)
			}
			return
		}
		if status != SMTFinalCheckContinue {
			t.Fatal("actual final checker did not reach regex construction", status)
		}
	}
	t.Fatal("actual final checker failed to advance through its backpropagation prefix")
}

func TestSMTStringRegexStageDisabledAndInitialBoundFailureOwnership(t *testing.T) {
	m, theory, _, _ := stringRegexStageFixture(t)
	word := stringContainmentWord(t, theory, "stage-disabled-word")
	regex := stringRegexLiteral(t, m, "")
	member := stringRegexRegister(t, theory, word, regex)
	stringRegexStageAssign(t, theory, member, lbool.True)
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	key := "str.regex_automata"
	astTestMust(t, context.SetBoolName(&params, &key, false))
	astTestMust(t, theory.parameterOwner.Update(&params))
	added, err := theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if added || len(theory.regexStage.owned) != 0 || len(theory.regexStage.lengthDone) != 0 {
		t.Fatal("disabled stage changed regex state")
	}
	astTestMust(t, context.SetBoolName(&params, &key, true))
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	added, err = theory.runStringRegexAutomataStage()
	if err == nil || !added || len(theory.regexStage.owned) != 1 || theory.regexStage.automata[regex] == nil {
		t.Fatal("native no-nonzero-solution guard lost its prior owned cache publication", added, err)
	}
	if theory.regexLastLower[word] == nil || theory.regexLastLower[word].Sign() != 0 {
		t.Fatal("epsilon-only guard lost native prior zero-cache write")
	}
	owner := theory.regexStage.automata[regex]
	astTestMust(t, theory.Close())
	if _, err := owner.Graph(); err == nil {
		t.Fatal("String close leaked a cached automaton after failed initial bounds")
	}
}

func TestSMTStringRegexStageReadsLiveBoundsAndSkipsRepeatedRefinement(t *testing.T) {
	for _, upper := range []bool{true, false} {
		m, theory, state, rw := stringRegexStageFixture(t)
		word := stringContainmentWord(t, theory, "stage-live-bound-word")
		regex, err := m.MkRegexStar(stringRegexLiteral(t, m, "aa"))
		astTestMust(t, err)
		astTestPin(t, m, regex)
		member := stringRegexRegister(t, theory, word, regex)
		stringRegexStageAssign(t, theory, member, lbool.True)
		_, err = theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		operation := smtArithOpGE
		if upper {
			operation = smtArithOpLE
		}
		stringAssertArithmeticBound(t, m, state, wordLengthForRegexStage(t, m, word), operation, "3", false)
		if _, exact, err := theory.lengthValue(word); err != nil || exact {
			t.Fatal("live-bound fixture unexpectedly has an exact core value", exact, err)
		}
		length := wordLengthForRegexStage(t, m, word)
		bound, known, err := theory.stringArithmeticBound(length, !upper)
		astTestMust(t, err)
		if !known {
			t.Fatal("actual arithmetic endpoint was unavailable", upper)
		}
		before := len(theory.refs)
		added, err := theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		var recorded *big.Int
		if upper {
			recorded = theory.regexStage.lastUpper[word]
		} else {
			recorded = theory.regexLastLower[word]
		}
		if recorded == nil || recorded.Cmp(bound) != 0 {
			t.Fatal("stage did not record the word-keyed live endpoint", recorded, bound)
		}
		shouldPublish := upper || bound.Bit(0) != 0
		if added != shouldPublish {
			t.Fatal("live bound dispatch changed disabled lower-at behavior", upper, bound, added)
		}
		if shouldPublish {
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			truth, err := m.MkTrue()
			astTestMust(t, err)
			b := bound.Int64()
			for n := int64(0); n <= 6; n++ {
				want := n < b || n >= b+1
				if upper {
					if b%2 == 0 {
						want = n > b || n == b || n <= b-2
					} else {
						want = n > b || n <= b-1
					}
				}
				values := map[ast.ASTHandle]ast.ASTHandle{member: truth, word: astTestString(t, m, strings.Repeat("x", int(n)))}
				if got := stringAxiomTruth(t, m, rw, axiom, values); got != want {
					t.Fatal("live bound refinement consequence changed", upper, b, n, got, want)
				}
			}
		} else if len(theory.refs) != before {
			t.Fatal("disabled lower case emitted an axiom")
		}
		added, err = theory.runStringRegexAutomataStage()
		astTestMust(t, err)
		if added {
			t.Fatal("unchanged endpoint repeated refinement")
		}
	}
}

func wordLengthForRegexStage(t *testing.T, m *ast.ASTManager, word ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	length, err := m.MkSequenceLength(word)
	astTestMust(t, err)
	astTestPin(t, m, length)
	return length
}

func TestSMTStringRegexStageEpsilonIntersectionAndEffortPolicy(t *testing.T) {
	m, theory, _, rw := stringRegexStageFixture(t)
	stringRegexStageOption(t, theory, "str.regex_automata_length_attempt_threshold", 0)
	word := stringContainmentWord(t, theory, "stage-epsilon-intersection")
	var members []ast.ASTHandle
	for _, text := range []string{"a", "b"} {
		regex, err := m.MkRegexStar(stringRegexLiteral(t, m, text))
		astTestMust(t, err)
		astTestPin(t, m, regex)
		member := stringRegexRegister(t, theory, word, regex)
		stringRegexStageAssign(t, theory, member, lbool.True)
		members = append(members, member)
	}
	_, err := theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if len(theory.regexStage.products) != 1 || len(theory.regexStage.persisted) != 0 {
		t.Fatal("epsilon-only product was treated as an empty-language conflict")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	for _, text := range []string{"", "a", "b", "ab"} {
		if got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{members[0]: truth, members[1]: truth, word: astTestString(t, m, text)}); got != (text == "") {
			t.Fatal("epsilon intersection did not constrain word and length", text, got)
		}
	}

	m, theory, _, _ = stringRegexStageFixture(t)
	word = stringContainmentWord(t, theory, "stage-intersection-effort")
	for _, text := range []string{"a", "b"} {
		member := stringRegexRegister(t, theory, word, stringRegexLiteral(t, m, text))
		stringRegexStageAssign(t, theory, member, lbool.True)
	}
	_, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if len(theory.regexStage.products) != 0 || len(theory.regexStage.intersectionFail) != 0 {
		t.Fatal("intersection bypassed its default length-attempt threshold")
	}
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	for key, value := range map[string]uint32{"str.regex_automata_length_attempt_threshold": 0, "str.regex_automata_intersection_difficulty_threshold": 0, "str.regex_automata_failed_intersection_threshold": 1} {
		key := key
		astTestMust(t, context.SetUIntName(&params, &key, value))
	}
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	_, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if len(theory.regexStage.products) != 0 || len(theory.regexStage.intersectionFail) != 1 {
		t.Fatal("difficult intersection did not record its first failure")
	}
	_, err = theory.runStringRegexAutomataStage()
	astTestMust(t, err)
	if len(theory.regexStage.products) != 1 || len(theory.regexStage.persisted) != 1 {
		t.Fatal("intersection failure threshold did not force the empty product")
	}
}
