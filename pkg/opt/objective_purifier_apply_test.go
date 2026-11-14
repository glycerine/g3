package opt

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestObjectivePurifierDefinitionsAndLinearPassThrough(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	x := decompositionIntegerVariable(t, f, "purifier-x")
	twoX, err := f.manager.MkArithmeticMul(decompositionInteger(t, f, 2), x)
	if err != nil {
		t.Fatal(err)
	}
	linear, err := f.manager.MkArithmeticAdd(twoX, decompositionInteger(t, f, 3))
	if err != nil {
		t.Fatal(err)
	}
	nonlinear, err := f.manager.MkArithmeticMul(x, x)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name              string
		objective         ast.ASTHandle
		fresh, arithmetic uint32
	}{
		{"nonlinear", nonlinear, 1, 1},
		{"linear", linear, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			purifier, err := NewObjectivePurifier(f.manager)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := purifier.Close(); err != nil {
					t.Error(err)
				}
			}()
			set, err := purifier.Purify(nil, []DecomposedObjective{{SourceKind: FormulaMaximize, Term: test.objective, OriginalTerm: test.objective}})
			if err != nil {
				t.Fatal(err)
			}
			stats, err := purifier.Statistics()
			if err != nil || stats.FreshDeclarations != test.fresh || stats.ArithmeticDefinitions != test.arithmetic {
				t.Fatal(stats, err)
			}
			if len(set.Hard) != int(test.fresh) || len(set.Objectives) != 1 {
				t.Fatal(len(set.Hard), len(set.Objectives))
			}
			if test.fresh == 0 && set.Objectives[0].Term != test.objective {
				t.Fatal("linear objective was rebuilt")
			}
			if test.fresh != 0 && set.Objectives[0].Term == test.objective {
				t.Fatal("nonlinear objective was not purified")
			}
		})
	}
}

// AUDIT.md FUZZ_A-F6: is_arith_expr tests the head symbol's family, so an
// Int-sorted uninterpreted application is not purified.
func TestObjectivePurifierKeepsNonArithmeticHead(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	x := decompositionIntegerVariable(t, f, "purifier-uf-x")
	sort, err := f.manager.GetSort(x)
	if err != nil {
		t.Fatal(err)
	}
	name, err := f.symbols.Intern("purifier-uf-f")
	if err != nil {
		t.Fatal(err)
	}
	decl, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	application, err := f.manager.MkApp(decl, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := f.manager.NewRef(application)
	if err != nil {
		t.Fatal(err)
	}
	f.refs = append(f.refs, ref)
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := purifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	var hard []ast.ASTHandle
	term, err := purifier.arithmeticTerm(application, &hard)
	if err != nil {
		t.Fatal(err)
	}
	if term != application || len(hard) != 0 {
		t.Fatal("uninterpreted objective was purified", len(hard))
	}
}

func TestObjectivePurifierSoftDefinition(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	x := decompositionIntegerVariable(t, f, "purifier-soft-x")
	condition, err := f.manager.MkArithmeticGT(x, decompositionInteger(t, f, 0))
	if err != nil {
		t.Fatal(err)
	}
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := purifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	set, err := purifier.Purify(nil, []DecomposedObjective{{SourceKind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{condition}, Weights: []big.Rat{*big.NewRat(3, 1)}, ID: "pref"}})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := purifier.Statistics()
	if err != nil || stats.FreshDeclarations != 1 || stats.SoftDefinitions != 1 || len(set.Hard) != 1 || len(set.Objectives[0].SoftTerms) != 1 {
		t.Fatal(stats, set, err)
	}
}

func TestObjectivePurifierLeavesPropositionalSoftTermsAtomic(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	conjunction, err := f.manager.MkAnd(f.p, f.q)
	if err != nil {
		t.Fatal(err)
	}
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = purifier.Close() }()
	set, err := purifier.Purify(nil, []DecomposedObjective{{SourceKind: FormulaMaxSAT, SoftTerms: []ast.ASTHandle{conjunction}, Weights: []big.Rat{*big.NewRat(1, 1)}}})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := purifier.Statistics()
	if err != nil || stats.FreshDeclarations != 0 || len(set.Hard) != 0 || set.Objectives[0].SoftTerms[0] != conjunction {
		t.Fatal(stats, set, err)
	}
}
