package pdr

// Source: pdr_context.cpp context::propagate simplify_formulas pre/post hooks.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestContextPropagationRunsConfiguredFrameSimplificationHooks(t *testing.T) {
	for _, test := range []struct {
		name          string
		before, after bool
	}{
		{name: "before", before: true},
		{name: "after", after: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager, owner, transformer, _, closeBase := closureFixture(t)
			truth, err := manager.MkTrue()
			closureMust(t, err)
			closureMust(t, transformer.InitializeSystem(truth, truth))
			strong, err := manager.MkFreshConst("propagation-strong", mustBoolSort(t, manager))
			closureMust(t, err)
			extra, err := manager.MkFreshConst("propagation-extra", mustBoolSort(t, manager))
			closureMust(t, err)
			weak, err := manager.MkOr(strong, extra)
			closureMust(t, err)
			_, err = transformer.AddProperty(strong, 0)
			closureMust(t, err)
			_, err = transformer.AddProperty(weak, 0)
			closureMust(t, err)
			head, err := transformer.Head()
			closureMust(t, err)
			context := &Context{
				manager: manager, owner: owner, transformers: map[ast.ASTHandle]*PredicateTransformer{head: transformer}, order: []ast.ASTHandle{head},
				simplifyPre: test.before, simplifyPost: test.after,
			}
			fixed, err := context.propagate(0)
			closureMust(t, err)
			if fixed {
				t.Fatal("non-invariant property unexpectedly closed frame")
			}
			stats, err := transformer.Statistics()
			closureMust(t, err)
			if stats.Properties != 1 {
				t.Fatal("configured propagation hook did not simplify frame", stats)
			}
			closureMust(t, transformer.Close())
			closureMust(t, owner.Close())
			closeBase()
		})
	}
}

func mustBoolSort(t *testing.T, manager *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	sort, err := manager.MkBoolSort()
	closureMust(t, err)
	return sort
}
