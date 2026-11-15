package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTBitVectorLogicSetupProfiles(t *testing.T) {
	for _, logic := range []string{"QF_BV", "QF_BVRE", "QF_FP", "QF_FPBV", "QF_BVFP", "QF_AUFBV", "QF_ABV", "QF_UFBV"} {
		for _, push := range []bool{false, true} {
			for _, automatic := range []bool{false, true} {
				t.Run(fmt.Sprint(logic, "/", push, "/", automatic), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					solver, err := NewSMTSolver(m, nil, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, solver.Close()) })
					if !automatic {
						smtDisableAutoConfig(t, solver)
					}
					k := solver.kernel
					k.smtParams.bv.state.CC = true
					k.bvState.CC = true
					family, err := m.RegisterBitVectorPlugin()
					astTestMust(t, err)
					theory := k.internalizer.theories[family].(*SMTBitVectorTheory)
					astTestMust(t, theory.ConfigureBitVector(k.bvState))
					one, err := m.BitVectorNumeralUint64(1, 4)
					astTestMust(t, err)
					sort, err := m.GetSort(one)
					astTestMust(t, err)
					x, err := m.MkConst(astTestSymbol(t, m, "bv-setup-x"), sort)
					astTestMust(t, err)
					equality, err := m.MkEq(x, one)
					astTestMust(t, err)
					astTestMust(t, solver.Assert(equality))
					check := func() {
						t.Helper()
						if k.smtParams.bv.state.CC == automatic || k.bvState.CC == automatic || theory.config.CC == automatic || k.smtParams.preprocess.bit.ExtGates != automatic || theory.bitBlasterParams.ExtGates != automatic || k.relevancy.enabled == automatic || k.preprocessState.NNFCNF == automatic {
							t.Fatal("BV setup owner and consumer controls diverged")
						}
						arrays := logic == "QF_AUFBV" || logic == "QF_ABV" || logic == "QF_UFBV"
						if automatic {
							if arrays {
								if !k.arithState.Reflect || k.smtParams.array.state.Mode != 1 || !k.preprocessState.PropagateBooleans {
									t.Fatal("array/BV policy changed reflection or omitted array/Boolean settings")
								}
							} else if k.arithState.Reflect || k.smtParams.arith.state.Reflect {
								t.Fatal("plain BV profile retained arithmetic reflection")
							}
							if logic == "QF_BVRE" && (!k.arithState.Eq2Ineq || k.arithState.PropagateEqs || !k.arithState.GCDTest || k.arithState.BranchCutRatio != 2) {
								t.Fatal("BVRE did not use the logic-only QF_LIA profile")
							}
						}
					}
					if push {
						astTestMust(t, solver.Push())
						check()
					}
					status, err := smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("BV profile changed satisfiability", status)
					}
					check()
					if logic == "QF_BVRE" {
						// AUDIT.md smtcore_06-F11: setup_QF_BVRE registers
						// bv, then arith, then seq; setup_unknown keeps its
						// own order.
						var order []string
						for _, plugin := range k.internalizer.theoryOrder {
							name, err := m.FamilyName(plugin.FamilyID())
							astTestMust(t, err)
							text, err := m.Symbols.String(name)
							astTestMust(t, err)
							order = append(order, text)
						}
						want := "[bv arith seq]"
						if !automatic {
							want = "[arith array bv datatype datalog_relation seq pb fpa]"
						}
						if got := fmt.Sprint(order); got != want {
							t.Fatal("QF_BVRE theory order", got, "want", want)
						}
					}
					if push {
						astTestMust(t, solver.Pop(1))
						check()
					}
				})
			}
		}
	}
}

func TestSMTBitVectorArithmeticUsesConfiguredExtendedGates(t *testing.T) {
	for _, extended := range []bool{false, true} {
		t.Run(fmt.Sprint(extended), func(t *testing.T) {
			m, family, solver := smtBVSolver(t)
			if !extended {
				smtDisableAutoConfig(t, solver)
			}
			astTestMust(t, solver.Push())
			family, err := m.RegisterBitVectorPlugin()
			astTestMust(t, err)
			theory := solver.kernel.internalizer.theories[family].(*SMTBitVectorTheory)
			left, right := make([]ast.ASTHandle, 4), make([]ast.ASTHandle, 4)
			for i := range left {
				left[i] = smtRelevancyBool(t, m, fmt.Sprint("adder-left-", i))
				right[i] = smtRelevancyBool(t, m, fmt.Sprint("adder-right-", i))
			}
			add, err := theory.addBits(left, right)
			astTestMust(t, err)
			subtract, carry, err := theory.subtractBits(left, right)
			astTestMust(t, err)
			for _, bits := range [][]ast.ASTHandle{add, append(subtract, carry)} {
				found := make(map[int32]bool)
				stack := append([]ast.ASTHandle(nil), bits...)
				seen := make(map[ast.ASTHandle]bool)
				for len(stack) > 0 {
					i := len(stack) - 1
					h := stack[i]
					stack = stack[:i]
					if seen[h] {
						continue
					}
					seen[h] = true
					op, own, err := theory.operation(h)
					astTestMust(t, err)
					if own {
						found[op] = true
					}
					n, err := m.GetNumArgs(h)
					astTestMust(t, err)
					for j := uint32(0); j < n; j++ {
						arg, err := m.GetArg(h, j)
						astTestMust(t, err)
						stack = append(stack, arg)
					}
				}
				if found[59] != extended || found[60] != extended {
					t.Fatal("arithmetic did not honor carry/xor3 gate policy", found, extended)
				}
			}
			astTestMust(t, solver.Pop(1))
		})
	}
}
