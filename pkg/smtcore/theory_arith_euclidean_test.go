package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTEuclideanBridgeCongruenceScopesProofsAndModels(t *testing.T) {
	for _, sign := range []int64{-1, 1} {
		for _, large := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, proofs := range []bool{false, true} {
					for _, rhs := range []int64{1, 5} {
						t.Run(fmt.Sprint(sign, "/", large, "/", enabled, "/", proofs, "/", rhs), func(t *testing.T) {
							m, cfg := smtModelGeneratorFixture(t)
							if proofs {
								astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
							}
							s, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
							astTestMust(t, err)
							t.Cleanup(func() { astTestMust(t, s.Close()) })
							smtDisableAutoConfig(t, s)
							s.kernel.preprocess = false
							astTestMust(t, s.SetProduceModels(true))
							family, err := m.ArithmeticFamilyID()
							astTestMust(t, err)
							sort, err := m.MkFamilySort(family, 1, nil)
							astTestMust(t, err)
							x, err := m.MkConst(astTestSymbol(t, m, "euclid-x"), sort)
							astTestMust(t, err)
							y, err := m.MkConst(astTestSymbol(t, m, "euclid-y"), sort)
							astTestMust(t, err)
							num := func(v int64) ast.ASTHandle {
								h, err := m.ArithmeticNumeralRat(big.NewRat(v, 1), true)
								astTestMust(t, err)
								return h
							}
							app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
								h, err := m.MkFamilyApp(family, op, nil, args)
								astTestMust(t, err)
								return h
							}
							coefficient, right := big.NewInt(3), big.NewInt(rhs)
							if large {
								coefficient.Add(new(big.Int).Lsh(big.NewInt(1), 100), big.NewInt(1))
								if rhs == 5 {
									right.Add(coefficient, big.NewInt(2))
								}
							}
							right.Mul(right, big.NewInt(sign))
							coefficientExpr, err := m.ArithmeticNumeralRat(new(big.Rat).SetInt(coefficient), true)
							astTestMust(t, err)
							rightExpr, err := m.ArithmeticNumeralRat(new(big.Rat).SetInt(right), true)
							astTestMust(t, err)
							eq, err := m.MkEq(app(smtArithOpAdd, app(smtArithOpMul, coefficientExpr, x), app(smtArithOpMul, num(2), y)), rightExpr)
							astTestMust(t, err)
							lowerX, upperX := int64(0), int64(2)
							yDirection := smtArithOpGE
							if sign < 0 {
								lowerX, upperX = -2, 0
								yDirection = smtArithOpLE
							}
							formulas := []ast.ASTHandle{eq, app(smtArithOpGE, x, num(lowerX)), app(smtArithOpLE, x, num(upperX)), app(yDirection, y, num(0))}
							doubleCoefficient, err := m.ArithmeticNumeralRat(new(big.Rat).SetInt(new(big.Int).Mul(big.NewInt(2), coefficient)), true)
							astTestMust(t, err)
							doubled := app(smtArithOpAdd, app(smtArithOpMul, doubleCoefficient, x), app(smtArithOpMul, num(4), y))
							doubleValue := new(big.Int).Mul(big.NewInt(2), right)
							lowerDouble, upperDouble := new(big.Int), new(big.Int).Add(doubleValue, big.NewInt(10))
							if sign < 0 {
								lowerDouble.Sub(doubleValue, big.NewInt(10))
								upperDouble.SetInt64(0)
							}
							looseLower, err := m.ArithmeticNumeralRat(new(big.Rat).SetInt(lowerDouble), true)
							astTestMust(t, err)
							looseUpper, err := m.ArithmeticNumeralRat(new(big.Rat).SetInt(upperDouble), true)
							astTestMust(t, err)
							formulas = append(formulas, app(smtArithOpGE, doubled, looseLower), app(smtArithOpLE, doubled, looseUpper))
							for _, formula := range formulas {
								ref, err := m.NewRef(formula)
								astTestMust(t, err)
								t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
							}
							theory := s.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
							config, err := s.kernel.smtParams.ArithmeticState()
							astTestMust(t, err)
							config.EuclideanSolver = enabled
							astTestMust(t, theory.ConfigureArithmetic(config))
							numerals, err := theory.euclideanNumeralOwner()
							astTestMust(t, err)
							liveNumerals := numerals.Z.Cells.Stats().Live
							for repeat := 0; repeat < 2; repeat++ {
								astTestMust(t, s.Push())
								for _, formula := range formulas {
									astTestMust(t, s.Assert(formula))
								}
								beforeCalls, beforeBounds, beforeBranches := theory.statistics.euclideanCalls, theory.statistics.euclideanBounds, theory.statistics.branches
								status, err := s.CheckSat(nil)
								astTestMust(t, err)
								want := lbool.True
								if rhs == 1 {
									want = lbool.False
								}
								if status != want {
									t.Fatal("Euclidean bridge result", status, want)
								}
								if got := numerals.Z.Cells.Stats().Live; got != liveNumerals {
									t.Fatalf("Euclidean bridge leaked integer cells after check %d: got %d want %d", repeat, got, liveNumerals)
								}
								if enabled && (theory.statistics.euclideanCalls == beforeCalls || theory.statistics.euclideanBounds == beforeBounds || theory.statistics.branches != beforeBranches) {
									t.Fatal("Euclidean congruence did not solve without branching", theory.statistics)
								}
								if !enabled && theory.statistics.euclideanCalls != beforeCalls {
									t.Fatal("disabled Euclidean bridge was used")
								}
								if rhs == 1 && proofs {
									proof, err := s.Proof()
									astTestMust(t, err)
									fact, err := m.GetProofFact(proof)
									astTestMust(t, err)
									if !m.IsFalse(fact) {
										t.Fatal("Euclidean conflict proof")
									}
								}
								if rhs == 5 {
									if enabled {
										smtTestEuclideanBoundExplanations(t, theory)
										key := ""
										for _, atom := range theory.atoms {
											if atom.left == doubled {
												key = smtArithmeticCoefficientKey(&atom.form)
												break
											}
										}
										lower, upper := false, false
										loosePremises := make(map[int32]bool)
										for _, atom := range theory.atoms {
											if atom.left == doubled {
												loosePremises[NewSMTLiteral(atom.variable, false).Index()] = true
												loosePremises[NewSMTLiteral(atom.variable, true).Index()] = true
											}
										}
										for _, bound := range theory.expressionBounds {
											if bound.key == key && bound.value.Main.Cmp(new(big.Rat).SetInt(doubleValue)) == 0 {
												for _, dependency := range bound.explanation.dependencies {
													if loosePremises[dependency.Literal] {
														t.Fatal("zero-GCD bound retained an unnecessary old endpoint premise")
													}
												}
												if bound.lower {
													lower = true
												} else {
													upper = true
												}
											}
										}
										if key == "" || !lower || !upper {
											t.Fatal("zero-GCD normalization did not fix the comparison-only polynomial")
										}
									}
									model, err := s.AcquireModel()
									astTestMust(t, err)
									for _, term := range []ast.ASTHandle{x, y} {
										decl, err := m.GetDecl(term)
										astTestMust(t, err)
										value, err := model.ConstantInterpretation(decl)
										astTestMust(t, err)
										text, _, matched, err := m.ArithmeticNumeralText(value)
										astTestMust(t, err)
										point, ok := new(big.Rat).SetString(text)
										if !matched || !ok || point.Cmp(big.NewRat(sign, 1)) != 0 {
											t.Fatal("Euclidean model violates unique integer solution", text)
										}
									}
									astTestMust(t, model.DecRef())
									// New variables shift comparison-column indices. Retained
									// expression bounds must follow their logical polynomial.
									astTestMust(t, s.Push())
									z, err := m.MkConst(astTestSymbol(t, m, fmt.Sprint("euclid-shift-", repeat)), sort)
									astTestMust(t, err)
									astTestMust(t, s.Assert(app(smtArithOpGE, z, num(0))))
									status, err = s.CheckSat(nil)
									astTestMust(t, err)
									if status != lbool.True {
										t.Fatal("Euclidean replay bounds followed a stale column", status)
									}
									if enabled {
										smtTestEuclideanBoundExplanations(t, theory)
									}
									astTestMust(t, s.Pop(1))
								}
								astTestMust(t, s.Pop(1))
								if len(theory.expressionBounds) != 0 || len(theory.derivedBounds) != 0 {
									t.Fatal("Euclidean numeric bounds survived pop")
								}
								astTestMust(t, s.Push())
								for _, term := range []ast.ASTHandle{x, y} {
									zero, err := m.MkEq(term, num(0))
									astTestMust(t, err)
									astTestMust(t, s.Assert(zero))
								}
								status, err = s.CheckSat(nil)
								astTestMust(t, err)
								if status != lbool.True {
									t.Fatal("Euclidean residue survived scope pop", status)
								}
								astTestMust(t, s.Pop(1))
							}
						})
					}
				}
			}
		}
	}
}

func smtTestEuclideanBoundExplanations(t *testing.T, theory *SMTLinearArithmeticTheory) {
	t.Helper()
	var explanations []*smtArithmeticBoundExplanation
	for _, bound := range theory.derivedBounds {
		explanations = append(explanations, bound.explanation)
		if _, known := theory.solver.BoundDependencies(bound.column, bound.lower); !known {
			t.Fatal("original column lost replay premises")
		}
	}
	for _, bound := range theory.expressionBounds {
		explanations = append(explanations, bound.explanation)
		found := false
		for i, atom := range theory.atoms {
			if atom.supported && smtArithmeticCoefficientKey(&atom.form) == bound.key {
				if _, known := theory.solver.BoundDependencies(theory.nextSimplex+uint32(i), bound.lower); !known {
					t.Fatal("comparison column lost replay premises")
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatal("derived comparison polynomial disappeared")
		}
	}
	if len(explanations) == 0 {
		t.Fatal("no Euclidean derived bounds")
	}
	for _, explanation := range explanations {
		if explanation == nil || len(explanation.dependencies) == 0 {
			t.Fatal("Euclidean bound lost its premises")
		}
		seen := make(map[int32]bool)
		for _, dependency := range explanation.dependencies {
			if seen[dependency.Literal] || dependency.Coefficient.Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("plain derived bound retained duplicate or weighted premise", dependency)
			}
			seen[dependency.Literal] = true
		}
	}
}
