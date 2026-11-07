package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func stringLegacyRegexFixture(t *testing.T) (*ast.ASTManager, *SMTStringTheory, *SMTInternalizerState, model.ModelEvaluatorConfig) {
	t.Helper()
	m, config := smtModelGeneratorFixture(t)
	parameters, err := NewSMTStrParams(config.Context, config.Params, config.Globals, m.Budget())
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	key := "str.regex_automata"
	astTestMust(t, config.Context.SetBoolName(config.Params, &key, false))
	astTestMust(t, parameters.Update(config.Params))
	theory, state := stringTheoryFixture(t, m)
	theory.parameterOwner = parameters
	return m, theory, state, config
}

func stringRegexLiteral(t *testing.T, m *ast.ASTManager, text string) ast.ASTHandle {
	t.Helper()
	regex, err := m.MkSequenceToRegex(astTestString(t, m, text))
	astTestMust(t, err)
	astTestPin(t, m, regex)
	return regex
}

func stringRegexRegister(t *testing.T, theory *SMTStringTheory, word, regex ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	expression, err := theory.state.manager.MkSequenceInRegex(word, regex)
	astTestMust(t, err)
	astTestMust(t, theory.state.Internalize(expression, true))
	astTestMust(t, theory.instantiateRegexMembership(theory.state.context.enodes.get(expression)))
	return expression
}

func TestSMTStringRegexTextKeysAndAutomataRegistries(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	a, b := stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "b")
	concat, err := m.MkRegexConcat(a, b)
	astTestMust(t, err)
	union, err := m.MkRegexUnion(a, b)
	astTestMust(t, err)
	star, err := m.MkRegexStar(a)
	astTestMust(t, err)
	rangeRE, err := m.MkRegexRange(astTestString(t, m, "z"), astTestString(t, m, "a"))
	astTestMust(t, err)
	sort, err := m.GetSort(a)
	astTestMust(t, err)
	full, err := m.MkRegexFull(sort)
	astTestMust(t, err)
	char, err := m.MkRegexFullChar(sort)
	astTestMust(t, err)
	for _, test := range []struct {
		regex ast.ASTHandle
		key   string
	}{{concat, "(a)(b)"}, {union, "(a)|(b)"}, {star, "(a)*"}, {rangeRE, "[z-a]"}, {full, "(.*)"}, {char, "str.allchar"}, {stringRegexLiteral(t, m, "\\^$.|?*+()[{]}\x00\xff"), "\\\\\\^\\$\\.\\|\\?\\*\\+\\(\\)\\[\\{]}\xff"}} {
		key, err := theory.regexTextKey(test.regex)
		astTestMust(t, err)
		if key != test.key {
			t.Fatalf("native registry key: got %q, want %q", key, test.key)
		}
	}
	x := stringContainmentWord(t, theory, "regex-automata-word")
	symbolic, err := m.MkSequenceToRegex(x)
	astTestMust(t, err)
	if key, err := theory.regexTextKey(symbolic); err != nil || key != "" {
		t.Fatal("native nonliteral to_re registry key changed", key, err)
	}
	option, err := m.MkRegexOption(a)
	astTestMust(t, err)
	if _, err := theory.regexTextKey(option); err == nil {
		t.Fatal("unsupported native registry operation was silently accepted")
	}
	astTestMust(t, state.PushScope())
	first := stringRegexRegister(t, theory, x, stringRegexLiteral(t, m, "\x00"))
	second := stringRegexRegister(t, theory, x, stringRegexLiteral(t, m, ""))
	if len(theory.regexMemberships[x]) != 1 || theory.regexMemberships[x][""] != second || len(theory.regexTermsByString[x]) != 2 || len(theory.regexTerms) != 2 {
		t.Fatal("text-key overwrite or automata per-string registration diverged")
	}
	before := len(theory.refs)
	astTestMust(t, theory.instantiateRegexMembership(state.context.enodes.get(first)))
	if len(theory.refs) != before || len(theory.regexTermsByString[x]) != 2 {
		t.Fatal("duplicate regex setup changed registration")
	}
	astTestMust(t, state.PopScopes(1))
	if len(theory.regexTerms) != 2 || theory.regexMemberships[x][""] != second {
		t.Fatal("native persistent regex registry was cleared on pop")
	}
	if _, err := m.GetArg(first, 1); err != nil {
		t.Fatal("registry lost ownership after pop", err)
	}
	fresh, err := theory.Fresh()
	astTestMust(t, err)
	defer func() { astTestMust(t, fresh.Close()) }()
	if len(fresh.regexTerms) != 0 || len(fresh.regexMemberships) != 0 || len(fresh.regexNFACache) != 0 {
		t.Fatal("fresh owner inherited mutable regex state")
	}
}

func TestSMTStringLegacyRegexLeafAxioms(t *testing.T) {
	for _, operation := range []string{"literal", "range", "full", "char"} {
		m, theory, _, config := stringLegacyRegexFixture(t)
		x := stringContainmentWord(t, theory, "regex-leaf-word")
		regex := stringRegexLiteral(t, m, "a")
		var err error
		sort, err := m.GetSort(regex)
		astTestMust(t, err)
		switch operation {
		case "range":
			// The legacy axiom swaps endpoints; the NFA does not. Keep
			// these distinct native algorithms rather than repairing one.
			regex, err = m.MkRegexRange(astTestString(t, m, "c"), astTestString(t, m, "a"))
		case "full":
			regex, err = m.MkRegexFull(sort)
		case "char":
			regex, err = m.MkRegexFullChar(sort)
		}
		astTestMust(t, err)
		member := stringRegexRegister(t, theory, x, regex)
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, word := range []string{"", "a", "b", "c", "d", "ab", "\x00", "\xff"} {
			want := word == "a"
			switch operation {
			case "range":
				want = len(word) == 1 && word[0] >= 'a' && word[0] <= 'c'
			case "full":
				want = true
			case "char":
				want = len(word) == 1
			}
			for _, assigned := range []bool{false, true} {
				bit, err := m.MkBoolVal(assigned)
				astTestMust(t, err)
				if got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, word), member: bit}); got != (assigned == want) {
					t.Fatal("legacy leaf axiom semantics", operation, word, assigned)
				}
			}
		}
		astTestMust(t, rw.Close())
	}
}

func TestSMTStringLegacyRegexNFAEqualityGuardsAndCache(t *testing.T) {
	m, theory, state, config := stringLegacyRegexFixture(t)
	x := stringContainmentWord(t, theory, "regex-nfa-word")
	a := stringRegexLiteral(t, m, "a")
	star, err := m.MkRegexStar(a)
	astTestMust(t, err)
	member := stringRegexRegister(t, theory, x, star)
	var cached *smtStringNFA
	for _, text := range []string{"", "a", "aaa", "b", "\x00", "\xff"} {
		constant := astTestString(t, m, text)
		astTestMust(t, state.Internalize(constant, false))
		astTestMust(t, theory.checkRegexNewEquality(x, constant))
		if len(theory.regexNFACache) != 1 || theory.regexNFACache[star] == nil {
			t.Fatal("NFA was not cached by regex AST")
		}
		if cached == nil {
			cached = theory.regexNFACache[star]
		} else if theory.regexNFACache[star] != cached {
			t.Fatal("NFA cache reconstructed the same regex on a cache hit")
		}
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, same := range []bool{false, true} {
			for _, assigned := range []bool{false, true} {
				word := "different"
				if same {
					word = text
				}
				bit, err := m.MkBoolVal(assigned)
				astTestMust(t, err)
				matches := strings.Trim(text, "a") == ""
				if got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, word), member: bit}); got != (!same || assigned == matches) {
					t.Fatal("NFA conclusion lost its pending merge guard", text, same, assigned)
				}
			}
		}
		astTestMust(t, rw.Close())
	}
	// Use a fresh context so the earlier helper-generated lemmas cannot
	// make an unwired new_eq_eh callback appear to work.
	m, theory, state, _ = stringLegacyRegexFixture(t)
	x = stringContainmentWord(t, theory, "regex-core-word")
	star, err = m.MkRegexStar(stringRegexLiteral(t, m, "a"))
	astTestMust(t, err)
	member = stringRegexRegister(t, theory, x, star)
	if len(theory.regexNFACache) != 0 {
		t.Fatal("registration eagerly ran the legacy equality checker")
	}
	literal, err := state.Literal(member)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	for _, text := range []string{"b", "a"} {
		astTestMust(t, state.PushScope())
		equality, err := m.MkEq(x, astTestString(t, m, text))
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if state.Inconsistent() != (text == "b") {
			t.Fatal("legacy NFA core conflict/compatible branch", text)
		}
		if len(theory.regexNFACache) != 1 {
			t.Fatal("core merge did not reach the native NFA consumer")
		}
		astTestMust(t, state.PopScopes(1))
		if state.Inconsistent() {
			t.Fatal("regex merge conflict survived pop")
		}
	}
	if assigned, err := state.Assignment(literal); err != nil || assigned != lbool.True {
		t.Fatal("parent membership assignment was lost", assigned, err)
	}
}

func TestSMTStringRegexAutomataModeAndBorrowedParameterUpdate(t *testing.T) {
	m, theory, state, config := stringLegacyRegexFixture(t)
	key := "str.regex_automata"
	astTestMust(t, config.Context.SetBoolName(config.Params, &key, true))
	astTestMust(t, theory.parameterOwner.Update(config.Params))
	x := stringContainmentWord(t, theory, "regex-parameter-mode")
	star, err := m.MkRegexStar(stringRegexLiteral(t, m, "a"))
	astTestMust(t, err)
	stringRegexRegister(t, theory, x, star)
	constant := astTestString(t, m, "b")
	astTestMust(t, state.Internalize(constant, false))
	_, err = theory.checkNewEqualityValuesAndLengths(x, constant)
	astTestMust(t, err)
	if len(theory.regexNFACache) != 0 || theory.freshID != 0 || len(theory.regexTerms) != 1 {
		t.Fatal("automata registration accidentally used legacy axioms or NFA checking")
	}
	astTestMust(t, config.Context.SetBoolName(config.Params, &key, false))
	astTestMust(t, theory.parameterOwner.Update(config.Params))
	_, err = theory.checkNewEqualityValuesAndLengths(x, constant)
	astTestMust(t, err)
	if len(theory.regexNFACache) != 1 {
		t.Fatal("equality checker ignored the borrowed regex parameter update")
	}
}

func TestSMTStringLegacyRegexConcatUnionWitnessesAndWorklists(t *testing.T) {
	for _, union := range []bool{false, true} {
		m, theory, state, config := stringLegacyRegexFixture(t)
		x := stringContainmentWord(t, theory, "regex-partition-word")
		a, b := stringRegexLiteral(t, m, "a"), stringRegexLiteral(t, m, "b")
		var regex ast.ASTHandle
		var err error
		if union {
			regex, err = m.MkRegexUnion(a, b)
		} else {
			regex, err = m.MkRegexConcat(a, b)
		}
		astTestMust(t, err)
		member := stringRegexRegister(t, theory, x, regex)
		axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
		astTestMust(t, err)
		sort, err := m.GetSort(x)
		astTestMust(t, err)
		first, err := m.MkSequenceSkolem(astTestSymbol(t, m, "regex!tmp0"), nil, sort)
		astTestMust(t, err)
		second, err := m.MkSequenceSkolem(astTestSymbol(t, m, "regex!tmp1"), nil, sort)
		astTestMust(t, err)
		for _, witness := range []ast.ASTHandle{first, second} {
			_, regular := theory.variableSet[witness]
			_, regexVariable := theory.regexVariables[witness]
			_, internal := theory.internalVariables[witness]
			_, scoped := theory.variableScopes[0][witness]
			if !regular || !regexVariable || internal || !scoped {
				t.Fatal("native regex witness classification or scope differs")
			}
		}
		leftMember, err := m.MkSequenceInRegex(first, a)
		astTestMust(t, err)
		rightMember, err := m.MkSequenceInRegex(second, b)
		astTestMust(t, err)
		rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
		astTestMust(t, err)
		for _, text := range []string{"", "a", "b", "ab"} {
			for _, assigned := range []bool{false, true} {
				for _, children := range []bool{false, true} {
					bit, err := m.MkBoolVal(assigned)
					astTestMust(t, err)
					childBit, err := m.MkBoolVal(children)
					astTestMust(t, err)
					wordEquality := text == "ab"
					if union {
						wordEquality = text == "a" || text == "b"
					}
					// Source asserts both child memberships unconditionally,
					// alongside the parent's equivalence. Keep that formula.
					want := children && assigned == wordEquality
					values := map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, text), first: astTestString(t, m, "a"), second: astTestString(t, m, "b"), member: bit, leftMember: childBit, rightMember: childBit}
					if got := stringAxiomTruth(t, m, rw, axiom, values); got != want {
						t.Fatal("native partition/child-membership formula changed", union, text, assigned, children)
					}
				}
			}
		}
		astTestMust(t, rw.Close())
		astTestMust(t, theory.Propagate())
		if theory.CanPropagate() || len(theory.regexMemberships[first]) != 1 || len(theory.regexMemberships[second]) != 1 || len(theory.regexTerms) != 0 {
			t.Fatal("recursive membership setup did not drain its source worklists")
		}
		if _, err := state.Literal(leftMember); err != nil {
			t.Fatal("child membership was not internalized", err)
		}
	}
}

func TestSMTStringLegacyRegexStarUnrollDefinition(t *testing.T) {
	m, theory, state, _ := stringLegacyRegexFixture(t)
	x := stringContainmentWord(t, theory, "regex-star-definition")
	child := stringRegexLiteral(t, m, "a")
	star, err := m.MkRegexStar(child)
	astTestMust(t, err)
	accesses := theory.cacheAccesses
	member := stringRegexRegister(t, theory, x, star)
	if theory.cacheAccesses != accesses+2 {
		t.Fatal("star setup lost the separate inner/outer empty-literal cache accesses")
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	bound, err := m.MkSequenceSkolem(astTestSymbol(t, m, "unroll!tmp0"), nil, sort)
	astTestMust(t, err)
	unroll, err := m.MkFamilyApp(theory.FamilyID(), 43, nil, []ast.ASTHandle{child, bound})
	astTestMust(t, err)
	if state.context.enodes.get(bound) == nil || theory.IsAttachedToVariable(state.context.enodes.get(bound)) || len(theory.regexVariables) != 0 {
		t.Fatal("unroll counter was incorrectly classified as a String witness")
	}
	// A zero bound forces the empty unroll, and membership then forces
	// the input empty. This checks the emitted definition through the core.
	boundZero, err := m.MkEq(bound, zero)
	astTestMust(t, err)
	for _, expression := range []ast.ASTHandle{boundZero, member} {
		astTestMust(t, state.Internalize(expression, true))
		literal, err := state.Literal(expression)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	}
	_, err = state.PropagateCore()
	astTestMust(t, err)
	empty := astTestString(t, m, "")
	if state.context.enodes.get(unroll).Root() != state.context.enodes.get(empty).Root() || state.context.enodes.get(x).Root() != state.context.enodes.get(empty).Root() {
		t.Fatal("zero-count unroll definition was not connected to membership")
	}
	if status, err := theory.FinalCheck(); err != nil || status != SMTFinalCheckDone {
		t.Fatal("zero-count grounded unroll did not reach native assigned completion", status, err)
	}
}
