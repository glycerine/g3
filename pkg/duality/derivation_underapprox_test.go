package duality

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/lbool"
	"strings"
	"testing"
)

func TestDerivationUnderapproximationChoicesSplitConflictingLeaves(t *testing.T) {
	ctx, _, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	p, err := ctx.BoolConst("under-choice-parent")
	must(err)
	closeExpr(t, p)
	a, err := ctx.BoolConst("under-choice-a")
	must(err)
	closeExpr(t, a)
	b, err := ctx.BoolConst("under-choice-b")
	must(err)
	closeExpr(t, b)
	ad, err := a.Decl()
	must(err)
	closeDecl(t, ad)
	bd, err := b.Decl()
	must(err)
	closeDecl(t, bd)
	conjunction, err := a.And(b)
	must(err)
	closeExpr(t, conjunction)
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(s))
	must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	parent, err := original.CreateNode(p)
	must(err)
	must(parent.Bound.SetEmpty())
	first, err := original.CreateNode(a)
	must(err)
	must(first.Underapprox.SetFull())
	second, err := original.CreateNode(b)
	must(err)
	must(second.Underapprox.SetFull())
	formula, err := NewTransformer(ctx, []*FuncDecl{ad, bd}, nil, conjunction)
	must(err)
	defer formula.Close()
	_, err = original.CreateEdge(parent, formula, []*Node{first, second})
	must(err)
	_, err = original.CreateLowerBoundEdge(first)
	must(err)
	_, err = original.CreateLowerBoundEdge(second)
	must(err)
	derivation, err := NewDerivationTree(original, working)
	must(err)
	derivation.Underapprox = true
	top, err := derivation.CreateApproximatedInstance(parent)
	must(err)
	derivation.Top = top
	must(working.Push())
	must(working.AssertNode(top))
	must(derivation.ExpandMaterializedNode(top))
	children := top.Outgoing.Children
	// The current instance of the second leaf has no usable lower approximation.
	must(children[1].Underapprox.SetEmpty())
	// Rebuild its guarded edge formula to incorporate the changed lower fact.
	must(working.Pop(1))
	must(working.Push())
	must(working.AssertNode(top))
	must(working.AssertEdge(top.Outgoing, 0, true, true))
	derivation.leaves = append([]*Node(nil), children...)
	status, err := working.Check(top, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("base graph not SAT")
	}
	var output bytes.Buffer
	derivation.Output = &output
	var choices []*Node
	must(derivation.ExpansionChoices(&choices, false, true))
	if len(choices) != 1 || choices[0] != children[1] {
		t.Fatal("wrong conflict expansion choices")
	}
	expectedUsing := fmt.Sprintf("Using underapprox of %d ", children[0].Number)
	expectedReject := fmt.Sprintf("Not using underapprox of %d", children[1].Number)
	if !strings.Contains(output.String(), expectedUsing) || !strings.Contains(output.String(), expectedReject) {
		t.Fatal("source choice diagnostics", output.String())
	}
	// Search refreshes its model before the next expansion-selection round.
	status, err = working.Check(top, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("refresh model")
	}
	// A prior conflict remains selected without another recursive probe.
	output.Reset()
	choices = nil
	must(derivation.ExpansionChoices(&choices, false, false))
	if len(choices) != 1 || choices[0] != children[1] || strings.Contains(output.String(), expectedReject) {
		t.Fatal("prior choices were not reused")
	}
	// Constrained mode uses full model-filtered selection instead of probes.
	status, err = working.Check(top, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("refresh model")
	}
	derivation.Constrained = true
	choices = nil
	output.Reset()
	must(derivation.ExpansionChoices(&choices, false, false))
	if len(choices) != 2 || output.Len() != 0 {
		t.Fatal("constrained mode did not use full choices")
	}
	derivation.Constrained = false
	must(first.Underapprox.SetEmpty())
	choices = nil
	output.Reset()
	must(derivation.ExpansionChoices(&choices, false, false))
	if len(choices) != 2 || output.Len() != 0 {
		t.Fatal("empty original approximation did not select expansion directly")
	}
	falsity, err := ctx.BoolVal(false)
	must(err)
	must(working.ConstrainEdgeLocalized(top.Outgoing, falsity))
	must(falsity.Close())
	choices = nil
	if err = derivation.ExpansionChoices(&choices, false, false); !errors.Is(err, ErrExpansionChoices) {
		t.Fatal("inconsistent base selection did not fail", err)
	}
	must(working.Pop(1))
}
