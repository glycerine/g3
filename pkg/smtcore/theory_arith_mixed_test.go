package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTMixedArithmeticScopeModel(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) { smtMixedArithmeticScopeModel(t, proofs) })
	}
}

func smtMixedArithmeticScopeModel(t *testing.T, proofs bool) {
	m, cfg := smtModelGeneratorFixture(t)
	if proofs {
		astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	}
	solver, err := NewSMTSolver(m, nil, "AUFLIRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, solver.SetProduceModels(true))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	i, err := m.MkConst(astTestSymbol(t, m, "mixed-i"), integer)
	astTestMust(t, err)
	r, err := m.MkConst(astTestSymbol(t, m, "mixed-r"), real)
	astTestMust(t, err)
	half, err := m.ArithmeticRealNumeralText("1/2")
	astTestMust(t, err)
	cast, err := m.MkFamilyApp(family, smtArithOpToReal, nil, []ast.ASTHandle{i})
	astTestMust(t, err)
	product, err := m.MkArithmeticMul(half, cast)
	astTestMust(t, err)
	base, err := m.MkEq(r, product)
	astTestMust(t, err)
	for _, h := range []ast.ASTHandle{base, i, r, half} {
		ref, err := m.NewRef(h)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
	}
	astTestMust(t, solver.Assert(base))
	astTestMust(t, solver.Push())
	zero, err := m.ArithmeticRealNumeralText("0")
	astTestMust(t, err)
	for _, spec := range []struct {
		op    int32
		value ast.ASTHandle
	}{{smtArithOpGT, zero}, {smtArithOpLT, half}} {
		a, err := m.MkFamilyApp(family, spec.op, nil, []ast.ASTHandle{r, spec.value})
		astTestMust(t, err)
		astTestMust(t, solver.Assert(a))
	}
	status, err := solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal(status)
	}
	if proofs {
		proof, err := solver.Proof()
		astTestMust(t, err)
		fact, err := m.GetProofFact(proof)
		astTestMust(t, err)
		if !m.IsFalse(fact) {
			t.Fatal("mixed arithmetic conflict proof")
		}
	}
	astTestMust(t, solver.Pop(1))
	final, err := m.MkEq(r, half)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(final))
	status, err = solver.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	model, err := solver.AcquireModel()
	astTestMust(t, err)
	defer func() { astTestMust(t, model.DecRef()) }()
	d, err := m.GetDecl(i)
	astTestMust(t, err)
	value, err := model.ConstantInterpretation(d)
	astTestMust(t, err)
	text, isInt, matched, err := m.ArithmeticNumeralText(value)
	astTestMust(t, err)
	if !matched || !isInt || text != "1" {
		t.Fatal("integer model value", text, isInt, matched)
	}
	d, err = m.GetDecl(r)
	astTestMust(t, err)
	value, err = model.ConstantInterpretation(d)
	astTestMust(t, err)
	text, isInt, matched, err = m.ArithmeticNumeralText(value)
	astTestMust(t, err)
	if !matched || isInt || text != "1/2" {
		t.Fatal("real model value", text, isInt, matched)
	}
	state := solver.kernel.internalizer
	if state.context.enodes.get(i).Root() == state.context.enodes.get(cast).Root() {
		t.Fatal("Int and Real enodes were merged")
	}
}

func TestSMTMixedArithmeticCastsScopedProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, negated := range []bool{false, true} {
			t.Run(fmt.Sprint(proofs, "/", negated), func(t *testing.T) {
				m, cfg := smtModelGeneratorFixture(t)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				solver, err := NewSMTSolver(m, nil, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, solver.Close()) })
				family, err := m.ArithmeticFamilyID()
				astTestMust(t, err)
				real, err := m.MkFamilySort(family, 0, nil)
				astTestMust(t, err)
				r, err := m.MkConst(astTestSymbol(t, m, "cast-proof-r"), real)
				astTestMust(t, err)
				predicate, err := m.MkFamilyApp(family, 16, nil, []ast.ASTHandle{r})
				astTestMust(t, err)
				if negated {
					predicate, err = m.MkNot(predicate)
					astTestMust(t, err)
				}
				zero, err := m.ArithmeticRealNumeralText("0")
				astTestMust(t, err)
				limit := zero
				lowerKind, upperKind := smtArithOpGE, smtArithOpLE
				if !negated {
					limit, err = m.ArithmeticRealNumeralText("1")
					astTestMust(t, err)
					lowerKind, upperKind = smtArithOpGT, smtArithOpLT
				}
				lower, err := m.MkFamilyApp(family, lowerKind, nil, []ast.ASTHandle{r, zero})
				astTestMust(t, err)
				upper, err := m.MkFamilyApp(family, upperKind, nil, []ast.ASTHandle{r, limit})
				astTestMust(t, err)
				for _, h := range []ast.ASTHandle{predicate, lower, upper} {
					ref, err := m.NewRef(h)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
				}
				for repeat := 0; repeat < 2; repeat++ {
					astTestMust(t, solver.Push())
					for _, formula := range []ast.ASTHandle{predicate, lower, upper} {
						astTestMust(t, solver.Assert(formula))
					}
					status, err := solver.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("cast contradiction", status)
					}
					if proofs {
						proof, err := solver.Proof()
						astTestMust(t, err)
						fact, err := m.GetProofFact(proof)
						astTestMust(t, err)
						if !m.IsFalse(fact) {
							t.Fatal("cast contradiction proof")
						}
					}
					astTestMust(t, solver.Pop(1))
					theory := solver.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
					if len(theory.castTerms) != 0 || len(theory.castPredicates) != 0 {
						t.Fatal("cast axiom scope was not restored")
					}
					status, err = solver.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("cast scope retained a conflict", status)
					}
				}
			})
		}
	}
}
