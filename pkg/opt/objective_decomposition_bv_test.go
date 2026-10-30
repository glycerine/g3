package opt

import "testing"

func TestObjectiveDecompositionConvertsBitVectorObjectives(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	if _, err := f.manager.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.manager.RegisterBitVectorPlugin(); err != nil {
		t.Fatal(err)
	}
	prototype, err := f.manager.BitVectorNumeralUnsigned(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	sort, err := f.manager.GetSort(prototype)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.manager.MkFreshConst("decomposition-bv", sort)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.manager.NewRef(value)
	if err != nil {
		t.Fatal(err)
	}
	f.refs = append(f.refs, ref)
	decomposer, err := NewObjectiveDecomposer(f.manager, nil)
	if err != nil {
		t.Fatal(err)
	}
	decomposed, stats, err := decomposer.Decompose(DecodedFormulaSet{Objectives: []DecodedFormulaObjective{
		{Kind: FormulaMaximize, Term: value, OriginalTerm: value},
		{Kind: FormulaMinimize, Term: value, OriginalTerm: value},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(decomposed) != 2 || stats.ConvertedBVTerms != 2 {
		t.Fatal(decomposed, stats)
	}
	for i := range decomposed {
		if !decomposed[i].Converted || len(decomposed[i].SoftTerms) != 3 || len(decomposed[i].Weights) != 3 || decomposed[i].Weights[0].RatString() != "1" || decomposed[i].Weights[1].RatString() != "2" || decomposed[i].Weights[2].RatString() != "4" {
			t.Fatal(decomposed[i])
		}
	}
	if !decomposed[0].Negate || decomposed[0].Offset.RatString() != "7" || decomposed[1].Negate || decomposed[1].Offset.Sign() != 0 {
		t.Fatal(decomposed)
	}
	if err = decomposer.Close(); err != nil {
		t.Fatal(err)
	}
}
