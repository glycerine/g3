package smtcore

import (
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTStringUnrollAssignmentWindowsWordsAndLengths(t *testing.T) {
	m, theory, state, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "unroll-options")
	tester, err := theory.freshStringUnrollTester()
	astTestMust(t, err)
	for _, core := range []string{"ab", "\x00\xff"} {
		for _, lower := range []int32{0, 2} {
			upper := lower + 2
			root, err := theory.generateStringUnrollAssignment(word, core, tester, lower, upper)
			astTestMust(t, err)
			formula, err := m.RefGet(root)
			astTestMust(t, err)
			length, err := m.MkSequenceLength(word)
			astTestMust(t, err)
			for _, label := range []string{strconv.Itoa(int(lower)), strconv.Itoa(int(lower + 1)), "more", "junk"} {
				choice, err := state.mkEqualityAtom(tester, astTestString(t, m, label))
				astTestMust(t, err)
				if _, exists := state.BoolVar(choice); exists {
					t.Fatal("ordinary unroll generation internalized selector")
				}
				for count := 0; count <= 5; count++ {
					for size := 0; size <= 10; size++ {
						text := strings.Repeat(core, count)
						number, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(size))
						astTestMust(t, err)
						want := false
						if label == "more" {
							want = size >= int(upper)*len(core) && count != int(lower) && count != int(lower+1)
						} else {
							index, err := strconv.Atoi(label)
							want = err == nil && index >= int(lower) && index < int(upper) && index == count && size == index*len(core)
						}
						if got := stringAxiomTruth(t, m, rw, formula, map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), length: number, tester: astTestString(t, m, label)}); got != want {
							t.Fatal("unroll choice/word/length truth", core, lower, label, count, size, got, want)
						}
					}
				}
			}
			astTestMust(t, m.DelRef(&root))
			if _, err := m.GetKind(formula); err != nil {
				t.Fatal("unroll assignment lost explicit native lifetime root", err)
			}
		}
	}
}

func TestSMTStringUnrollTesterOwnershipAndAggressivePhases(t *testing.T) {
	m, theory, state, _ := stringRegexStageFixture(t)
	word := stringGroundingWord(t, theory, "unroll-phases")
	astTestMust(t, state.PushScope())
	tester, err := theory.freshStringUnrollTester()
	astTestMust(t, err)
	decl, err := m.GetDecl(tester)
	astTestMust(t, err)
	symbol, err := m.GetASTName(decl)
	astTestMust(t, err)
	name, _, err := m.Symbols.BareString(symbol)
	astTestMust(t, err)
	if name != "unrollTest!tmp0" {
		t.Fatal("unroll tester name", name)
	}
	roots := 0
	for _, ref := range theory.refs {
		term, err := m.RefGet(ref)
		astTestMust(t, err)
		if term == tester {
			roots++
		}
	}
	_, role := theory.unrollTestVariables[tester]
	_, scoped := theory.variableScopes[theory.level][tester]
	if roots != 1 || !role || !scoped || theory.testers.lengthID != 0 || theory.testers.valueID != 0 {
		t.Fatal("unroll creator gained a length/value root/counter or lost role/scope")
	}
	context := theory.parameterOwner.Context
	params, err := context.NewValue()
	astTestMust(t, err)
	key := "str.aggressive_unroll_testing"
	astTestMust(t, context.SetBoolName(&params, &key, true))
	astTestMust(t, theory.parameterOwner.Update(&params))
	astTestMust(t, context.Del(&params))
	root, err := theory.generateStringUnrollAssignment(word, "ab", tester, 0, 2)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	for _, label := range []string{"0", "1", "more"} {
		choice, err := state.mkEqualityAtom(tester, astTestString(t, m, label))
		astTestMust(t, err)
		variable, exists := state.BoolVar(choice)
		if !exists {
			t.Fatal("aggressive unroll choice not internalized")
		}
		data, err := state.BooleanData(variable)
		astTestMust(t, err)
		if !state.isRelevant(choice) || !data.PhaseAvailable || data.Phase != (label != "more") || data.TryTrueFirst() {
			t.Fatal("unroll phase hint changed", label)
		}
		assigned, err := state.Assignment(NewSMTLiteral(variable, false))
		astTestMust(t, err)
		if assigned != lbool.Undef {
			t.Fatal("unroll phase hint assigned a value")
		}
	}
	astTestMust(t, state.PopScopes(1))
	if _, ok := theory.unrollTestVariables[tester]; !ok {
		t.Fatal("scope pop erased lifetime unroll tester role")
	}
}

func TestSMTStringConditionalUnrollReplayMoreAndConcreteProgression(t *testing.T) {
	m, theory, _, rw := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "conditional-unroll")
	count1, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	count2, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	regex := stringRegexLiteral(t, m, "ab")
	u1, u2 := stringUnrollTerm(t, theory, regex, count1), stringUnrollTerm(t, theory, regex, count2)
	root, err := theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u2, u1, u2}, "ab")
	astTestMust(t, err)
	formula, err := m.RefGet(root)
	astTestMust(t, err)
	history := theory.testers.unrollHistories[word][0]
	if len(history.unrolls) != 2 || len(history.testers) != 1 {
		t.Fatal("unroll set key/history lost identity")
	}
	first := history.testers[0]
	fresh := theory.freshID
	root2, err := theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u1, u2}, "ab")
	astTestMust(t, err)
	replay, err := m.RefGet(root2)
	astTestMust(t, err)
	if formula != replay || theory.freshID != fresh {
		t.Fatal("unassigned tester replay changed history or formula")
	}
	length, err := m.MkSequenceLength(word)
	astTestMust(t, err)
	for _, guard := range []bool{false, true} {
		text := "ab"
		other := "other"
		if guard {
			other = text
		}
		values := map[ast.ASTHandle]ast.ASTHandle{word: astTestString(t, m, text), u1: astTestString(t, m, text), u2: astTestString(t, m, other), first: astTestString(t, m, "junk"), length: count2}
		if got := stringAxiomTruth(t, m, rw, formula, values); got == guard {
			t.Fatal("unroll membership guard changed", guard, got)
		}
	}
	astTestMust(t, m.DelRef(&root2))
	astTestMust(t, m.DelRef(&root))
	stringTesterNewEquality(t, theory, first, astTestString(t, m, "more"))
	root, err = theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u1, u2}, "ab")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(history.testers) != 2 {
		t.Fatal("assigned more did not append repeat window")
	}
	second := history.testers[1]
	stringTesterNewEquality(t, theory, second, astTestString(t, m, "2"))
	root, err = theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u1, u2}, "ab")
	astTestMust(t, err)
	formula, err = m.RefGet(root)
	astTestMust(t, err)
	if len(history.testers) != 3 {
		t.Fatal("native assigned-concrete fallthrough was changed")
	}
	third := history.testers[2]
	eight, err := m.ArithmeticIntegerNumeralText("8")
	astTestMust(t, err)
	text := astTestString(t, m, "abababab")
	values := map[ast.ASTHandle]ast.ASTHandle{word: text, u1: text, u2: text, length: eight, first: astTestString(t, m, "more"), second: astTestString(t, m, "junk"), third: astTestString(t, m, "4")}
	if !stringAxiomTruth(t, m, rw, formula, values) {
		t.Fatal("concrete earlier tester incorrectly added a more premise")
	}
	values[first] = astTestString(t, m, "0")
	values[third] = astTestString(t, m, "junk")
	if !stringAxiomTruth(t, m, rw, formula, values) {
		t.Fatal("assigned-more guard missing")
	}
	astTestMust(t, m.DelRef(&root))
}

func TestSMTStringConditionalUnrollScopeRoleFilterAndFresh(t *testing.T) {
	m, theory, state, _ := stringLibraryFixture(t)
	word := stringGroundingWord(t, theory, "unroll-scope")
	count, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	u := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "x"), count)
	astTestMust(t, state.PushScope())
	root, err := theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u}, "x")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	history := theory.testers.unrollHistories[word][0]
	first := history.testers[0]
	astTestMust(t, state.PopScopes(1))
	fresh := theory.freshID
	root, err = theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u}, "x")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if theory.freshID != fresh || history.testers[0] != first || state.context.enodes.get(first) == nil {
		t.Fatal("scope pop replaced lifetime-role tester instead of refreshing")
	}
	delete(theory.unrollTestVariables, first)
	root, err = theory.generateStringConditionalUnrollOptions(word, []ast.ASTHandle{u}, "x")
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&root))
	if len(history.testers) != 1 || history.testers[0] == first {
		t.Fatal("unroll role filter failed to erase inactive tester")
	}
	owner, err := theory.Fresh()
	astTestMust(t, err)
	defer func() { astTestMust(t, owner.Close()) }()
	if len(owner.testers.unrollHistories) != 0 || len(owner.unrollTestVariables) != 0 {
		t.Fatal("Fresh copied unroll histories")
	}
}
