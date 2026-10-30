package opt

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func decompositionIntegerVariable(t *testing.T, f *objectiveFormulaFixture, name string) ast.ASTHandle {
	t.Helper()
	arith, err := f.manager.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := f.manager.MkFamilySort(arith, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := f.symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.manager.NewRef(value)
	if err != nil {
		t.Fatal(err)
	}
	f.refs = append(f.refs, ref)
	return value
}

func decompositionInteger(t *testing.T, f *objectiveFormulaFixture, value int64) ast.ASTHandle {
	t.Helper()
	result, err := f.manager.ArithmeticNumeralRat(big.NewRat(value, 1), true)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestObjectiveDecompositionConvertsPseudoBooleanSum(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	x := decompositionIntegerVariable(t, f, "decomposition-x")
	twoX, err := f.manager.MkArithmeticMul(decompositionInteger(t, f, 2), x)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := f.manager.MkArithmeticAdd(twoX, decompositionInteger(t, f, 3))
	if err != nil {
		t.Fatal(err)
	}
	decomposer, err := NewObjectiveDecomposer(f.manager, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	decomposed, stats, err := decomposer.Decompose(DecodedFormulaSet{Objectives: []DecodedFormulaObjective{{Kind: FormulaMaximize, Term: sum, OriginalTerm: sum}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(decomposed) != 1 || !decomposed[0].Converted || !decomposed[0].Negate || decomposed[0].Offset.RatString() != "5" || len(decomposed[0].SoftTerms) != 1 || decomposed[0].Weights[0].RatString() != "2" {
		t.Fatal(decomposed)
	}
	if stats.Objectives != 1 || stats.ConvertedPBSums != 1 || stats.GeneratedTerms == 0 {
		t.Fatal(stats)
	}
	value, err := decomposed[0].RestoreValue(big.NewRat(2, 1))
	if err != nil || value.RatString() != "3" {
		t.Fatal(value, err)
	}
	if err = decomposer.Close(); err != nil {
		t.Fatal(err)
	}
}
