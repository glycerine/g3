package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

func TestPredicateFormulasRefRewritesAndScopesFrameConjunction(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	head := pdrFunction(t, manager, "frame_formula_rewriter", nil, boolean)
	transformer, err := pdr.NewPredicateTransformer(owner, head, pdr.ReachableNoCache)
	pdrMust(t, err)
	p := pdrBool(t, manager, "frame_formula_p")
	_, err = transformer.AddProperty(p, 2)
	pdrMust(t, err)
	_, err = transformer.AddProperty(p, pdr.InfinityLevel)
	pdrMust(t, err)
	before := manager.Stats().LiveReferences
	for i := 0; i < 3; i++ {
		ref, x := transformer.FormulasRef(1, false)
		pdrMust(t, x)
		h, x := manager.RefGet(ref)
		pdrMust(t, x)
		if h != p {
			t.Fatal("duplicate frame property did not simplify", h, p)
		}
		pdrMust(t, manager.DelRef(&ref))
		if got := manager.Stats().LiveReferences; got != before {
			t.Fatalf("frame refs after call %d: got %d want %d", i, got, before)
		}
	}
	// The compatibility borrow survives an independent scoped call.
	borrowed, err := transformer.Formulas(1, false)
	pdrMust(t, err)
	ref, err := transformer.FormulasRef(1, false)
	pdrMust(t, err)
	pdrMust(t, manager.DelRef(&ref))
	kind, err := manager.GetKind(borrowed)
	pdrMust(t, err)
	if kind != ast.ASTApp || borrowed != p {
		t.Fatal("invalid borrowed frame result", borrowed)
	}
	notP, err := manager.MkNot(p)
	pdrMust(t, err)
	_, err = transformer.AddProperty(notP, pdr.InfinityLevel)
	pdrMust(t, err)
	ref, err = transformer.FormulasRef(1, false)
	pdrMust(t, err)
	h, err := manager.RefGet(ref)
	pdrMust(t, err)
	if !manager.IsFalse(h) {
		t.Fatal("contradictory frame did not simplify to false", h)
	}
	pdrMust(t, manager.DelRef(&ref))
	pdrMust(t, transformer.Close())
	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
