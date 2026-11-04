package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestUDocQuantifierPropertyOwnerSurvivesCleanupRefusal(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	before := contextDisplayLiveCounts(f.m.Stats())
	borrowed, err := f.e.set.Rule(0)
	fragmentMust(t, err)
	head, err := borrowed.Head()
	fragmentMust(t, err)
	fragmentMust(t, borrowed.Close())
	rule, err := f.e.rules.NewRule(head, nil, nil, "property-owner")
	fragmentMust(t, err)
	set, err := base.NewRuleSet(f.e.rules)
	fragmentMust(t, err)
	fragmentMust(t, set.Add(rule))
	fragmentMust(t, rule.Close())
	properties, err := base.NewRuleProperties(f.e.rules)
	fragmentMust(t, err)
	fragmentMust(t, properties.Collect(set))
	fragmentMust(t, set.Close()) // The collector is now the rule's sole owner.
	attempt := &contextFormulaAttempt{manager: f.m, properties: properties}
	original := errors.New("property check failed")
	var receipt memory.ConstructionFailure
	f.refuse(func() {
		cause := attempt.finish(original)
		if !errors.Is(cause, original) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &receipt) {
			t.Fatal("lost property owner or original error", cause)
		}
		for i := 0; i < 2; i++ {
			if err := receipt.Cleanup(); !errors.Is(err, ast.ErrASTPending) || attempt.properties != properties {
				t.Fatal("cleanup crossed active native owner", err)
			}
		}
	})
	fragmentMust(t, receipt.Cleanup())
	fragmentMust(t, receipt.Cleanup())
	if attempt.properties != nil || receipt.CleanupCause() != nil {
		t.Fatal("property owner not released")
	}
	if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
		t.Fatal("property cleanup leaked", before, after)
	}
}
