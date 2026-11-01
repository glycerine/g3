package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerDerivationSummaryUsesSourceBooleanProbePolicy(t *testing.T) {
	for _, name := range []string{"true", "false", "unknown", "limited", "quiet-unknown"} {
		t.Run(name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			p, m := f.context.transformers[f.query], f.context.manager
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			a, err := m.MkFreshConst("active-summary", boolean)
			depthMust(t, err)
			summary, err := m.MkNot(a)
			depthMust(t, err)
			pin, err := m.NewRef(summary)
			depthMust(t, err)
			defer func() { depthMust(t, m.DelRef(&pin)) }()
			truth, err := m.MkTrue()
			depthMust(t, err)
			falsity, err := m.MkFalse()
			depthMust(t, err)
			parent, err := p.NewProofObligation(nil, 2, 0, truth, nil)
			depthMust(t, err)
			d, err := NewDerivation(parent, f.rule, truth, nil)
			depthMust(t, err)
			defer func() { depthMust(t, d.Close()) }()
			depthMust(t, d.AddPremise(p, 0, summary, false, nil))
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			decl, err := m.GetDecl(a)
			depthMust(t, err)
			if name != "unknown" && name != "quiet-unknown" {
				value := falsity
				if name == "false" {
					value = truth
				}
				depthMust(t, md.Core().RegisterConstant(decl, value))
			}
			config := p.owner.contexts[0].config.ModelEvaluator
			params, err := config.Context.NewValue()
			depthMust(t, err)
			completion, steps := "completion", "max_steps"
			depthMust(t, config.Context.SetBoolName(&params, &completion, true))
			if name == "limited" {
				depthMust(t, config.Context.SetUIntName(&params, &steps, 0))
			}
			override := config
			override.Params = &params
			p.owner.contexts[0].config.ModelEvaluator = override
			defer func() { p.owner.contexts[0].config.ModelEvaluator = config; depthMust(t, config.Context.Del(&params)) }()
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			depthMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			depthMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
			depthMust(t, err)
			defer func() {
				depthMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				depthMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				depthMust(t, stream.Close())
			}()
			level := uint32(1)
			if name == "quiet-unknown" {
				level = 0
			}
			depthMust(t, diagnostic.SetProcessVerbosityLevel(level))
			depthMust(t, diagnostic.SetProcessVerboseStream(stream))
			before := m.Stats().LiveReferences
			child, err := d.CreateFirstChild(md.Core())
			depthMust(t, err)
			if (child != nil) != (name == "true") {
				t.Fatal("unexpected child for active summary", name, child)
			}
			if child == nil && m.Stats().LiveReferences != before {
				t.Fatal("failed summary leaked references")
			}
			message, err := stream.String()
			depthMust(t, err)
			expected := ""
			if name != "true" && level != 0 {
				expected = "Summary unexpectendly not true\n"
			}
			if message != expected {
				t.Fatal("wrong source summary diagnostic", message, expected)
			}
			if name == "unknown" || name == "quiet-unknown" {
				completed, err := md.Core().HasConstantInterpretation(decl)
				depthMust(t, err)
				if completed {
					t.Fatal("active-summary probe completed the model")
				}
			}
			if child == nil {
				p.owner.contexts[0].config.ModelEvaluator = config
				depthMust(t, md.Core().RegisterConstant(decl, falsity))
				retry, err := d.CreateFirstChild(md.Core())
				depthMust(t, err)
				if retry == nil {
					t.Fatal("retry did not create the active child")
				}
			}
		})
	}
}
