package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

func newThreeRuleSelectionFixture(t *testing.T, reachUsed, factEnabled bool) (*PredicateTransformer, *model.Model, []*base.Rule) {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("rule selection fixture leaked bytes", budget.Used())
		}
	})
	f := newContextAnswerFixture(t, true)
	m := f.context.manager
	budget = m.Budget()
	head, err := m.MkApp(f.query, nil)
	depthMust(t, err)
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	guard, err := m.MkFreshConst("selection-second-guard", boolean)
	depthMust(t, err)
	second, err := f.manager.NewRule(head, []ast.ASTHandle{head, guard}, nil, "second-recursive")
	depthMust(t, err)
	t.Cleanup(func() { depthMust(t, second.Close()) })
	fact, err := f.manager.NewRule(head, nil, nil, "third-fact")
	depthMust(t, err)
	t.Cleanup(func() { depthMust(t, fact.Close()) })
	depthMust(t, f.rules.Add(second))
	depthMust(t, f.rules.Add(fact))
	depthMust(t, f.context.UpdateRules(f.rules))
	p := f.context.transformers[f.query]
	expected := []*base.Rule{f.rule, second, fact}
	if len(p.rules) != len(expected) {
		t.Fatal("wrong candidate count")
	}
	md, err := model.NewModel(m)
	depthMust(t, err)
	t.Cleanup(func() { depthMust(t, md.Close()) })
	truth, err := m.MkTrue()
	depthMust(t, err)
	falsity, err := m.MkFalse()
	depthMust(t, err)
	for i, r := range p.rules {
		same, e := r.rule.Same(expected[i])
		depthMust(t, e)
		if !same {
			t.Fatal("unexpected candidate order", i)
		}
		decl, e := m.GetDecl(r.tag)
		depthMust(t, e)
		value := truth
		if i == 2 && !factEnabled {
			value = falsity
		}
		depthMust(t, md.Core().RegisterConstant(decl, value))
	}
	latest, found, err := p.LastReachCaseVariable()
	depthMust(t, err)
	if !found {
		t.Fatal("missing reach case")
	}
	origin, err := p.owner.FormulaNewToOld(latest, 0, false)
	depthMust(t, err)
	h, err := m.RefGet(origin)
	depthMust(t, err)
	declaration, err := m.GetDecl(h)
	depthMust(t, err)
	value := truth
	if reachUsed {
		value = falsity
	}
	depthMust(t, md.Core().RegisterConstant(declaration, value))
	depthMust(t, m.DelRef(&origin))
	return p, md, expected
}

func checkThreeRuleSelection(t *testing.T, p *PredicateTransformer, md *model.Model, expected *base.Rule, concrete bool, predecessors int) {
	t.Helper()
	before := p.manager.Stats().LiveReferences
	selection, err := p.SelectRule(md.Core())
	depthMust(t, err)
	defer func() { depthMust(t, selection.Close()) }()
	if selection.Rule == nil {
		t.Fatal("no selected rule")
	}
	same, err := selection.Rule.Same(expected)
	depthMust(t, err)
	reuse := uint32(0)
	if predecessors > 0 && concrete {
		reuse = uint32(predecessors)
	}
	if !same || selection.IsConcrete != concrete || len(selection.ReachPredUsed) != predecessors || selection.NumReuseReach != reuse {
		t.Fatal("wrong candidate preference or stale predecessor state", selection.IsConcrete, selection.NumReuseReach, selection.ReachPredUsed)
	}
	for _, used := range selection.ReachPredUsed {
		if used != concrete {
			t.Fatal("wrong predecessor use")
		}
	}
	depthMust(t, selection.Close())
	if p.manager.Stats().LiveReferences != before {
		t.Fatal("selection leaked references")
	}
}

func TestSpacerThreeRuleSelectionPreferenceAndReuse(t *testing.T) {
	for _, tc := range []struct {
		reach, fact  bool
		index        int
		concrete     bool
		predecessors int
	}{{false, false, 1, false, 1}, {false, true, 2, true, 0}, {true, true, 0, true, 1}} {
		t.Run(fmt.Sprintf("reach=%t/fact=%t", tc.reach, tc.fact), func(t *testing.T) {
			p, md, rules := newThreeRuleSelectionFixture(t, tc.reach, tc.fact)
			checkThreeRuleSelection(t, p, md, rules[tc.index], tc.concrete, tc.predecessors)
		})
	}
}

func TestSpacerRuleSelectionAllocationFailureAndRetry(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 256 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			p, md, rules := newThreeRuleSelectionFixture(t, false, true)
			m := p.manager
			// Exhaust the existing reference slots without allowing a new page.
			// Selection must then cross a real AST reference allocation boundary.
			var held []ast.ASTRef
			t.Cleanup(func() {
				for i := range held {
					depthMust(t, m.DelRef(&held[i]))
				}
			})
			frontier := false
			for i := 0; i < 4096; i++ {
				m.Budget().FailAfter(0)
				ref, e := m.NewRef(ast.ASTHandle{})
				m.Budget().FailAfter(-1)
				if errors.Is(e, arena.ErrInjectedFailure) {
					frontier = true
					break
				}
				depthMust(t, e)
				held = append(held, ref)
			}
			if !frontier {
				t.Fatal("reference allocation boundary not reached")
			}
			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			selection, err := p.SelectRule(md.Core())
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				if selection.Rule != nil || selection.IsConcrete || selection.NumReuseReach != 0 || len(selection.ReachPredUsed) != 0 {
					t.Fatal("failed selection published partial state")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(err, &cleanup) {
					depthMust(t, cleanup.Cleanup())
				}
				var failure memory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &failure) || failure.Kind != memory.FailureMemout) {
					t.Fatal("unexpected selection error", err)
				}
			} else {
				succeeded = true
				depthMust(t, selection.Close())
			}
			if m.Stats().LiveReferences != before {
				t.Fatal("failed selection leaked references", stop)
			}
			checkThreeRuleSelection(t, p, md, rules[2], true, 0)
		})
		if t.Failed() {
			return
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("missing selection allocation failure/success frontier", failures, succeeded)
	}
}
