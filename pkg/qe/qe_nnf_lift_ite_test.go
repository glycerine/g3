package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestQENNFLiftsRelevantNestedITEAndCollectsConditionPolarities(t *testing.T) {
	f := newQENNFFixture(t)
	one := f.number("1")
	two := f.number("2")
	z := f.constant("qe-nnf-z", f.integer)
	choice := f.own(f.r.manager.MkIte(f.x, one, two))
	formula := f.own(f.r.manager.MkArithmeticLE(choice, z))
	ref := qeNNFRef(t, f, formula)
	atoms, err := f.normalizer.Normalize(ref)
	fmMust(t, err)
	defer func() { fmMust(t, atoms.Close()) }()
	result, err := f.r.manager.RefGet(ref)
	fmMust(t, err)
	assertQENNFConnectives(t, f.r.manager, result)
	positive, err := atoms.Positive(f.x)
	fmMust(t, err)
	negative, err := atoms.Negative(f.x)
	fmMust(t, err)
	if !positive || !negative {
		t.Fatal("lifted ITE condition polarities", positive, negative)
	}
}

func TestQENNFOwnershipCopyAndCloseGuards(t *testing.T) {
	f := newQENNFFixture(t)
	copyNormalizer := *f.normalizer
	formula := qeNNFRef(t, f, f.x)
	if _, err := copyNormalizer.Normalize(formula); err == nil {
		t.Fatal("copied NNF normalizer accepted")
	}
	copyAdapter := *f.adapter
	if _, err := copyAdapter.IsRelevant(f.x); err == nil {
		t.Fatal("copied NNF adapter accepted")
	}
	foreignContext, err := NewSolverContext(f.r.manager, []ast.ASTHandle{f.x})
	fmMust(t, err)
	adapter, err := NewSolverNNFAdapter(foreignContext)
	fmMust(t, err)
	fmMust(t, adapter.Close())
	fmMust(t, adapter.Close())
	if _, err = adapter.IsRelevant(f.x); err == nil {
		t.Fatal("closed NNF adapter accepted")
	}
	fmMust(t, foreignContext.Close())
}
