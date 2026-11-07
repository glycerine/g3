package smtcore

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func TestSMTStringContainmentProspectiveConstantGuards(t *testing.T) {
	for _, haystackVariable := range []bool{false, true} {
		for _, constantText := range []string{"", "a", "ab", "\x00\xff"} {
			m, config := smtModelGeneratorFixture(t)
			theory, state := stringTheoryFixture(t, m)
			sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "contain-pending-constant"), sort)
			astTestMust(t, err)
			other := astTestString(t, m, "a")
			left, right := other, x
			if haystackVariable {
				left, right = x, other
			}
			relation, err := m.MkSequenceContains(left, right)
			astTestMust(t, err)
			astTestMust(t, state.Internalize(relation, true))
			astTestMust(t, theory.instantiateContains(state.context.enodes.get(relation)))
			constant := astTestString(t, m, constantText)
			astTestMust(t, state.Internalize(constant, false))
			before := len(theory.refs)
			consistent, err := theory.checkNewEqualityValuesAndLengths(x, constant)
			astTestMust(t, err)
			if !consistent || len(theory.refs) <= before {
				t.Fatal("pre-merge constant containment did not publish its guarded consequence")
			}
			axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
			astTestMust(t, err)
			rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
			astTestMust(t, err)
			for _, value := range []string{"", "a", "ab", "b", "\x00\xff"} {
				for _, assigned := range []bool{false, true} {
					boolean, err := m.MkBoolVal(assigned)
					astTestMust(t, err)
					actual := strings.Contains("a", constantText)
					if haystackVariable {
						actual = strings.Contains(constantText, "a")
					}
					want := value != constantText || assigned == actual
					if got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, value), relation: boolean}); got != want {
						t.Fatal("prospective equality premise lost", haystackVariable, constantText, value, assigned)
					}
				}
			}
			astTestMust(t, rw.Close())
		}
	}
}

func TestSMTStringContainmentMandatoryConcatLiteral(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	theory, state := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "contain-haystack"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "contain-tail"), sort)
	astTestMust(t, err)
	needle := stringEqualityConcat(t, m, astTestString(t, m, "b"), y)
	relation, err := m.MkSequenceContains(x, needle)
	astTestMust(t, err)
	astTestMust(t, state.Internalize(relation, true))
	astTestMust(t, theory.instantiateContains(state.context.enodes.get(relation)))
	constant := astTestString(t, m, "aaa")
	astTestMust(t, state.Internalize(constant, false))
	before := len(theory.refs)
	astTestMust(t, theory.checkContainNewEquality(x, constant))
	if len(theory.refs) != before+1 {
		t.Fatal("mandatory missing literal did not refute containment")
	}
	axiom, err := m.RefGet(theory.refs[len(theory.refs)-1])
	astTestMust(t, err)
	rw, err := rewriter.NewTheoryRewriter(m, config.Context, config.Globals, config.Params)
	astTestMust(t, err)
	defer func() { astTestMust(t, rw.Close()) }()
	for _, value := range []string{"aaa", "b", ""} {
		for _, assigned := range []bool{false, true} {
			boolean, err := m.MkBoolVal(assigned)
			astTestMust(t, err)
			got := stringAxiomTruth(t, m, rw, axiom, map[ast.ASTHandle]ast.ASTHandle{x: astTestString(t, m, value), relation: boolean})
			if got != (value != "aaa" || !assigned) {
				t.Fatal("mandatory-literal refusal escaped its equality guard")
			}
		}
	}
	before = len(theory.refs)
	compatible := astTestString(t, m, "abb")
	astTestMust(t, state.Internalize(compatible, false))
	astTestMust(t, theory.checkContainNewEquality(x, compatible))
	if len(theory.refs) != before {
		t.Fatal("presence of a mandatory literal was mistaken for a complete containment decision")
	}
}

func TestSMTStringContainmentConstantMergeScopeRollback(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	sort, err := m.MkFamilySort(theory.FamilyID(), 2, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "contain-scoped-word"), sort)
	astTestMust(t, err)
	relation, err := m.MkSequenceContains(x, astTestString(t, m, "a"))
	astTestMust(t, err)
	astTestMust(t, state.Internalize(relation, true))
	astTestMust(t, theory.instantiateContains(state.context.enodes.get(relation)))
	literal, err := state.Literal(relation)
	astTestMust(t, err)
	literal.Negate()
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	for _, text := range []string{"a", "b"} {
		astTestMust(t, state.PushScope())
		equality, err := m.MkEq(x, astTestString(t, m, text))
		astTestMust(t, err)
		astTestMust(t, state.Internalize(equality, true))
		equalLiteral, err := state.Literal(equality)
		astTestMust(t, err)
		astTestMust(t, state.AssignLiteral(equalLiteral, NewSMTBooleanAxiom()))
		_, err = state.PropagateCore()
		astTestMust(t, err)
		if state.Inconsistent() != (text == "a") {
			t.Fatal("constant containment conflict or subsequent compatible branch is wrong", text)
		}
		astTestMust(t, state.PopScopes(1))
		if state.Inconsistent() {
			t.Fatal("containment conflict survived its scope")
		}
	}
}
