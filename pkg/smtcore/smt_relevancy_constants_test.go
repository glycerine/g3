package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTRelevancyAssertedOrWithTrue(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	other := smtRelevancyBool(t, m, "relevance-or-other")
	root, err := m.MkOr(truth, other)
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
	if !p.IsRelevant(truth) || p.IsRelevant(other) {
		t.Fatal("asserted OR must select true as its relevant witness")
	}
	astTestMust(t, p.Check([]ast.ASTHandle{root}))
	astTestMust(t, s.PopScopes(1))
	if p.IsRelevant(root) || p.IsRelevant(truth) {
		t.Fatal("popping the assertion must remove its relevance marks")
	}
}

func TestSMTRelevancyBooleanITEWithConstant(t *testing.T) {
	for _, test := range []struct {
		name    string
		truth   bool
		negated bool
	}{
		{name: "true", truth: true},
		{name: "false"},
		{name: "not true", truth: true, negated: true},
		{name: "not false", negated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := astTestManager(t)
			_, p, s := smtInternalizerFixture(t, m, nil)
			constant := m.MkFalse
			if test.truth {
				constant = m.MkTrue
			}
			condition, err := constant()
			astTestMust(t, err)
			if test.negated {
				condition, err = m.MkNot(condition)
				astTestMust(t, err)
			}
			thenBranch := smtRelevancyBool(t, m, "relevance-ite-then")
			elseBranch := smtRelevancyBool(t, m, "relevance-ite-else")
			root, err := m.MkIte(condition, thenBranch, elseBranch)
			astTestMust(t, err)
			astTestMust(t, s.Assert(root, ast.ASTHandle{}, 0))
			want, other := elseBranch, thenBranch
			if test.truth != test.negated {
				want, other = thenBranch, elseBranch
			}
			if !p.IsRelevant(condition) || !p.IsRelevant(want) || p.IsRelevant(other) {
				t.Fatal("asserted ITE must mark its condition and the branch selected by the constant")
			}
			astTestMust(t, p.Check([]ast.ASTHandle{root}))
		})
	}
}

func TestSMTRelevancyTermITEWithTrue(t *testing.T) {
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	sort, err := m.MkSort(astTestSymbol(t, m, "RelevanceITESort"), ast.ASTSortInfo{})
	astTestMust(t, err)
	thenTerm, _ := smtEnodeConstant(t, m, 2240, sort, 0)
	elseTerm, _ := smtEnodeConstant(t, m, 2241, sort, 0)
	root, err := m.MkIte(truth, thenTerm, elseTerm)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(root, false))
	thenEquality, err := s.mkEqualityAtom(root, thenTerm)
	astTestMust(t, err)
	elseEquality, err := s.mkEqualityAtom(root, elseTerm)
	astTestMust(t, err)
	astTestMust(t, s.markRelevant(root))
	if !p.IsRelevant(thenEquality) || p.IsRelevant(elseEquality) {
		t.Fatal("term ITE must mark the defining equality for the branch selected by true")
	}
}
