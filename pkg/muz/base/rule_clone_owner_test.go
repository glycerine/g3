package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleCloneRejectsDifferentManagerAndClosedSource(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	source, err := f.rules.NewRule(f.head, nil, nil, "owner")
	ruleMust(t, err)
	otherContext, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, otherContext.Close()) })
	otherManager, err := otherContext.Manager()
	ruleMust(t, err)
	otherRules, err := base.NewRuleManager(otherManager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, otherRules.Close()) })
	if _, err = otherRules.CloneRule(source); !errors.Is(err, base.ErrRuleOwner) {
		t.Fatal("cross-manager clone", err)
	}
	ruleMust(t, source.Close())
	if _, err = f.rules.CloneRule(source); !errors.Is(err, base.ErrRuleOwner) {
		t.Fatal("closed source clone", err)
	}
}
