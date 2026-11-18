package opt

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectiveDecompositionRetainsMaxSATAndCopiesValues(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	source := DecodedFormulaObjective{
		Kind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{f.p, f.q},
		Weights: []numeral.Rational{testRat(2, 1), testRat(3, 2)},
		Offset:  testRat(-1, 3), Negate: true, ID: "pref",
	}
	decomposer, err := NewObjectiveDecomposer(f.manager, nil)
	if err != nil {
		t.Fatal(err)
	}
	decomposed, stats, err := decomposer.Decompose(DecodedFormulaSet{Objectives: []DecodedFormulaObjective{source}})
	if err != nil {
		t.Fatal(err)
	}
	source.Weights[0], source.Offset = testRat(99, 1), testRat(88, 1)
	if len(decomposed) != 1 || stats.RetainedMaxSAT != 1 || !decomposed[0].Converted || decomposed[0].ID != "pref" || !decomposed[0].Negate || ratString(&decomposed[0].Offset) != "-1/3" || ratString(&decomposed[0].Weights[0]) != "2" || ratString(&decomposed[0].Weights[1]) != "3/2" {
		t.Fatal(decomposed, stats)
	}
	if err = decomposer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestObjectiveDecompositionLeavesGeneralArithmeticExplicit(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	x := decompositionIntegerVariable(t, f, "decomposition-general")
	square, err := f.manager.MkArithmeticMul(x, x)
	if err != nil {
		t.Fatal(err)
	}
	decomposer, err := NewObjectiveDecomposer(f.manager, nil)
	if err != nil {
		t.Fatal(err)
	}
	decomposed, stats, err := decomposer.Decompose(DecodedFormulaSet{Objectives: []DecodedFormulaObjective{{Kind: FormulaMinimize, Term: square, OriginalTerm: square}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(decomposed) != 1 || decomposed[0].Converted || decomposed[0].Term != square || stats.Unconverted != 1 {
		t.Fatal(decomposed, stats)
	}
	if err = decomposer.Close(); err != nil {
		t.Fatal(err)
	}
}
