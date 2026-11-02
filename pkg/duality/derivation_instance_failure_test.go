package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestDualityDerivationInstanceAllocationFailures(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("instance-allocation")
	must(err)
	closeExpr(t, term)
	original, err := NewRPFP(ctx)
	must(err)
	defer func() { must(original.Close()) }()
	parent, err := original.CreateNode(term)
	must(err)
	must(parent.Annotation.SetEmpty())
	h, err := term.Raw()
	must(err)
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			must(ctx.manager.DelRef(&pins[i]))
		}
	}()
	for i := 0; i < 4096; i++ {
		ctx.manager.Budget().FailAfter(0)
		pin, e := ctx.manager.NewRef(h)
		ctx.manager.Budget().FailAfter(-1)
		if errors.Is(e, arena.ErrInjectedFailure) {
			break
		}
		must(e)
		pins = append(pins, pin)
		if i == 4095 {
			t.Fatal("reference allocation frontier not reached")
		}
	}
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 128 && !complete; ordinal++ {
		working, err := NewRPFPWithVocabulary(original.vocabulary)
		must(err)
		d, err := NewDerivationTree(original, working)
		must(err)
		ctx.manager.Budget().FailAfter(ordinal)
		instance, failure := d.CreateApproximatedInstance(parent)
		ctx.manager.Budget().FailAfter(-1)
		if failure != nil {
			failures++
			if !errors.Is(failure, arena.ErrInjectedFailure) || instance != nil {
				t.Fatal("invalid partial instance publication", ordinal, failure, instance)
			}
		} else {
			complete = true
			if instance.Map != parent || len(d.leaves) != 1 || d.leaves[0] != instance {
				t.Fatal("instance publication", ordinal)
			}
			empty, err := instance.Annotation.IsEmpty()
			must(err)
			if !empty {
				t.Fatal("instance lost original annotation")
			}
		}
		must(working.Close())
		empty, err := parent.Annotation.IsEmpty()
		must(err)
		if !empty {
			t.Fatal("failed construction modified the original annotation", ordinal)
		}
	}
	if !complete || failures == 0 {
		t.Fatal("allocation campaign incomplete", complete, failures)
	}
}
