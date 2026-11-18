package opt

import (
	"github.com/glycerine/g3/pkg/util/numeral"
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
	weights := []numeral.Rational{testRat(2, 1), testRat(-3, 1), testRat(4, 1), testRat(5, 1), testRat(0, 1)}
	marker, err := codec.EncodeObjective(FormulaObjective{Kind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{f.p, f.q, truth, falsehood, f.p}, Weights: weights, ID: "soft"})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := codec.Decode([]ast.ASTHandle{marker})
	if err != nil || len(decoded.Objectives) != 1 {
		t.Fatal(decoded, err)
	}
	soft := decoded.Objectives[0]
	if ratString(&soft.Offset) != "2" || len(soft.SoftTerms) != 2 || len(soft.Weights) != 2 || ratString(&soft.Weights[0]) != "2" || ratString(&soft.Weights[1]) != "3" {
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
