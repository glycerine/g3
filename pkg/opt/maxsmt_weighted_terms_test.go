package opt

// Source: src/opt/sortmax.cpp unsigned-weight input expansion.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestMaxSMTIntegralTermsAndExpansionPreserveMultiplicity(t *testing.T) {
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
	terms, total, usable, err := maxSMTIntegralTerms(fixture.manager, objective)
	if err != nil || !usable || total != 5 {
		t.Fatal(terms, total, usable, err)
	}
	expanded := maxSMTExpandTerms(terms, total)
	if len(expanded) != 5 || expanded[0] != fixture.p || expanded[1] != fixture.p || expanded[2] != fixture.q {
		t.Fatal("expanded weighted terms", expanded)
	}
	objective.softWeights[0] = big.NewRat(1, 2)
	if _, _, usable, err = maxSMTIntegralTerms(fixture.manager, objective); err != nil || usable {
		t.Fatal("fractional SortMax input", usable, err)
	}
}
