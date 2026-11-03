package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestReachFactConstructionFailureRetainsPartialDestructionFrontier(t *testing.T) {
	m, _ := newSpacerTestServices(t)
	rules, err := base.NewRuleManager(m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rules.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	head, err := m.MkFreshConst("reach-construction-frontier-head", boolean)
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := m.GetDecl(head)
	if err != nil {
		t.Fatal(err)
	}
	if err = rules.RegisterPredicate(declaration); err != nil {
		t.Fatal(err)
	}
	rule, err := rules.NewRule(head, nil, nil, "reach-construction-frontier")
	if err != nil {
		t.Fatal(err)
	}
	auxiliary, err := m.MkFreshConst("reach-construction-frontier-aux", boolean)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := NewReachFact(m, rule, head, []ast.ASTHandle{auxiliary}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = rule.Close(); err != nil {
		t.Fatal(err)
	}
	ownedRule := fact.state.rule
	// Model an independent exceptional rule-destructor refusal after the
	// reach fact has already released its justification and AST-root prefix.
	// The standard receipt must preserve this exact frontier for retry.
	fact.state.rule = &base.Rule{}
	failure := finishReachFactConstruction(ast.ErrASTDomain, fact)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || !errors.Is(receipt.OriginalCause(), ast.ErrASTDomain) || !errors.Is(receipt.CleanupCause(), base.ErrRuleOwner) {
		t.Fatal("partial reach-fact cleanup did not publish a standard receipt", failure)
	}
	if fact.closed || fact.state == nil || fact.state.fact != (ast.ASTRef{}) || len(fact.state.auxiliaries) != 0 || fact.state.rule == nil {
		t.Fatal("partial destruction frontier was not preserved")
	}
	if receipt.RuntimeOwner() != m.NumeralService().RuntimeOwner() {
		t.Fatal("partial destruction receipt lost native runtime")
	}
	reportedText := failure.Error()
	if !errors.Is(failure, base.ErrRuleOwner) {
		t.Fatal("partial destruction receipt omitted cleanup cause")
	}
	fact.state.rule = ownedRule
	if err = receipt.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if err = receipt.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if failure.Error() != reportedText || !errors.Is(failure, ast.ErrASTDomain) || !errors.Is(failure, base.ErrRuleOwner) || receipt.CleanupCause() != nil {
		t.Fatal("partial destruction receipt changed its report after retry", failure, receipt.CleanupCause())
	}
}
