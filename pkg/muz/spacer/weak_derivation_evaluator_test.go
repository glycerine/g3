package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestSpacerWeakDerivationUsesSourceBooleanProbePolicy(t *testing.T) {
	for _, tc := range []struct {
		name          string
		weak, limited bool
		post          string
		keep          bool
	}{
		{"satisfied", true, false, "true", true},
		{"false-post", true, false, "false", false},
		{"unknown-post", true, false, "unknown", false},
		{"step-exception", true, true, "true", false},
		{"disabled", false, true, "true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			p := c.transformers[f.query]
			depthMust(t, p.SetWeakAbstraction(tc.weak))
			var pins []ast.ASTRef
			defer func() { depthMust(t, releaseNormalizeRefs(m, pins)) }()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				t.Helper()
				depthMust(t, err)
				ref, err := m.NewRef(h)
				depthMust(t, err)
				pins = append(pins, ref)
				return h
			}
			truth := own(m.MkTrue())
			falsity := own(m.MkFalse())
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			a := own(m.MkFreshConst("weak-transition", boolean))
			transition := own(m.MkNot(a))
			post := truth
			if tc.post == "false" {
				post = falsity
			}
			if tc.post == "unknown" {
				post = own(m.MkFreshConst("weak-post", boolean))
			}
			md, err := model.NewModel(m)
			depthMust(t, err)
			defer func() { depthMust(t, md.Close()) }()
			decl, err := m.GetDecl(a)
			depthMust(t, err)
			depthMust(t, md.Core().RegisterConstant(decl, falsity))
			parent, err := p.NewProofObligation(nil, 2, 0, truth, nil)
			depthMust(t, err)
			child, err := p.NewProofObligation(parent, 1, 1, truth, nil)
			depthMust(t, err)
			derivation, err := NewDerivation(parent, f.rule, transition, nil)
			depthMust(t, err)
			depthMust(t, child.SetDerivation(derivation))
			services := p.owner.contexts[0].config.ModelEvaluator
			params, err := services.Context.NewValue()
			depthMust(t, err)
			defer func() {
				p.owner.contexts[0].config.ModelEvaluator = services
				depthMust(t, services.Context.Del(&params))
			}()
			if tc.limited {
				key := "max_steps"
				depthMust(t, services.Context.SetUIntName(&params, &key, 0))
				config := services
				config.Params = &params
				p.owner.contexts[0].config.ModelEvaluator = config
			}
			depthMust(t, p.checkWeakDerivation(child, md.Core(), transition, post))
			_, found, err := child.Derivation()
			depthMust(t, err)
			if found != tc.keep {
				t.Fatal("weak derivation policy", found, tc.keep)
			}
			if !tc.keep && !derivation.closed {
				t.Fatal("discarded derivation was not closed")
			}
		})
	}
}
