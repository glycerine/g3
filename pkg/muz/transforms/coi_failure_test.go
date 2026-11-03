package transforms_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestConeConstructionFailureReleasesModelPrefixAndRetries(t *testing.T) {
	failures, succeeded := 0, false
	for stop := int64(0); stop < 1024 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newConeFixture(t, "Goal", "Missing", "Pruned")
			f.edge(t, "Missing", "Goal")
			f.add(t, f.applications["Pruned"], "pruned-fact")
			// This head requires a cold, wide conjunction after the first
			// converter entry has already acquired its owning reference.
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			truth, err := f.manager.MkTrue()
			coneMust(t, err)
			domain := make([]ast.ASTHandle, 256)
			arguments := make([]ast.ASTHandle, len(domain))
			for i := range domain {
				domain[i], arguments[i] = boolean, truth
			}
			name, err := f.manager.Symbols.Intern("cone-cold-wide-pruned")
			coneMust(t, err)
			declaration, err := f.manager.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
			coneMust(t, err)
			coneMust(t, f.rules.RegisterPredicate(declaration))
			head, err := f.manager.MkApp(declaration, arguments)
			coneMust(t, err)
			rule, err := f.rules.NewRule(head, nil, nil, "wide-pruned-fact")
			coneMust(t, err)
			coneMust(t, f.set.Add(rule))
			coneMust(t, rule.Close())
			coneMust(t, f.set.SetOutputPredicate(f.declarations["Goal"]))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("unstratified fixture")
			}
			plugin := transforms.NewConePlugin(0)
			defer func() { coneMust(t, plugin.Close()) }()
			before := f.manager.Stats().LiveReferences
			f.manager.Budget().FailAfter(stop)
			result, failure := plugin.Transform(f.set)
			f.manager.Budget().FailAfter(-1)
			if failure != nil {
				if result != nil {
					t.Fatal("failed cone published a rule set")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					coneMust(t, cleanup.Cleanup())
				}
				if !errors.Is(failure, arena.ErrInjectedFailure) {
					t.Fatal("unexpected cone failure", failure)
				}
				steps, err := plugin.TakeModelSteps()
				coneMust(t, err)
				if len(steps) != 0 {
					coneMust(t, transforms.CloseConeModelSteps(steps))
					t.Fatal("failed cone published model definitions")
				}
				if after := f.manager.Stats().LiveReferences; after != before {
					t.Fatal("failed cone leaked its model or rule prefix", before, after)
				}
				failures++
				result, err = plugin.Transform(f.set)
				coneMust(t, err)
			} else {
				succeeded = true
			}
			if result == nil {
				t.Fatal("successful cone did not prune the unfounded rule")
			}
			if count, err := result.Len(); err != nil || count != 0 {
				t.Fatal("wrong retry result", count, err)
			}
			coneMust(t, result.Close())
			steps, err := plugin.TakeModelSteps()
			coneMust(t, err)
			if len(steps) != 2 {
				t.Fatal("retry lost ordered converter steps", len(steps))
			}
			coneMust(t, transforms.CloseConeModelSteps(steps))
			if after := f.manager.Stats().LiveReferences; after != before {
				t.Fatal("closed cone kept model or rule roots", before, after)
			}
		})
	}
	if failures == 0 || !succeeded {
		t.Fatal("cone allocation campaign did not reach failure and success", failures, succeeded)
	}
}
