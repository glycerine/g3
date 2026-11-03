package pdr

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// A cached leaf without a model must establish that an initial rule explains
// its state. A unique initial rule alone is not evidence for that state.
func TestTraceInitialSelectionValidatesModelLessState(t *testing.T) {
	manager, context, rules := classifierContextFixture(t)
	set, declaration := classifierRuleSet(t, manager, rules, "fact")
	closureMust(t, context.Initialize(set))
	transformer, found, err := context.Transformer(declaration)
	closureMust(t, err)
	if !found {
		t.Fatal("initialized predicate missing")
	}
	for _, possible := range []bool{false, true} {
		state, err := manager.MkFalse()
		if possible {
			state, err = manager.MkTrue()
		}
		closureMust(t, err)
		node, err := NewModelNode(nil, state, transformer, 0)
		closureMust(t, err)
		selected, cause := context.selectedTraceRule(node)
		if !possible {
			if selected != nil || !errors.Is(cause, ast.ErrASTDomain) || node.model != nil || node.selected != nil {
				t.Fatal("unmatched state fabricated an initial derivation", selected, cause)
			}
		} else {
			closureMust(t, cause)
			if selected == nil || node.model == nil || node.selected != selected || node.rule == nil {
				t.Fatal("initial selection did not own its solver model and source rule")
			}
			repeated, err := context.selectedTraceRule(node)
			closureMust(t, err)
			if repeated != selected {
				t.Fatal("initial selection changed after recovering its model")
			}
		}
		closureMust(t, node.release())
	}
}
