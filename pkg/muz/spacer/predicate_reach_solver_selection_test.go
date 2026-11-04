package spacer

// Source: spacer_context.cpp add_reach_fact encodes a chain of case
// variables consumed by is_must_reachable and get_used_reach_fact.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMustReachabilitySolverSelectsCompatibleFact(t *testing.T) {
	for _, reachAsInitial := range []bool{false, true} {
		t.Run(fmt.Sprintf("reach-as-initial=%t", reachAsInitial), func(t *testing.T) {
			f := newContextAnswerFixture(t, true)
			p, m := f.context.transformers[f.query], f.context.manager
			depthMust(t, p.SetReachAsInitial(reachAsInitial))
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			a, err := m.MkFreshConst("reach-solver-state", boolean)
			depthMust(t, err)
			notA, err := m.MkNot(a)
			depthMust(t, err)
			facts := []ast.ASTHandle{a, notA}
			selected := make([]*reachFactState, 2)
			for i, formula := range facts {
				fact, err := NewReachFact(m, f.rule, formula, nil, false)
				depthMust(t, err)
				selected[i] = fact.state
				added, err := p.AddReachFact(fact)
				depthMust(t, err)
				depthMust(t, fact.Close())
				if !added {
					t.Fatal("fact not added")
				}
			}
			falsity, err := m.MkFalse()
			depthMust(t, err)
			for _, i := range []int{0, 1, -1, 0, 1} {
				state := falsity
				if i >= 0 {
					state = facts[i]
				}
				reachable, md, err := p.IsMustReachable(state, true)
				depthMust(t, err)
				if i < 0 {
					if reachable || md != nil {
						t.Fatal("false state was reachable")
					}
					continue
				}
				if !reachable || md == nil {
					t.Fatal("expected model for reachable state", i)
				}
				fact, found, err := p.UsedReachFact(md, true)
				depthMust(t, err)
				if !found || fact.state != selected[i] {
					t.Fatal("solver selected incompatible fact", i)
				}
				depthMust(t, fact.Close())
				depthMust(t, md.DecRef())
				reachable, md, err = p.IsMustReachable(state, false)
				depthMust(t, err)
				if !reachable || md != nil {
					t.Fatal("no-model reachability result", i)
				}
			}
		})
	}
}
