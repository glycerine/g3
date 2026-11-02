package spacer

// Source: spacer_context.cpp get_used_reach_fact/get_used_origin_reach_fact
// evaluate case variables with model_completion=false and choose the first false.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestSpacerReachFactSelectionDisablesCompletionAndPreservesOrder(t *testing.T) {
	for _, origin := range []bool{false, true} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("origin=%t/all=%t", origin, all), func(t *testing.T) {
				f := newContextAnswerFixture(t, false)
				p, m := f.context.transformers[f.query], f.context.manager
				boolean, err := m.MkBoolSort()
				depthMust(t, err)
				for i := 0; i < 2; i++ {
					formula, err := m.MkFreshConst("selection-fact", boolean)
					depthMust(t, err)
					fact, err := NewReachFact(m, f.rule, formula, nil, false)
					depthMust(t, err)
					added, err := p.AddReachFact(fact)
					depthMust(t, err)
					depthMust(t, fact.Close())
					if !added {
						t.Fatal("missing distinct reach fact")
					}
				}
				if p.initialReachFacts != 1 || len(p.reachFacts) != 3 {
					t.Fatal("invalid selection fixture")
				}
				config := p.owner.contexts[0].config.ModelEvaluator
				params, err := config.Context.NewValue()
				depthMust(t, err)
				completion := "completion"
				depthMust(t, config.Context.SetBoolName(&params, &completion, true))
				override := config
				override.Params = &params
				p.owner.contexts[0].config.ModelEvaluator = override
				defer func() { p.owner.contexts[0].config.ModelEvaluator = config; depthMust(t, config.Context.Del(&params)) }()
				md, err := model.NewModel(m)
				depthMust(t, err)
				defer func() { depthMust(t, md.Close()) }()
				falsity, err := m.MkFalse()
				depthMust(t, err)
				start := 0
				if !all && !origin {
					start = int(p.initialReachFacts)
				}
				cases := make([]ast.ASTHandle, len(p.reachFacts))
				for i, ref := range p.reachCaseVariables {
					h, err := m.RefGet(ref)
					depthMust(t, err)
					if origin {
						shifted, err := p.owner.FormulaNewToOld(h, 2, false)
						depthMust(t, err)
						h, err = m.RefGet(shifted)
						depthMust(t, err)
						// Keep each translated variable alive through the selection call.
						defer func() { depthMust(t, m.DelRef(&shifted)) }()
					}
					cases[i] = h
					if i == start {
						continue
					} // The first candidate is deliberately unknown.
					decl, err := m.GetDecl(h)
					depthMust(t, err)
					depthMust(t, md.Core().RegisterConstant(decl, falsity))
				}
				before := m.Stats().LiveReferences
				var selected *ReachFact
				var found bool
				if origin {
					selected, found, err = p.UsedOriginReachFact(md.Core(), 2)
				} else {
					selected, found, err = p.UsedReachFact(md.Core(), all)
				}
				depthMust(t, err)
				if !found || selected.state != p.reachFacts[start+1].state {
					t.Fatal("selection did not skip unknown case and choose first false", found)
				}
				depthMust(t, selected.Close())
				if got := m.Stats().LiveReferences; got != before {
					t.Fatal("selection leaked temporary references", got, before)
				}
				decl, err := m.GetDecl(cases[start])
				depthMust(t, err)
				completed, err := md.Core().HasConstantInterpretation(decl)
				depthMust(t, err)
				if completed {
					t.Fatal("selection completed an unknown case variable")
				}
			})
		}
	}
}

func TestSpacerReachFactSelectionWithoutFalseCase(t *testing.T) {
	for _, origin := range []bool{false, true} {
		for _, known := range []bool{false, true} {
			t.Run(fmt.Sprintf("origin=%t/known=%t", origin, known), func(t *testing.T) {
				f := newContextAnswerFixture(t, false)
				p, m := f.context.transformers[f.query], f.context.manager
				md, err := model.NewModel(m)
				depthMust(t, err)
				defer func() { depthMust(t, md.Close()) }()
				truth, err := m.MkTrue()
				depthMust(t, err)
				var cases []ast.ASTHandle
				for _, ref := range p.reachCaseVariables {
					h, err := m.RefGet(ref)
					depthMust(t, err)
					if origin {
						shifted, err := p.owner.FormulaNewToOld(h, 2, false)
						depthMust(t, err)
						defer func() { depthMust(t, m.DelRef(&shifted)) }()
						h, err = m.RefGet(shifted)
						depthMust(t, err)
					}
					if !origin {
						// OriginSummary owns cached origin declarations through
						// the multiplexer. Prepare that persistent mapping before
						// measuring temporary selection/summary references.
						shifted, e := p.owner.FormulaNewToOld(h, 2, false)
						depthMust(t, e)
						depthMust(t, m.DelRef(&shifted))
					}
					cases = append(cases, h)
					if known {
						decl, err := m.GetDecl(h)
						depthMust(t, err)
						depthMust(t, md.Core().RegisterConstant(decl, truth))
					}
				}
				if len(cases) == 0 {
					t.Fatal("fixture has no reach cases")
				}
				before := m.Stats().LiveReferences
				var selected *ReachFact
				var found bool
				if origin {
					selected, found, err = p.UsedOriginReachFact(md.Core(), 2)
				} else {
					selected, found, err = p.UsedReachFact(md.Core(), true)
				}
				depthMust(t, err)
				if found || selected != nil {
					if selected != nil {
						depthMust(t, selected.Close())
					}
					t.Fatal("selected a reach fact without a false case")
				}
				if m.Stats().LiveReferences != before {
					t.Fatal("no-selection path leaked references")
				}
				// A production consumer must reject the missing must-summary,
				// preserving the source SASSERT(res) invariant as an error.
				summary, auxiliaries, cause := p.OriginSummary(md.Core(), 0, 2, true)
				if !errors.Is(cause, ast.ErrASTDomain) || summary != (ast.ASTRef{}) || len(auxiliaries) != 0 {
					t.Fatal("missing must fact did not reject summary", summary, auxiliaries, cause)
				}
				if m.Stats().LiveReferences != before {
					t.Fatal("missing must-summary leaked references")
				}

				for _, h := range cases {
					decl, err := m.GetDecl(h)
					depthMust(t, err)
					interpreted, err := md.Core().HasConstantInterpretation(decl)
					depthMust(t, err)
					if interpreted != known {
						t.Fatal("selection changed model completion", known, interpreted)
					}
				}
			})
		}
	}
}

func TestSpacerReachFactSelectionWithZeroStepBudget(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	p, m := f.context.transformers[f.query], f.context.manager
	config := p.owner.contexts[0].config.ModelEvaluator
	params, err := config.Context.NewValue()
	depthMust(t, err)
	key := "max_steps"
	depthMust(t, config.Context.SetUIntName(&params, &key, 0))
	limited := config
	limited.Params = &params
	p.owner.contexts[0].config.ModelEvaluator = limited
	defer func() { p.owner.contexts[0].config.ModelEvaluator = config; depthMust(t, config.Context.Del(&params)) }()
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	if len(p.reachCaseVariables) == 0 {
		t.Fatal("missing reach cases")
	}
	h, err := m.RefGet(p.reachCaseVariables[0])
	depthMust(t, err)
	decl, err := m.GetDecl(h)
	depthMust(t, err)
	falsity, err := m.MkFalse()
	depthMust(t, err)
	depthMust(t, md.Core().RegisterConstant(decl, falsity))
	before := m.Stats().LiveReferences
	fact, found, err := p.UsedReachFact(md.Core(), true)
	depthMust(t, err)
	if !found || fact == nil {
		t.Fatal("zero-step constant case was not selected")
	}
	if fact.state != p.reachFacts[0].state {
		depthMust(t, fact.Close())
		t.Fatal("wrong constant case")
	}
	depthMust(t, fact.Close())

	if m.Stats().LiveReferences != before {
		t.Fatal("limited selection leaked references")
	}
}

func TestSpacerReachSelectionSkipsNonliteralModelInterpretation(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	p, m := f.context.transformers[f.query], f.context.manager
	boolean, err := m.MkBoolSort()
	depthMust(t, err)
	formula, err := m.MkFreshConst("after-evaluator-failure", boolean)
	depthMust(t, err)
	fact, err := NewReachFact(m, f.rule, formula, nil, false)
	depthMust(t, err)
	added, err := p.AddReachFact(fact)
	depthMust(t, err)
	depthMust(t, fact.Close())
	if !added || len(p.reachCaseVariables) != 2 {
		t.Fatal("invalid nonliteral selection fixture")
	}
	config := p.owner.contexts[0].config.ModelEvaluator
	params, err := config.Context.NewValue()
	depthMust(t, err)
	key := "max_steps"
	depthMust(t, config.Context.SetUIntName(&params, &key, 0))
	limited := config
	limited.Params = &params
	p.owner.contexts[0].config.ModelEvaluator = limited
	defer func() { p.owner.contexts[0].config.ModelEvaluator = config; depthMust(t, config.Context.Del(&params)) }()
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	atom, err := m.MkFreshConst("evaluation-rewrite-input", boolean)
	depthMust(t, err)
	atomDecl, err := m.GetDecl(atom)
	depthMust(t, err)
	yes, err := m.MkTrue()
	depthMust(t, err)
	depthMust(t, md.Core().RegisterConstant(atomDecl, yes))
	compound, err := m.MkNot(atom)
	depthMust(t, err)
	compoundPin, err := m.NewRef(compound)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&compoundPin)) }()
	no, err := m.MkFalse()
	depthMust(t, err)
	for i, ref := range p.reachCaseVariables {
		h, err := m.RefGet(ref)
		depthMust(t, err)
		decl, err := m.GetDecl(h)
		depthMust(t, err)
		value := compound
		if i == 1 {
			value = no
		}
		depthMust(t, md.Core().RegisterConstant(decl, value))
	}
	before := m.Stats().LiveReferences
	selected, found, err := p.UsedReachFact(md.Core(), true)
	depthMust(t, err)
	if !found || selected == nil {
		t.Fatal("nonliteral interpretation hid the later false case")
	}
	if selected.state != p.reachFacts[1].state {
		depthMust(t, selected.Close())
		t.Fatal("wrong reach fact after nonliteral interpretation")
	}
	depthMust(t, selected.Close())
	if m.Stats().LiveReferences != before {
		t.Fatal("nonliteral selection leaked references")
	}
}
