package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTSharedArithmeticFixedRowsScopedProofs(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, proofs := range []bool{false, true} {
			for _, text := range []string{"0", "3", "-2", "3/2"} {
				point, _ := new(big.Rat).SetString(text)
				if integer && !point.IsInt() {
					continue
				}
				t.Run(fmt.Sprint(integer, "/", proofs, "/", text), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					if proofs {
						astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
					}
					logic, kind := "QF_UFLRA", int32(0)
					if integer {
						logic, kind = "QF_UFLIA", 1
					}
					s, err := NewSMTSolver(m, nil, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					// Explicit legacy algorithm target; QF real automatic setup now uses LAR.
					if logic == "QF_LRA" || logic == "QF_UFLRA" {
						smtDisableAutoConfig(t, s)
					}
					t.Cleanup(func() { astTestMust(t, s.Close()) })
					s.kernel.preprocess = false
					family, err := m.ArithmeticFamilyID()
					astTestMust(t, err)
					sort, err := m.MkFamilySort(family, kind, nil)
					astTestMust(t, err)
					x, err := m.MkConst(astTestSymbol(t, m, "fixed-shared-x"), sort)
					astTestMust(t, err)
					y, err := m.MkConst(astTestSymbol(t, m, "fixed-shared-y"), sort)
					astTestMust(t, err)
					unrelated, err := m.MkConst(astTestSymbol(t, m, "fixed-shared-unrelated"), sort)
					astTestMust(t, err)
					f, err := m.MkFuncDecl(astTestSymbol(t, m, "fixed-shared-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
					astTestMust(t, err)
					fx, err := m.MkApp(f, []ast.ASTHandle{x})
					astTestMust(t, err)
					fy, err := m.MkApp(f, []ast.ASTHandle{y})
					astTestMust(t, err)
					equality, err := m.MkEq(fx, fy)
					astTestMust(t, err)
					different, err := m.MkNot(equality)
					astTestMust(t, err)
					value, err := m.ArithmeticNumeralRat(point, integer)
					astTestMust(t, err)
					app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
						h, err := m.MkFamilyApp(family, op, nil, args)
						astTestMust(t, err)
						return h
					}
					irrelevantBound := app(smtArithOpGE, unrelated, value)
					astTestMust(t, s.Assert(irrelevantBound))
					firstLower := app(smtArithOpGE, x, value)
					secondLower := app(smtArithOpGE, y, value)
					for _, formula := range []ast.ASTHandle{different, firstLower, secondLower} {
						astTestMust(t, s.Assert(formula))
					}
					sum := app(smtArithOpAdd, x, y)
					twice, err := m.ArithmeticNumeralRat(new(big.Rat).Mul(point, big.NewRat(2, 1)), integer)
					astTestMust(t, err)
					fixed := app(smtArithOpLE, sum, twice)
					ref, err := m.NewRef(fixed)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
					for repeat := 0; repeat < 2; repeat++ {
						astTestMust(t, s.Push())
						astTestMust(t, s.Assert(fixed))
						status, err := s.CheckSat(nil)
						astTestMust(t, err)
						if status != lbool.False {
							t.Fatal("arithmetic-fixed UF inputs did not merge", status)
						}
						if proofs {
							proof, err := s.Proof()
							astTestMust(t, err)
							fact, err := m.GetProofFact(proof)
							astTestMust(t, err)
							if !m.IsFalse(fact) {
								t.Fatal("fixed-row UF proof")
							}
							// As in z3 4.7.1: the merge of f(x) and f(y) comes from the
							// adapter's x = y split (triangle-eq), refuted by Farkas
							// lemmas over the row and the endpoint bounds.
							if smtProofTheoryRuleNode(t, m, proof, "triangle-eq").IsNull() {
								t.Fatal("fixed-row proof lost the adapter triangle-eq axiom")
							}
							lemma := smtProofTheoryRuleNode(t, m, proof, "farkas")
							if lemma.IsNull() {
								t.Fatal("fixed-row proof lost its farkas lemma")
							}
							if smtTestArithmeticExpressionContains(t, m, lemma, unrelated) {
								t.Fatal("fixed-row lemma cites unrelated arithmetic assertion")
							}
						}
						astTestMust(t, s.Pop(1))
						astTestMust(t, s.Push())
						next, err := m.ArithmeticNumeralRat(new(big.Rat).Add(point, big.NewRat(1, 1)), integer)
						astTestMust(t, err)
						for _, pair := range [][2]ast.ASTHandle{{x, value}, {y, next}} {
							equality, err := m.MkEq(pair[0], pair[1])
							astTestMust(t, err)
							astTestMust(t, s.Assert(equality))
						}
						status, err = s.CheckSat(nil)
						astTestMust(t, err)
						if status != lbool.True {
							t.Fatal("fixed-row equality survived pop", status)
						}
						astTestMust(t, s.Pop(1))
					}
				})
			}
		}
	}
}
