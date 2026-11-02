package spacer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestSpacerReachSelectionCatchesSourceEvaluatorExceptions(t *testing.T) {
	for _, origin := range []bool{false, true} {
		for _, key := range []string{"max_steps", "max_memory"} {
			name := key + "/current"
			if origin {
				name = key + "/origin"
			}
			t.Run(name, func(t *testing.T) {
				f := newContextAnswerFixture(t, false)
				p, m := f.context.transformers[f.query], f.context.manager
				md, err := model.NewModel(m)
				depthMust(t, err)
				defer func() { depthMust(t, md.Close()) }()

				boolean, err := m.MkBoolSort()
				depthMust(t, err)
				caseVariable, err := m.MkFreshConst("exception-reach-case", boolean)
				depthMust(t, err)
				var shifted ast.ASTRef
				probe := caseVariable
				if origin {
					shifted, err = p.owner.FormulaNewToOld(caseVariable, 2, false)
					depthMust(t, err)
					defer func() { depthMust(t, m.DelRef(&shifted)) }()
					probe, err = m.RefGet(shifted)
					depthMust(t, err)
				}
				declaration, err := m.GetDecl(probe)
				depthMust(t, err)
				falsity, err := m.MkFalse()
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(declaration, falsity))
				// The source helper accepts arbitrary expressions even though the
				// normal producer supplies a nullary case application. Force this
				// unused case through the rewriter so the typed exception boundary
				// is observable, then flip its model value for the retry below.
				selector, err := m.MkNot(caseVariable)
				depthMust(t, err)
				depthMust(t, m.SetRef(p.reachCaseVariables[0], selector))

				params, err := p.owner.contexts[0].config.ModelEvaluator.Context.NewValue()
				depthMust(t, err)
				defer func() { depthMust(t, p.owner.contexts[0].config.ModelEvaluator.Context.Del(&params)) }()
				depthMust(t, p.owner.contexts[0].config.ModelEvaluator.Context.SetUIntName(&params, &key, 0))
				var output bytes.Buffer
				tags := diagnostic.NewTraceTagsWithWriter(&output)
				tags.Enable("spacer_model_evaluator")
				config := p.owner.contexts[0].config.ModelEvaluator
				config.Params = &params
				config.Trace = tags
				if key == "max_memory" {
					block, err := config.Memory.Allocate(16)
					depthMust(t, err)
					defer func() { depthMust(t, config.Memory.Deallocate(&block)) }()
				}
				p.owner.contexts[0].config.ModelEvaluator = config
				before := m.Stats().LiveReferences
				selected, found, err := selectReachFactForExceptionTest(p, md.Core(), origin)
				depthMust(t, err)
				if found || selected != nil {
					t.Fatal("unknown evaluation selected a reach fact")
				}
				if m.Stats().LiveReferences != before {
					t.Fatal("caught evaluator exception leaked references")
				}
				expected := messages.MaxSteps
				if key == "max_memory" {
					expected = messages.MaxMemory
				}
				if tags.ShouldTrace("spacer_model_evaluator") && !strings.Contains(output.String(), expected+"\n") {
					t.Fatal("missing evaluator exception trace", output.String())
				}

				config.Params = nil
				p.owner.contexts[0].config.ModelEvaluator = config
				truth, err := m.MkTrue()
				depthMust(t, err)
				depthMust(t, md.Core().RegisterConstant(declaration, truth))
				selected, found, err = selectReachFactForExceptionTest(p, md.Core(), origin)
				depthMust(t, err)
				if !found || selected == nil || selected.state != p.reachFacts[0].state {
					t.Fatal("unlimited retry did not select the reach fact")
				}
				depthMust(t, selected.Close())
			})
		}
	}
}

func selectReachFactForExceptionTest(p *PredicateTransformer, md *model.ModelCore, origin bool) (*ReachFact, bool, error) {
	if origin {
		return p.UsedOriginReachFact(md, 2)
	}
	return p.UsedReachFact(md, true)
}
