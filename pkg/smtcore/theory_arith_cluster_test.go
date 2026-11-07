package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTNonlinearClusterInequalityRowsScopedProofs(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, tc := range []struct {
				secondBound int64
				strict      bool
			}{{1, false}, {-1, false}, {0, true}} {
				t.Run(fmt.Sprint(proofs, "/", enabled, "/", tc), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					if proofs {
						astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					}
					s, err := NewSMTSolver(m, nil, "QF_NRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, s.Close()) })
					s.kernel.preprocess = false
					family, err := m.ArithmeticFamilyID()
					astTestMust(t, err)
					sort, err := m.MkFamilySort(family, 0, nil)
					astTestMust(t, err)
					variables := make([]ast.ASTHandle, 4)
					for i := range variables {
						variables[i], err = m.MkConst(astTestSymbol(t, m, fmt.Sprint("cluster-", i)), sort)
						astTestMust(t, err)
					}
					x, y, z, unrelated := variables[0], variables[1], variables[2], variables[3]
					app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
						h, err := m.MkFamilyApp(family, op, nil, args)
						astTestMust(t, err)
						return h
					}
					numeral := func(value int64) ast.ASTHandle {
						h, err := m.ArithmeticNumeralRat(big.NewRat(value, 1), false)
						astTestMust(t, err)
						return h
					}
					products := []ast.ASTHandle{app(smtArithOpMul, x, y), app(smtArithOpMul, x, z)}
					sum := app(smtArithOpAdd, y, z)
					equality, err := m.MkEq(sum, numeral(0))
					astTestMust(t, err)
					op, firstBound := smtArithOpGE, int64(1)
					if tc.strict {
						op, firstBound = smtArithOpGT, 0
					}
					formulas := []ast.ASTHandle{
						app(op, products[0], numeral(firstBound)),
						app(op, products[1], numeral(tc.secondBound)),
						equality,
						app(smtArithOpGE, unrelated, numeral(0)),
					}
					for _, formula := range formulas {
						ref, err := m.NewRef(formula)
						astTestMust(t, err)
						t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
					}
					for repeat := 0; repeat < 2; repeat++ {
						astTestMust(t, s.Push())
						theory := s.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
						// This free definition can absorb any x and unrelated value.
						// Source traversal must skip it as a free substitution row.
						freeSum := app(smtArithOpAdd, x, unrelated)
						_, err := theory.InternalizeTerm(freeSum)
						astTestMust(t, err)
						config, err := s.kernel.smtParams.ArithmeticState()
						astTestMust(t, err)
						config.NLArithGB = enabled
						config.NLArithBranching = false
						astTestMust(t, theory.ConfigureArithmetic(config))
						for _, formula := range formulas {
							astTestMust(t, s.Assert(formula))
						}
						before := theory.statistics.grobner
						status, err := s.CheckSat(nil)
						astTestMust(t, err)
						if tc.secondBound >= 0 {
							want := lbool.Undef
							if enabled {
								want = lbool.False
							}
							if status != want {
								t.Fatal("coupled nonlinear inequalities", status, want)
							}
						} else if status == lbool.False {
							t.Fatal("satisfiable inequality counterpart rejected")
						}
						if enabled && theory.statistics.grobner == before {
							t.Fatal("tableau-row Grobner path was not exercised")
						}
						if status == lbool.False && proofs {
							proof, err := s.Proof()
							astTestMust(t, err)
							fact, err := m.GetProofFact(proof)
							astTestMust(t, err)
							if !m.IsFalse(fact) {
								t.Fatal("tableau-row nonlinear proof")
							}
						}
						solver, feasible, err := theory.buildSimplex()
						astTestMust(t, err)
						if feasible != lbool.True {
							t.Fatal("linear relaxation should remain feasible", feasible)
						}
						cluster, err := theory.nonlinearCluster(solver)
						astTestMust(t, err)
						found := make(map[uint32]bool)
						for _, column := range cluster.columns {
							found[column] = true
						}
						for _, variable := range []ast.ASTHandle{x, y, z} {
							column := theory.simplexVariables[theory.state.context.enodes.get(variable).TheoryVar(family)]
							if !found[column] {
								t.Fatal("connected factor missing from nonlinear cluster")
							}
						}
						column := theory.simplexVariables[theory.state.context.enodes.get(unrelated).TheoryVar(family)]
						if found[column] {
							t.Fatal("free substitution pulled an unrelated variable into the cluster")
						}
						if len(cluster.rows) == 0 {
							t.Fatal("nonlinear cluster contains no tableau rows")
						}
						astTestMust(t, s.Pop(1))
						status, err = s.CheckSat(nil)
						astTestMust(t, err)
						if status != lbool.True {
							t.Fatal("nonlinear tableau rows survived pop", status)
						}
					}
				})
			}
		}
	}
}
