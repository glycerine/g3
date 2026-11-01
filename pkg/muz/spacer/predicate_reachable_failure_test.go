package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerReachableColdFailureCleanupAndRetry(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 1024 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			var budget *arena.Budget
			t.Cleanup(func() {
				if used := budget.Used(); used != 0 {
					t.Errorf("reachable leaked %d budget bytes", used)
				}
			})
			m, globals := newSpacerTestServices(t)
			budget = m.Budget()
			owner, err := NewManager(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
			depthMust(t, err)
			defer func() { depthMust(t, owner.Close()) }()
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			name, err := m.Symbols.Intern("reachable-failure")
			depthMust(t, err)
			predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			depthMust(t, err)
			pt, err := NewRulePredicateTransformer(owner, predicate, PropSolverConfig{})
			depthMust(t, err)
			defer func() { depthMust(t, pt.Close()) }()
			rules, err := base.NewRuleManager(m)
			depthMust(t, err)
			defer func() { depthMust(t, rules.Close()) }()
			depthMust(t, rules.RegisterPredicate(predicate))
			truth, err := m.MkTrue()
			depthMust(t, err)
			head, err := m.MkApp(predicate, []ast.ASTHandle{truth})
			depthMust(t, err)
			rule, err := rules.NewRule(head, nil, nil, "reachable-failure")
			depthMust(t, err)
			defer func() { depthMust(t, rule.Close()) }()
			signature, err := pt.Signature()
			depthMust(t, err)
			currentDecl, err := owner.OldToNew(signature[0], 0)
			depthMust(t, err)
			current, err := m.MkApp(currentDecl, nil)
			depthMust(t, err)
			equality, err := m.MkEq(current, truth)
			depthMust(t, err)
			fact, err := NewReachFact(m, rule, equality, nil, true)
			depthMust(t, err)
			_, err = pt.AddReachFact(fact)
			depthMust(t, err)
			depthMust(t, fact.Close())
			before := m.Stats().LiveReferences
			count, err := m.GetRefCount(current)
			depthMust(t, err)
			m.Budget().FailAfter(stop)
			result, err := pt.Reachable()
			m.Budget().FailAfter(-1)
			if result != (ast.ASTRef{}) {
				depthMust(t, m.DelRef(&result))
			}
			if err != nil {
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					depthMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal(err)
				}
				failures++
			} else {
				succeeded = true
			}
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("reachable leaked reference slots", got, before)
			}
			after, err := m.GetRefCount(current)
			depthMust(t, err)
			if after != count {
				t.Fatal("reachable leaked source ownership", after, count)
			}
			retry, err := pt.Reachable()
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&retry)) }()
			actual, err := m.RefGet(retry)
			depthMust(t, err)
			expected, err := m.MkVar(0, boolean)
			depthMust(t, err)
			if actual != expected {
				t.Fatal("retry did not rename and simplify the reach fact")
			}
		})
		if t.Failed() {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing failure/success frontier", failures, succeeded)
	}
}
