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

func TestObjectiveDecompositionWrappedCoefficientConditionalValues(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	m := f.manager
	if _, err := m.RegisterArithmeticPlugin(); err != nil {
		t.Fatal(err)
	}
	keep := func(h ast.ASTHandle, err error) ast.ASTHandle {
		if err != nil {
			t.Fatal(err)
		}
		ref, err := m.NewRef(h)
		if err != nil {
			t.Fatal(err)
		}
		f.refs = append(f.refs, ref)
		return h
	}
	three := keep(m.ArithmeticIntegerNumeralText("3"))
	negative := keep(m.MkArithmeticNeg(three))
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		t.Fatal(err)
	}
	toReal := keep(m.MkFamilyApp(family, 14, nil, []ast.ASTHandle{negative}))
	coefficient := keep(m.MkArithmeticNeg(toReal))
	half := keep(m.ArithmeticRealNumeralText("1/2"))
	minusTwoThirds := keep(m.ArithmeticRealNumeralText("-2/3"))
	conditional := keep(m.MkIte(f.p, half, minusTwoThirds))
	term := keep(m.MkArithmeticMul(coefficient, conditional))
	for _, kind := range []FormulaObjectiveKind{FormulaMinimize, FormulaMaximize} {
		d, err := NewObjectiveDecomposer(m, nil)
		if err != nil {
			t.Fatal(err)
		}
		results, stats, err := d.Decompose(DecodedFormulaSet{Objectives: []DecodedFormulaObjective{{Kind: kind, Term: term, OriginalTerm: term}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || stats.ConvertedPBSums != 1 || !results[0].Converted {
			t.Fatalf("wrapped coefficient not converted: %+v %+v", results, stats)
		}
		objective := results[0]
		for _, pv := range []bool{false, true} {
			var eval func(ast.ASTHandle) bool
			eval = func(h ast.ASTHandle) bool {
				if h == f.p {
					return pv
				}
				if m.IsTrue(h) {
					return true
				}
				if m.IsFalse(h) {
					return false
				}
				not, arg, err := m.IsNot(h)
				if err != nil {
					t.Fatal(err)
				}
				if not {
					return !eval(arg)
				}
				t.Fatal("unexpected soft term", h)
				return false
			}
			penalty := new(big.Rat)
			for i, h := range objective.SoftTerms {
				if objective.Weights[i].Sign() <= 0 {
					t.Fatal("nonpositive soft weight")
				}
				if !eval(h) {
					penalty.Add(penalty, &objective.Weights[i])
				}
			}
			value, err := objective.RestoreValue(penalty)
			if err != nil {
				t.Fatal(err)
			}
			want := big.NewRat(-2, 1)
			if pv {
				want = big.NewRat(3, 2)
			}
			if value.Cmp(want) != 0 {
				t.Fatalf("kind=%v p=%v value=%s want=%s", kind, pv, value, want)
			}
		}
		if err = d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
