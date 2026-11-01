package duality

import (
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func derivationEvaluatorFixture(t *testing.T, ctx *Context, globals *uparams.GlobalParamsService) model.ModelEvaluatorConfig {
	t.Helper()
	memory, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: ctx.manager.Budget()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := memory.Close(); err != nil {
			t.Error(err)
		}
	})
	cooperation, err := cooperate.NewCooperation(ctx.manager.Budget())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cooperation.Close(); err != nil {
			t.Error(err)
		}
	})
	return model.ModelEvaluatorConfig{Context: ctx.config.params, Globals: globals, Memory: memory, Cooperation: cooperation}
}
func TestDerivationFullChoicesUseModelAndPreserveLeafOrder(t *testing.T) {
	ctx, _, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("choose-R")
	must(err)
	closeExpr(t, term)
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(s))
	must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	derivation, err := NewDerivationTree(original, working)
	must(err)
	var originals, instances []*Node
	for n := 0; n < 3; n++ {
		node, err := original.CreateNode(term)
		must(err)
		originals = append(originals, node)
		_, err = original.CreateLowerBoundEdge(node)
		must(err)
		instance, err := derivation.CreateApproximatedInstance(node)
		must(err)
		instances = append(instances, instance)
	}
	// High-priority base heuristic is a no-op and requires no current model.
	expanded, err := derivation.ExpandSomeFullNodes(true, 10)
	must(err)
	if expanded || len(derivation.leaves) != 3 {
		t.Fatal("high priority expanded nodes")
	}
	must(working.Push())
	// Two leaves are active; the middle leaf must never be offered for expansion.
	for n, node := range instances {
		b, vars, err := working.RedVars(node)
		must(err)
		if len(vars) != 0 {
			t.Fatal("unexpected parameters")
		}
		constraint := b
		if n == 1 {
			constraint, err = b.Not()
			must(err)
		}
		h, err := constraint.Raw()
		must(err)
		must(s.Assert(h))
		if n == 1 {
			must(constraint.Close())
		}
		must(b.Close())
	}
	status, err := working.Check(instances[0], nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("expected SAT")
	}
	var choices []*Node
	must(derivation.ExpansionChoicesFull(&choices, false, false))
	if len(choices) != 2 || choices[0] != instances[0] || choices[1] != instances[2] {
		t.Fatal("inactive leaf included or order changed")
	}
	expanded, err = derivation.ExpandSomeFullNodes(false, 0)
	must(err)
	if !expanded || len(working.edges) != 0 || len(derivation.leaves) != 3 {
		t.Fatal("zero limit did not preserve source return behavior")
	}
	expanded, err = derivation.ExpandSomeFullNodes(false, 1)
	must(err)
	if !expanded || instances[0].Outgoing == nil || instances[2].Outgoing != nil || len(derivation.leaves) != 2 || derivation.leaves[0] != instances[1] || derivation.leaves[1] != instances[2] {
		t.Fatal("limited expansion changed leaf order")
	}
	must(derivation.Heuristic.Update(originals[2]))
	// Remaining active leaf still wins even after receiving an update.
	expanded, err = derivation.ExpandSomeFullNodes(false, 1)
	must(err)
	if !expanded || instances[2].Outgoing == nil || len(derivation.leaves) != 1 || derivation.leaves[0] != instances[1] {
		t.Fatal("remaining active leaf not expanded")
	}
	expanded, err = derivation.ExpandSomeFullNodes(false, 1)
	must(err)
	if expanded {
		t.Fatal("inactive leaf expanded")
	}
	must(working.Pop(1))
}
