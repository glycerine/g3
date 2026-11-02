package pdr_test

// Source: pdr_context.cpp pred_transformer::simplify_formulas.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestPredicateTransformerSimplifiesInvariantAndFiniteFrames(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 2)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "simplified-frames", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableHashCache)
	pdrMust(t, err)
	truth, err := manager.MkTrue()
	pdrMust(t, err)
	pdrMust(t, transformer.InitializeSystem(truth, truth))

	invariant := pdrBool(t, manager, "simplified-invariant")
	invariantExtra := pdrBool(t, manager, "simplified-invariant-extra")
	weakInvariant, err := manager.MkOr(invariant, invariantExtra)
	pdrMust(t, err)
	finite := pdrBool(t, manager, "simplified-finite")
	finiteExtra := pdrBool(t, manager, "simplified-finite-extra")
	weakFinite, err := manager.MkOr(finite, finiteExtra)
	pdrMust(t, err)
	for _, property := range []struct {
		formula ast.ASTHandle
		level   uint32
	}{
		{invariant, pdr.InfinityLevel},
		{weakInvariant, pdr.InfinityLevel},
		{finite, 1},
		{weakFinite, 1},
	} {
		changed, x := transformer.AddProperty(property.formula, property.level)
		pdrMust(t, x)
		if !changed {
			t.Fatal("new frame property was ignored", property)
		}
	}
	pdrMust(t, transformer.SimplifyFormulas())
	stats, err := transformer.Statistics()
	pdrMust(t, err)
	if stats.Properties != 2 {
		t.Fatal("unit subsumption did not prune both weak properties", stats)
	}
	for _, frame := range []struct {
		level int
		want  ast.ASTHandle
	}{{-1, invariant}, {1, finite}} {
		cover, x := transformer.CoverDelta(frame.level)
		pdrMust(t, x)
		handle, x := manager.RefGet(cover)
		pdrMust(t, x)
		if handle != frame.want {
			t.Fatal("simplified cover", frame.level, handle, frame.want)
		}
		pdrMust(t, manager.DelRef(&cover))
	}
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
