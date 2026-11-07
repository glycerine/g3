package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func stringUnrollTerm(t *testing.T, theory *SMTStringTheory, regex, bound ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	m := theory.state.manager
	term, err := m.MkFamilyApp(theory.FamilyID(), 43, nil, []ast.ASTHandle{regex, bound})
	astTestMust(t, err)
	astTestPin(t, m, term)
	astTestMust(t, theory.state.Internalize(term, false))
	return term
}

func TestSMTStringUnrollConstantQuotientGuards(t *testing.T) {
	for _, pattern := range []string{"ab", "x", "\x00\xff"} {
		for _, text := range []string{"a", "ab", "aba", "abab", "\x00\xff\x00"} {
			m, config := smtModelGeneratorFixture(t)
			theory, state := stringTheoryFixture(t, m)
			bound, err := theory.freshUnrollBound()
			astTestMust(t, err)
			unroll := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, pattern), bound)
			constant := astTestString(t, m, text)
			astTestMust(t, state.Internalize(constant, false))
			before := len(theory.refs)
			astTestMust(t, theory.processUnrollConstant(unroll, constant))
			if len(theory.refs) != before+1 {
				t.Fatal("literal-core unroll did not produce its native count/length implication")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			guard, err := m.MkEq(unroll, constant)
			astTestMust(t, err)
			astTestPin(t, m, guard)
			length, err := m.MkSequenceLength(unroll)
			astTestMust(t, err)
			astTestPin(t, m, length)
			rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
			astTestMust(t, err)
			quotient := len(text) / len(pattern)
			for _, parent := range []bool{false, true} {
				for _, count := range []int{quotient, quotient + 1} {
					for _, bytes := range []int{len(text), len(text) + 1} {
						bit, err := m.MkBoolVal(parent)
						astTestMust(t, err)
						cn, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(count))
						astTestMust(t, err)
						ln, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(bytes))
						astTestMust(t, err)
						values := map[ast.ASTHandle]ast.ASTHandle{guard: bit, bound: cn, length: ln}
						want := !parent || count == quotient && bytes == len(text)
						if got := stringAxiomTruth(t, m, rw, axiom, values); got != want {
							t.Fatal("native floor quotient/byte length or pending guard changed", pattern, text, parent, count, bytes)
						}
					}
				}
			}
			astTestMust(t, rw.Close())
		}
	}
}

func TestSMTStringUnrollClassCollectionCycleAndRollback(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	x := stringContainmentWord(t, theory, "unroll-class-x")
	bound, err := theory.freshUnrollBound()
	astTestMust(t, err)
	u1 := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "a"), bound)
	u2 := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, "b"), bound)
	a, b := astTestString(t, m, "a"), astTestString(t, m, "b")
	for _, constant := range []ast.ASTHandle{a, b} {
		astTestMust(t, state.Internalize(constant, false))
	}
	astTestMust(t, state.PushScope())
	for _, member := range []ast.ASTHandle{u2, u1, a, b} {
		theory.find.Merge(uint32(state.context.enodes.get(x).TheoryVar(theory.FamilyID())), uint32(state.context.enodes.get(member).TheoryVar(theory.FamilyID())))
	}
	for _, test := range [][2]ast.ASTHandle{{a, b}, {b, a}} {
		constant, unrolls, err := theory.stringClassUnrolls(test[0])
		astTestMust(t, err)
		if constant != test[1] || len(unrolls) != 2 || unrolls[0] != u1 || unrolls[1] != u2 {
			t.Fatal("unroll collection lost cycle-relative last constant or stable set order")
		}
	}
	astTestMust(t, state.PopScopes(1))
	constant, unrolls, err := theory.stringClassUnrolls(x)
	astTestMust(t, err)
	if !constant.IsNull() || len(unrolls) != 0 {
		t.Fatal("unroll class collection read a popped private class")
	}
}

func TestSMTStringUnrollConstantEmptyAndUnsupportedCases(t *testing.T) {
	m := astTestManager(t)
	theory, _ := stringTheoryFixture(t, m)
	bound, err := theory.freshUnrollBound()
	astTestMust(t, err)
	literal := stringRegexLiteral(t, m, "a")
	star, err := m.MkRegexStar(literal)
	astTestMust(t, err)
	complex := stringUnrollTerm(t, theory, star, bound)
	simple := stringUnrollTerm(t, theory, literal, bound)
	emptyCore := stringUnrollTerm(t, theory, stringRegexLiteral(t, m, ""), bound)
	empty := astTestString(t, m, "")
	before := len(theory.refs)
	for _, pair := range [][2]ast.ASTHandle{{simple, empty}, {emptyCore, empty}, {complex, astTestString(t, m, "aaa")}, {empty, empty}} {
		astTestMust(t, theory.processUnrollConstant(pair[0], pair[1]))
	}
	if len(theory.refs) != before {
		t.Fatal("empty/non-simple unroll unexpectedly added a count consequence")
	}
	if err := theory.processUnrollConstant(emptyCore, astTestString(t, m, "a")); err != ast.ErrASTDomain {
		t.Fatal("source nonempty literal-core divisor precondition was ignored", err)
	}
}

func TestSMTStringUnrollConstantLegacyCoreCallbackAndPop(t *testing.T) {
	m, theory, state, _ := stringLegacyRegexFixture(t)
	x := stringContainmentWord(t, theory, "unroll-callback-word")
	star, err := m.MkRegexStar(stringRegexLiteral(t, m, "ab"))
	astTestMust(t, err)
	member := stringRegexRegister(t, theory, x, star)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	bound, err := m.MkSequenceSkolem(astTestSymbol(t, m, "unroll!tmp0"), nil, sort)
	astTestMust(t, err)
	literal, err := state.Literal(member)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	for _, test := range []struct{ word, count string }{{"abab", "2"}, {"ab", "1"}} {
		astTestMust(t, state.PushScope())
		equality, err := m.MkEq(x, astTestString(t, m, test.word))
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		literal, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		text, _, numeral, err := m.ArithmeticNumeralText(state.context.enodes.get(bound).Root().Owner())
		astTestMust(t, err)
		if state.Inconsistent() || !numeral || text != test.count {
			t.Fatal("legacy star membership did not reach the unroll count callback", text, numeral)
		}
		astTestMust(t, state.PopScopes(1))
		if _, _, numeral, err := m.ArithmeticNumeralText(state.context.enodes.get(bound).Root().Owner()); err != nil || numeral {
			t.Fatal("unroll count survived its scope", numeral, err)
		}
	}
}
