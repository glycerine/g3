package pdr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRInitialRuleAllocationFailureAndRetry(t *testing.T) {
	failures, succeeded := 0, false
	stops := []int64{0, 1, 2, 3, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384}
	for _, stop := range stops {
		if succeeded {
			break
		}
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, owner, old, _, closeBase := closureFixture(t)
			defer func() { closureMust(t, old.Close()); closureMust(t, owner.Close()); closeBase() }()
			must := func(err error) { t.Helper(); closureMust(t, err) }
			context, err := NewContext(m, nil, 2, ReachableConstraintCache, owner.contexts.config)
			must(err)
			defer func() { must(context.Close()) }()
			declaration, err := old.Head()
			must(err)
			falsity, err := m.MkFalse()
			must(err)
			must(context.AddCover(2, declaration, falsity))
			prior := context.transformers[declaration]
			active := context.owner.contexts.active
			rules, err := base.NewRuleManager(m)
			must(err)
			defer func() { must(rules.Close()) }()
			must(rules.RegisterPredicate(declaration))
			set, err := base.NewRuleSet(rules)
			must(err)
			defer func() { must(set.Close()) }()
			sort, err := m.GetDomain(declaration, 0)
			must(err)
			variable, err := m.MkVar(0, sort)
			must(err)
			head, err := m.MkApp(declaration, []ast.ASTHandle{variable})
			must(err)
			rule, err := rules.NewRule(head, nil, nil, "prequery-failure-fact")
			must(err)
			must(set.Add(rule))
			must(rule.Close())
			m.Budget().FailAfter(stop)
			err = context.Initialize(set)
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					must(cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected initialization failure", err)
				}
				if context.initialized || context.transformers[declaration] != prior || prior.closed {
					t.Fatal("failed initialization replaced the original transformer")
				}
				if got := context.owner.contexts.active; got != active {
					t.Fatal("failed initialization left active staged contexts", got, active)
				}
				proofs, err := m.ProofsEnabled()
				must(err)
				if proofs {
					t.Fatal("failed initialization changed the caller's proof mode")
				}
				cover, err := context.CoverDelta(2, declaration)
				must(err)
				handle, err := m.RefGet(cover)
				must(err)
				if !m.IsFalse(handle) {
					t.Fatal("failed initialization changed the original cover")
				}
				must(m.DelRef(&cover))
				must(context.Initialize(set))
			} else {
				succeeded = true
			}
			proofs, err := m.ProofsEnabled()
			must(err)
			if proofs {
				t.Fatal("initialization changed the caller's proof mode")
			}
			cover, err := context.CoverDelta(2, declaration)
			must(err)
			defer func() { must(m.DelRef(&cover)) }()
			handle, err := m.RefGet(cover)
			must(err)
			if !m.IsFalse(handle) {
				t.Fatal("initialization retry lost the original cover")
			}
		})
		if t.Failed() {
			break
		}
	}
	if failures == 0 || !succeeded {
		t.Fatal("missing initialization failure/success frontier", failures, succeeded)
	}
}
