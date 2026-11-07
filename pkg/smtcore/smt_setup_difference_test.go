package smtcore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTIDLSetupFirstBoundaryAndSimplex(t *testing.T) {
	for _, push := range []bool{false, true} {
		for _, automatic := range []bool{false, true} {
			t.Run(fmt.Sprint(push, "/", automatic), func(t *testing.T) {
				m, cfg := smtModelGeneratorFixture(t)
				solver, err := NewSMTSolver(m, nil, "QF_IDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, solver.Close()) })
				if !automatic {
					smtDisableAutoConfig(t, solver)
				}
				family, err := m.RegisterArithmeticPlugin()
				astTestMust(t, err)
				one, err := m.ArithmeticIntegerNumeralText("1")
				astTestMust(t, err)
				sort, err := m.GetSort(one)
				astTestMust(t, err)
				x, err := m.MkConst(astTestSymbol(t, m, "idl-setup-x"), sort)
				astTestMust(t, err)
				y, err := m.MkConst(astTestSymbol(t, m, "idl-setup-y"), sort)
				astTestMust(t, err)
				fixture := smtIntegerDiffFixture{manager: m, family: family, sort: sort, x: x, y: y}
				bound := fixture.bound(t, smtArithOpLE, x, y, 1)
				astTestMust(t, solver.Assert(bound))
				k := solver.kernel
				check := func() {
					t.Helper()
					if !automatic {
						if !k.relevancy.enabled || !k.preprocessState.NNFCNF || k.arithState.Eq2Ineq {
							t.Fatal("disabled automatic setup applied the IDL profile")
						}
						return
					}
					theory, ok := smtArithFactsOf(k.internalizer.theories[family])
					if !ok || !theory.integer || theory.propagateEqualities || k.arithState != k.smtParams.arith.state || !k.arithState.Eq2Ineq || k.arithState.Reflect || k.arithState.SmallLemmaSize != 30 || k.relevancy.enabled || k.preprocessState.NNFCNF {
						t.Fatal("IDL profile did not reach integer simplex and parameter owners")
					}
					if push {
						if k.smtState.PhaseSelection != 3 || k.smtState.RandomInitialActivity != 1 {
							t.Fatal("first push applied static IDL tuning")
						}
					} else if k.smtState.PhaseSelection != 4 || k.smtState.RandomInitialActivity != 2 || k.internalizer.randomInitialActivity != 2 {
						t.Fatal("static unit IDL tuning did not reach Boolean initialization")
					}
				}
				if push {
					astTestMust(t, solver.Push())
					check()
				}
				status, err := smtSetupProfileCheck(solver)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("IDL feasible difference", status)
				}
				check()
				astTestMust(t, solver.Push())
				astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, y, x, -2)))
				status, err = smtSetupProfileCheck(solver)
				astTestMust(t, err)
				if status != lbool.False {
					t.Fatal("IDL contradictory differences", status)
				}
				astTestMust(t, solver.Pop(1))
				status, err = smtSetupProfileCheck(solver)
				astTestMust(t, err)
				if status != lbool.True {
					t.Fatal("IDL pop did not restore SAT", status)
				}
				check()
				if push {
					astTestMust(t, solver.Pop(1))
				}
			})
		}
	}
}

func TestSMTIDLStaticSetupRejectsWrongFeatures(t *testing.T) {
	for _, kind := range []string{"non-difference", "real", "uf"} {
		t.Run(kind, func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			solver, err := NewSMTSolver(m, nil, "QF_IDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, solver.Close()) })
			one, err := m.ArithmeticIntegerNumeralText("1")
			astTestMust(t, err)
			if kind == "real" {
				one, err = m.ArithmeticRealNumeralText("1")
				astTestMust(t, err)
			}
			sort, err := m.GetSort(one)
			astTestMust(t, err)
			x, err := m.MkConst(astTestSymbol(t, m, "idl-invalid-x"), sort)
			astTestMust(t, err)
			term := x
			family, err := m.ArithmeticFamilyID()
			astTestMust(t, err)
			want := "has real variables"
			if kind == "non-difference" {
				y, err := m.MkConst(astTestSymbol(t, m, "idl-invalid-y"), sort)
				astTestMust(t, err)
				term, err = m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{x, y})
				astTestMust(t, err)
				want = "not in QF_IDL"
			} else if kind == "uf" {
				f, err := m.MkFuncDecl(astTestSymbol(t, m, "idl-invalid-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				term, err = m.MkApp(f, []ast.ASTHandle{x})
				astTestMust(t, err)
				want = "uninterpreted function symbols"
			}
			bound, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{term, one})
			astTestMust(t, err)
			astTestMust(t, solver.Assert(bound))
			_, err = smtSetupProfileCheck(solver)
			if err == nil || !strings.Contains(err.Error(), want) || solver.kernel.preprocessingSetupDone {
				t.Fatal("IDL accepted invalid static features", err, want)
			}
		})
	}
}

func TestSMTIDLDenseFeaturePolicyAndProofRouting(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, push := range []bool{false, true} {
			for _, simplex := range []bool{false, true} {
				t.Run(fmt.Sprint(proofs, "/", push, "/", simplex), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					if proofs {
						astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					}
					solver, err := NewSMTSolver(m, nil, "QF_IDL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, solver.Close()) })
					k := solver.kernel
					k.smtParams.arith.state.AutoConfigSimplex = simplex
					k.arithState.AutoConfigSimplex = simplex
					family, err := m.RegisterArithmeticPlugin()
					astTestMust(t, err)
					integer, err := m.MkFamilySort(family, 1, nil)
					astTestMust(t, err)
					x, err := m.MkConst(astTestSymbol(t, m, "idl-dense-x"), integer)
					astTestMust(t, err)
					y, err := m.MkConst(astTestSymbol(t, m, "idl-dense-y"), integer)
					astTestMust(t, err)
					fixture := smtIntegerDiffFixture{manager: m, family: family, sort: integer, x: x, y: y}
					for bound := int64(0); bound < 24; bound++ {
						astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, x, y, bound)))
					}
					if push {
						astTestMust(t, solver.Push())
					}
					status, err := smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("dense IDL conjunction", status)
					}
					phase := uint32(3)
					if !push && (proofs || simplex) {
						phase = 2
					}
					if k.smtState.PhaseSelection != phase || (!push && (k.smtState.RestartStrategy != 0 || k.smtState.RestartAdaptive || k.smtState.RandomInitialActivity != 2)) {
						t.Fatal("dense feature policy did not match proof/simplex/first-push routing", k.smtState)
					}
					theory, ok := smtArithFactsOf(k.internalizer.theories[family])
					_, dense := k.internalizer.theories[family].(*SMTDenseDiffTheory)
					wantDense := !push && !proofs && !simplex
					if dense != wantDense || !wantDense && (!ok || theory.mixed != (proofs && !push) || theory.integer == (proofs && !push)) {
						t.Fatal("proof checking and first push selected the wrong integrality mode")
					}
					astTestMust(t, solver.Push())
					astTestMust(t, solver.Assert(fixture.bound(t, smtArithOpLE, y, x, -1)))
					status, err = smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("dense IDL child conflict", status)
					}
					astTestMust(t, solver.Pop(1))
					status, err = smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("dense IDL pop recovery", status)
					}
					if push {
						astTestMust(t, solver.Pop(1))
					}
				})
			}
		}
	}
}
