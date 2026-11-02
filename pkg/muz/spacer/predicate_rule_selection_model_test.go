package spacer

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestSpacerRuleSelectionUsesDirectModelInterpretations(t *testing.T) {
	for _, tagState := range []string{"true", "false", "missing", "symbolic"} {
		for _, reachState := range []string{"missing", "true", "false", "symbolic"} {
			t.Run(fmt.Sprintf("tag=%s/reach=%s", tagState, reachState), func(t *testing.T) {
				reached := reachState == "false"
				f := newContextAnswerFixture(t, true)
				p, m := f.context.transformers[f.query], f.context.manager
				truth, err := m.MkTrue()
				depthMust(t, err)
				falsity, err := m.MkFalse()
				depthMust(t, err)
				fact, err := NewReachFact(m, f.rule, truth, nil, false)
				depthMust(t, err)
				_, err = p.AddReachFact(fact)
				depthMust(t, err)
				depthMust(t, fact.Close())
				md, err := model.NewModel(m)
				depthMust(t, err)
				defer func() { depthMust(t, md.Close()) }()
				sort, err := m.MkBoolSort()
				depthMust(t, err)
				symbolic, err := m.MkFreshConst("symbolic-selection-value", sort)
				depthMust(t, err)
				symbolicPin, err := m.NewRef(symbolic)
				depthMust(t, err)
				defer func() { depthMust(t, m.DelRef(&symbolicPin)) }()
				tag, err := p.RuleTag(f.rule)
				depthMust(t, err)
				tagDecl, err := m.GetDecl(tag)
				depthMust(t, err)
				if tagState != "missing" {
					value := truth
					if tagState == "false" {
						value = falsity
					}
					if tagState == "symbolic" {
						value = symbolic
					}
					depthMust(t, md.Core().RegisterConstant(tagDecl, value))
				}
				latest, found, err := p.LastReachCaseVariable()
				depthMust(t, err)
				if !found {
					t.Fatal("missing reach case")
				}
				origin, err := p.owner.FormulaNewToOld(latest, 0, false)
				depthMust(t, err)
				defer func() { depthMust(t, m.DelRef(&origin)) }()
				h, err := m.RefGet(origin)
				depthMust(t, err)
				originDecl, err := m.GetDecl(h)
				depthMust(t, err)
				if reachState != "missing" {
					value := truth
					if reached {
						value = falsity
					}
					if reachState == "symbolic" {
						value = symbolic
					}
					depthMust(t, md.Core().RegisterConstant(originDecl, value))
				}
				config := p.owner.contexts[0].config.ModelEvaluator
				params, err := config.Context.NewValue()
				depthMust(t, err)
				completion, steps := "completion", "max_steps"
				depthMust(t, config.Context.SetBoolName(&params, &completion, true))
				depthMust(t, config.Context.SetUIntName(&params, &steps, 0))
				override := config
				override.Params = &params
				p.owner.contexts[0].config.ModelEvaluator = override
				defer func() { p.owner.contexts[0].config.ModelEvaluator = config; depthMust(t, config.Context.Del(&params)) }()
				before := m.Stats().LiveReferences
				selection, err := p.SelectRule(md.Core())
				depthMust(t, err)
				defer func() { depthMust(t, selection.Close()) }()
				selected := tagState == "true"
				if (selection.Rule != nil) != selected {
					t.Fatal("wrong rule selection", tagState)
				}
				if selected {
					same, err := selection.Rule.Same(f.rule)
					depthMust(t, err)
					if !same || selection.IsConcrete != reached || len(selection.ReachPredUsed) != 1 || selection.ReachPredUsed[0] != reached {
						t.Fatal("wrong predecessor selection", selection)
					}
					expected := uint32(0)
					if reached {
						expected = 1
					}
					if selection.NumReuseReach != expected {
						t.Fatal("wrong reuse count", selection.NumReuseReach)
					}
				} else if !selection.IsConcrete || selection.NumReuseReach != 0 || len(selection.ReachPredUsed) != 0 {
					t.Fatal("wrong extension selection", selection)
				}
				if got := m.Stats().LiveReferences; got != before {
					t.Fatal("selection leaked references", got, before)
				}
				for _, decl := range []ast.ASTHandle{tagDecl, originDecl} {
					exists, err := md.Core().HasConstantInterpretation(decl)
					depthMust(t, err)
					want := reachState != "missing"
					if decl == tagDecl {
						want = tagState != "missing"
					}
					if exists != want {
						t.Fatal("selection completed a model interpretation", exists, want)
					}
				}
			})
		}
	}
}

func TestSpacerRuleSelectionPrefersConcreteLaterRule(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	m := f.context.manager
	head, err := m.MkApp(f.query, nil)
	depthMust(t, err)
	concrete, err := f.manager.NewRule(head, nil, nil, "concrete-fallback")
	depthMust(t, err)
	defer func() { depthMust(t, concrete.Close()) }()
	depthMust(t, f.rules.Add(concrete))
	depthMust(t, f.context.UpdateRules(f.rules))
	p := f.context.transformers[f.query]
	if len(p.rules) != 2 {
		t.Fatal("expected two candidate rules")
	}
	first, err := p.rules[0].rule.Same(f.rule)
	depthMust(t, err)
	if !first {
		t.Fatal("fixture did not put the nonconcrete rule first")
	}
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	truth, err := m.MkTrue()
	depthMust(t, err)
	for _, rule := range p.rules {
		decl, err := m.GetDecl(rule.tag)
		depthMust(t, err)
		depthMust(t, md.Core().RegisterConstant(decl, truth))
	}
	before := m.Stats().LiveReferences
	selection, err := p.SelectRule(md.Core())
	depthMust(t, err)
	if selection.Rule == nil {
		t.Fatal("no selected rule")
	}
	same, err := selection.Rule.Same(concrete)
	depthMust(t, err)
	if !same || !selection.IsConcrete || selection.NumReuseReach != 0 || len(selection.ReachPredUsed) != 0 {
		depthMust(t, selection.Close())
		t.Fatal("later concrete rule did not replace nonconcrete selection")
	}
	depthMust(t, selection.Close())
	if m.Stats().LiveReferences != before {
		t.Fatal("replacing the selected rule leaked references")
	}
}
