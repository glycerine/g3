package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestSMTLRAPublicSetupLogicAutoAndFirstPush(t *testing.T) {
	for _, logic := range []string{"QF_LRA", "QF_UFLRA"} {
		for _, automatic := range []bool{false, true} {
			for _, push := range []bool{false, true} {
				t.Run(fmt.Sprint(logic, "/", automatic, "/", push), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					params, err := cfg.Context.NewValue()
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
					key := "auto_config"
					astTestMust(t, cfg.Context.SetBoolName(&params, &key, automatic))
					s, err := NewSMTSolver(m, &params, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, s.Close()) })
					astTestMust(t, s.SetProduceModels(true))
					family, err := m.ArithmeticFamilyID()
					astTestMust(t, err)
					sort, err := m.MkFamilySort(family, 0, nil)
					astTestMust(t, err)
					pin := func(h ast.ASTHandle, e error) ast.ASTHandle {
						astTestMust(t, e)
						ref, e := m.NewRef(h)
						astTestMust(t, e)
						t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
						return h
					}
					x := pin(m.MkConst(astTestSymbol(t, m, "public-lra-x"), sort))
					y := pin(m.MkConst(astTestSymbol(t, m, "public-lra-y"), sort))
					number := func(n int64) ast.ASTHandle { return pin(bigTestArithNumeral(m, big.NewRat(n, 1), false)) }
					app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle { return pin(m.MkFamilyApp(family, op, nil, args)) }
					// Late context LP parameters must be read at source plugin initialization.
					key = "simplex_strategy"
					astTestMust(t, cfg.Context.SetUIntName(&params, &key, 2))
					key = "bprop_on_pivoted_rows"
					astTestMust(t, cfg.Context.SetBoolName(&params, &key, false))
					astTestMust(t, s.UpdateParams(&params))
					if push {
						astTestMust(t, s.Push())
					}
					for _, h := range []ast.ASTHandle{app(smtArithOpGT, x, number(0)), app(smtArithOpLT, x, number(3)), app(smtArithOpGE, y, x), pin(m.MkEq(y, app(smtArithOpAdd, app(smtArithOpMul, number(2), x), number(1))))} {
						astTestMust(t, s.Assert(h))
					}
					status, err := s.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("public LRA check", status)
					}
					a, selected := s.kernel.internalizer.Theory(family).(*SMTLRATheory)
					if selected != automatic {
						t.Fatal("wrong source setup backend", selected, automatic)
					}
					if selected {
						if a.config.Reflect || a.numeric.solver.Settings.SimplexStrategy != lp.StrategyLU || a.numeric.solver.Core.RPrimal.PivotedRows != nil {
							t.Fatal("source profile/LP initialization was not forwarded")
						}
						for bv := range a.boundByBoolean {
							expr, err := s.kernel.internalizer.BoolExpression(bv)
							astTestMust(t, err)
							op, err := m.GetDeclKind(expr)
							astTestMust(t, err)
							rhs, err := m.GetArg(expr, 1)
							astTestMust(t, err)
							_, matched, err := a.numeralBig(rhs)
							astTestMust(t, err)
							if (op != smtArithOpLE && op != smtArithOpGE) || !matched {
								t.Fatal("asserted arithmetic was not normalized for LRA", op, matched)
							}
						}
					}
					model, err := s.AcquireModel()
					astTestMust(t, err)
					var values [2]*big.Rat
					for i, h := range []ast.ASTHandle{x, y} {
						decl, err := m.GetDecl(h)
						astTestMust(t, err)
						value, err := model.ConstantInterpretation(decl)
						astTestMust(t, err)
						text, _, matched, err := m.ArithmeticNumeralText(value)
						astTestMust(t, err)
						if !matched {
							t.Fatal("missing model numeral")
						}
						values[i], matched = new(big.Rat).SetString(text)
						if !matched {
							t.Fatal("invalid rational", text)
						}
					}
					astTestMust(t, model.DecRef())
					if values[0].Sign() <= 0 || values[0].Cmp(big.NewRat(3, 1)) >= 0 || values[1].Cmp(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), values[0]), big.NewRat(1, 1))) != 0 {
						t.Fatal("invalid public LRA model", values)
					}
					astTestMust(t, s.Push())
					astTestMust(t, s.Assert(app(smtArithOpLE, x, number(0))))
					status, err = s.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("public child contradiction", status)
					}
					astTestMust(t, s.Pop(1))
					status, err = s.CheckSat(nil)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("public pop recovery", status)
					}
				})
			}
		}
	}
}

func TestSMTLRAPublicSetupAllocationFailureRetiresCandidate(t *testing.T) {
	failures, successes := 0, 0
	for _, allowance := range []int64{-1, 0, 1, 2, 4, 8, 12, 24, 48} {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			m, cfg := smtModelGeneratorFixture(t)
			s, err := NewSMTSolver(m, nil, "QF_UFLRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, s.Close()) })
			m.Budget().FailAfter(allowance)
			err = s.kernel.installRealLRA()
			m.Budget().FailAfter(-1)
			if err != nil {
				var failure umemory.Failure
				if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &failure) && failure.Kind == umemory.FailureMemout && allowance >= 0) {
					t.Fatal("unexpected setup refusal", err)
				}
				failures++
			} else {
				successes++
			}
			if s.kernel.pendingSetupLRA != nil {
				astTestMust(t, s.kernel.pendingSetupLRA.Close())
			}
		})
	}
	if failures == 0 || successes == 0 {
		t.Fatal("setup requires failure/success coverage", failures, successes)
	}
}

func TestSMTLRAPublicSetupProofAndTrackedConflict(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	s, err := NewSMTSolver(m, nil, "QF_UFLRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	pin := func(h ast.ASTHandle, e error) ast.ASTHandle {
		astTestMust(t, e)
		ref, e := m.NewRef(h)
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, m.DelRef(&ref)) })
		return h
	}
	x, y := pin(m.MkConst(astTestSymbol(t, m, "proof-lra-x"), sort)), pin(m.MkConst(astTestSymbol(t, m, "proof-lra-y"), sort))
	number := func(n int64) ast.ASTHandle { return pin(bigTestArithNumeral(m, big.NewRat(n, 1), false)) }
	app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle { return pin(m.MkFamilyApp(family, op, nil, args)) }
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	formulas := []ast.ASTHandle{app(smtArithOpGE, x, number(0)), app(smtArithOpGE, y, number(0)), app(smtArithOpLE, app(smtArithOpAdd, x, y), number(-1))}
	tags := make(map[ast.ASTHandle]bool)
	for i, h := range formulas {
		label := pin(m.MkConst(astTestSymbol(t, m, fmt.Sprint("lra-premise-", i)), boolean))
		tags[label] = true
		astTestMust(t, s.AssertTracked(h, label))
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("tracked LRA conflict", status)
	}
	if _, ok := s.kernel.internalizer.Theory(family).(*SMTLRATheory); !ok {
		t.Fatal("proof mode bypassed LRA")
	}
	core, err := s.UnsatCore()
	astTestMust(t, err)
	if len(core) != 3 {
		t.Fatal("tracked core size", len(core))
	}
	for _, tag := range core {
		if !tags[tag] {
			t.Fatal("foreign core assumption")
		}
		delete(tags, tag)
	}
	proof, err := s.Proof()
	astTestMust(t, err)
	if proof.IsNull() {
		t.Fatal("proof unavailable")
	}
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if !m.IsFalse(fact) {
		t.Fatal("proof does not conclude false")
	}
}
