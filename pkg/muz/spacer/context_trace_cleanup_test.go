package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerTraceCleanupRetainsRemainingRuleFrontier(t *testing.T) {
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
	head, err := m.MkFreshConst("trace-cleanup-head", boolean)
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
	makeRule := func(name string) *base.Rule {
		rule, cause := rules.NewRule(head, nil, nil, name)
		if cause != nil {
			t.Fatal(cause)
		}
		return rule
	}
	consumed := makeRule("trace-cleanup-consumed")
	replacement := makeRule("trace-cleanup-retry")
	attempt := &spacerTraceCleanupAttempt{
		manager: m,
		// Reverse destruction consumes the valid rule before independently
		// refusing the invalid remaining owner.
		rules: []*base.Rule{{}, consumed},
	}
	failure := attempt.finish(ast.ErrASTDomain)
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) || !errors.Is(receipt.OriginalCause(), ast.ErrASTDomain) || !errors.Is(receipt.CleanupCause(), base.ErrRuleOwner) {
		t.Fatal("trace cleanup did not publish its remaining owner", failure)
	}
	if len(attempt.rules) != 1 || attempt.rules[0] == nil {
		t.Fatal("trace cleanup lost or replayed its refusal frontier")
	}
	if receipt.RuntimeOwner() != m.NumeralService().RuntimeOwner() {
		t.Fatal("trace cleanup receipt lost runtime identity")
	}
	reportedText := failure.Error()
	if !errors.Is(failure, base.ErrRuleOwner) {
		t.Fatal("trace cleanup receipt omitted cleanup cause")
	}
	attempt.rules[0] = replacement
	if err = receipt.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if err = receipt.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if failure.Error() != reportedText || !errors.Is(failure, ast.ErrASTDomain) || !errors.Is(failure, base.ErrRuleOwner) || receipt.CleanupCause() != nil {
		t.Fatal("trace cleanup receipt changed its report after retry", failure, receipt.CleanupCause())
	}
}
