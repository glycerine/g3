package opt

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectiveFormulaCodecNormalizesSoftConstantsAndNegativeWeights(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	codec, err := NewObjectiveFormulaCodec(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := codec.Close(); err != nil {
			t.Error(err)
		}
	}()
	truth, err := f.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsehood, err := f.manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	weights := []*big.Rat{big.NewRat(2, 1), big.NewRat(-3, 1), big.NewRat(4, 1), big.NewRat(5, 1), big.NewRat(0, 1)}
	marker, err := codec.EncodeObjective(FormulaObjective{Kind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{f.p, f.q, truth, falsehood, f.p}, Weights: weights, ID: "soft"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.Decode([]ast.ASTHandle{marker})
	if err != nil || len(decoded.Objectives) != 1 {
		t.Fatal(decoded, err)
	}
	soft := decoded.Objectives[0]
	if soft.Offset.RatString() != "2" || len(soft.SoftTerms) != 2 || len(soft.Weights) != 2 || soft.Weights[0].RatString() != "2" || soft.Weights[1].RatString() != "3" {
		t.Fatal(soft)
	}
	notQ, err := f.manager.MkNot(f.q)
	if err != nil {
		t.Fatal(err)
	}
	if soft.SoftTerms[0] != f.p || soft.SoftTerms[1] != notQ {
		t.Fatal(soft.SoftTerms)
	}
}
