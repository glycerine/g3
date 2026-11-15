package opt

// Source: src/opt/sortmax.cpp unsigned-weight input expansion.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxSMTScaledTermsAndExpansionPreserveMultiplicity(t *testing.T) {
	fixture := newWeightedSoftFixture(t)
	refs := make([]ast.ASTRef, 2)
	var err error
	refs[0], err = fixture.manager.NewRef(fixture.p)
	if err == nil {
		refs[1], err = fixture.manager.NewRef(fixture.q)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.manager.DelRef(&refs[0])
	defer fixture.manager.DelRef(&refs[1])
	objective := &arithmeticObjective{softTerms: refs, softWeights: []*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}}
	terms, scale, err := maxSMTScaledTerms(fixture.manager, objective)
	if err != nil || scale.Int64() != 1 || terms[0].weight.Int64() != 2 || terms[1].weight.Int64() != 3 {
		t.Fatal(terms, scale, err)
	}
	expanded := maxSMTExpandTerms(terms, 5)
	if len(expanded) != 5 || expanded[0] != fixture.p || expanded[1] != fixture.p || expanded[2] != fixture.q {
		t.Fatal("expanded weighted terms", expanded)
	}
	// AUDIT.md opt_01-F1: a fractional weight is scaled, not refused;
	// sortmax rejects it after find_mutexes.
	objective.softWeights[0] = big.NewRat(3, 2)
	objective.softWeights[1] = big.NewRat(2, 3)
	terms, scale, err = maxSMTScaledTerms(fixture.manager, objective)
	if err != nil || scale.Int64() != 6 || terms[0].weight.Int64() != 9 || terms[1].weight.Int64() != 4 {
		t.Fatal("scaled fractional weights", terms, scale, err)
	}
	if _, unsigned := maxSMTUnscaleUnsigned(&terms[0].weight, scale); unsigned {
		t.Fatal("3/2 is not unsigned")
	}
	if w, unsigned := maxSMTUnscaleUnsigned(big.NewInt(12), scale); !unsigned || w != 2 {
		t.Fatal("12/6 is unsigned 2", w, unsigned)
	}
	if _, unsigned := maxSMTUnscaleUnsigned(new(big.Int).Lsh(big.NewInt(1), 32), big.NewInt(1)); unsigned {
		t.Fatal("2^32 is not unsigned")
	}
}
