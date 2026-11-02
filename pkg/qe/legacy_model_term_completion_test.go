package qe

// Source: spacer_util.cpp model_evaluator_util::eval and mbqi_project_var.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLegacyModelTermProbeForcesNonCompletingEvaluation(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	p, err := NewLegacyProjection(m, f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, p.Close()) }()
	x := f.constant("noncompletion-x", f.integer)
	k := f.constant("noncompletion-k", f.integer)
	formula := f.own(m.MkEq(x, k))
	current, err := m.NewRef(formula)
	fmMust(t, err)
	defer func() { fmMust(t, m.DelRef(&current)) }()
	md, err := model.NewModel(m)
	fmMust(t, err)
	defer func() { fmMust(t, md.Close()) }()
	evaluator, err := newLegacyModelEvaluator(md.Core(), model.ModelEvaluatorConfig{
		Context: f.r.config.Context, Globals: f.r.config.Globals,
		Memory: m.NumeralService().RuntimeOwner(), Cooperation: f.r.config.Cooperation,
	})
	fmMust(t, err)
	defer func() { fmMust(t, evaluator.Close()) }()
	// The retained caller can reuse an evaluator whose previous operation used
	// completion. Source model_evaluator_util resets false on every MBQI probe.
	fmMust(t, evaluator.SetModelCompletion(true))
	eliminated, err := p.projectModelTerm(evaluator, current, x)
	fmMust(t, err)
	if eliminated {
		t.Fatal("partial-model probe used stale completion and equated absent constants")
	}
	actual, err := m.RefGet(current)
	fmMust(t, err)
	if actual != formula {
		t.Fatal("non-completing refusal changed the formula", actual, formula)
	}
	for _, constant := range []ast.ASTHandle{x, k} {
		decl, err := m.GetDecl(constant)
		fmMust(t, err)
		present, err := md.Core().HasConstantInterpretation(decl)
		fmMust(t, err)
		if present {
			t.Fatal("probe completed an absent model constant", constant)
		}
	}
}
