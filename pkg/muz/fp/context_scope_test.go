package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestFixedpointPopResumesMetadataAllocationFailure(t *testing.T) {
	for _, closeInstead := range []bool{false, true} {
		name := "pop"
		if closeInstead {
			name = "close"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			m, _ := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngine(m)
			must(err)
			var padding []ast.ASTRef
			defer func() {
				m.BudgetField.FailAfter(-1)
				for i := range padding {
					must(m.DelRef(&padding[i]))
				}
				must(e.Close())
			}()
			must(e.Push())
			boolean, err := m.MkBoolSort()
			must(err)
			head, err := m.MkFreshConst("scope-saved-output", boolean)
			must(err)
			declaration, err := m.GetDecl(head)
			must(err)
			must(e.RegisterPredicate(declaration))
			saved, err := e.rules.NewRule(head, nil, nil, "saved")
			must(err)
			defer func() { must(saved.Close()) }()
			must(e.set.Add(saved))
			must(e.set.SetOutputPredicate(declaration))
			must(e.Push())
			must(e.set.Reset())
			current, err := e.rules.NewRule(head, nil, nil, "current")
			must(err)
			defer func() { must(current.Close()) }()
			must(e.set.Add(current))
			before := e.revision
			// Keep the current rule shared, so resetting it frees no ASTRef slot.
			// Exhaust the current reference page through the real allocator; the
			// saved output's NewRef is then the first allocation during restore.
			m.BudgetField.FailAfter(0)
			for i := 0; ; i++ {
				ref, cause := m.NewRef(ast.ASTHandle{})
				if errors.Is(cause, arena.ErrInjectedFailure) {
					break
				}
				must(cause)
				padding = append(padding, ref)
				if i >= 256 {
					t.Fatal("reference page exhaustion not reached")
				}
			}
			if err = e.Pop(); !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("missing real metadata allocation refusal", err)
			}
			if e.popPhase != contextPopRules || len(e.scopes) != 2 || len(e.rawRuleScopes) != 2 || len(e.assertionScopes) != 2 || e.revision != before+1 || e.queryReady {
				t.Fatal("failed restore lost its scope or published a query", e.popPhase, len(e.scopes), e.revision, before)
			}
			if _, _, err = e.NamedRules(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("partial rules escaped", err)
			}
			if _, err = e.BackgroundAssertions(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("partial assertions escaped", err)
			}
			if err = e.Push(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("pending pop accepted a new scope", err)
			}
			if _, err = e.QueryPredicate(declaration); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("pending pop accepted a query", err)
			}
			if closeInstead {
				if err = e.Close(); !errors.Is(err, arena.ErrInjectedFailure) || e.closed || e.closing || len(e.scopes) != 2 {
					t.Fatal("refused close lost the pending restoration", err)
				}
				m.BudgetField.FailAfter(-1)
				must(e.Close())
				must(e.Close())
				return
			}
			m.BudgetField.FailAfter(-1)
			must(e.Pop())
			if e.popPhase != contextPopNone || len(e.scopes) != 1 || e.revision != before+1 {
				t.Fatal("retry consumed another scope or revision")
			}
			rule, err := e.set.Rule(0)
			must(err)
			restoredName, err := rule.Name()
			must(err)
			must(rule.Close())
			output, err := e.set.IsOutputPredicate(declaration)
			must(err)
			if restoredName != "saved" || !output {
				t.Fatal("retry lost saved rule metadata", restoredName, output)
			}
			must(e.Pop())
			count, err := e.set.Len()
			must(err)
			if count != 0 || len(e.scopes) != 0 {
				t.Fatal("outer scope did not restore the empty program")
			}
		})
	}
}
