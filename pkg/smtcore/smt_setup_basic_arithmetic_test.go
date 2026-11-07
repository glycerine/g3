package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSMTBasicArithmeticModesBeforeFirstBoundary(t *testing.T) {
	for _, mode := range []uint32{0, 1, 2, 3, 4, 5} {
		for _, real := range []bool{false, true} {
			for _, push := range []bool{false, true} {
				t.Run(fmt.Sprint(mode, "/", real, "/", push), func(t *testing.T) {
					m, cfg := smtModelGeneratorFixture(t)
					params, err := cfg.Context.NewValue()
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
					name := "auto_config"
					astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
					name = "arith.solver"
					astTestMust(t, cfg.Context.SetUIntName(&params, &name, mode))
					solver, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, solver.Close()) })
					astTestMust(t, solver.SetProduceModels(true))
					k := solver.kernel
					family, err := m.RegisterArithmeticPlugin()
					astTestMust(t, err)
					kind := int32(1)
					if real {
						kind = 0
					}
					sort, err := m.MkFamilySort(family, kind, nil)
					astTestMust(t, err)
					x, err := m.MkConst(astTestSymbol(t, m, "basic-mode-x"), sort)
					astTestMust(t, err)
					lower, upper := big.NewRat(1, 1), big.NewRat(2, 1)
					if real {
						lower, upper = big.NewRat(1, 6), big.NewRat(1, 3)
					}
					for _, bound := range []struct {
						op    int32
						value *big.Rat
					}{{smtArithOpGE, lower}, {smtArithOpLE, upper}} {
						n, err := m.ArithmeticNumeralRat(bound.value, !real)
						astTestMust(t, err)
						formula, err := m.MkFamilyApp(family, bound.op, nil, []ast.ASTHandle{x, n})
						astTestMust(t, err)
						astTestMust(t, solver.Assert(formula))
					}
					if push {
						astTestMust(t, solver.Push())
					}
					status, err := smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if mode == 0 {
						if status != lbool.Undef {
							t.Fatal("disabled arithmetic did not give up", status)
						}
						if _, ok := k.internalizer.theories[family].(*SMTDummyTheory); !ok {
							t.Fatal("disabled arithmetic did not select its source checkpoint")
						}
						return
					}
					if status != lbool.True {
						t.Fatal("basic arithmetic input should be satisfiable", status)
					}
					switch mode {
					case 1:
						if graph, ok := k.internalizer.theories[family].(*SMTRealDiffTheory); !ok || !graph.mixed {
							t.Fatal("mixed graph not selected")
						}
					case 2:
						theory, ok := smtArithFactsOf(k.internalizer.theories[family])
						if !ok || theory.integer || !theory.mixed {
							t.Fatal("basic simplex integrality selection")
						}
					case 3:
						if _, ok := k.internalizer.theories[family].(*SMTDenseDiffTheory); !ok {
							t.Fatal("dense mode not selected")
						}
						stats := statistics.New()
						astTestMust(t, solver.CollectStatistics(stats))
						found := false
						for i := 0; i < stats.Size(); i++ {
							if stats.Key(i) == "dd assertions" {
								found = stats.IsUint(i) && stats.UintValue(i) != 0
							}
						}
						if !found {
							t.Fatal("solver statistics omitted the selected dense backend")
						}
					case 4:
						if _, ok := k.internalizer.theories[family].(*SMTUTVPITheory); !ok {
							t.Fatal("UTVPI mode not selected")
						}
						stats := statistics.New()
						astTestMust(t, solver.CollectStatistics(stats))
						found := false
						for i := 0; i < stats.Size(); i++ {
							if stats.Key(i) == "utvpi asserts" {
								found = stats.IsUint(i) && stats.UintValue(i) != 0
							}
						}
						if !found {
							t.Fatal("solver statistics omitted the selected UTVPI backend")
						}
					case 5:
						theory, ok := smtArithFactsOf(k.internalizer.theories[family])
						if !ok || !theory.infinite {
							t.Fatal("infinite arithmetic mode not selected")
						}
					}
					if k.arithState.Eq2Ineq != (mode == 1 || mode == 3 || mode == 4) || k.smtParams.arith.state != k.arithState || !k.preprocessState.NNFCNF {
						t.Fatal("basic selection changed unrelated policy or omitted eq2ineq")
					}
					generated, err := solver.AcquireModel()
					astTestMust(t, err)
					declaration, err := m.GetDecl(x)
					astTestMust(t, err)
					value, err := generated.ConstantInterpretation(declaration)
					astTestMust(t, err)
					text, integer, matched, err := m.ArithmeticNumeralText(value)
					astTestMust(t, err)
					astTestMust(t, generated.DecRef())
					actual, ok := new(big.Rat).SetString(text)
					if !matched || !ok || integer == real || actual.Cmp(lower) < 0 || actual.Cmp(upper) > 0 {
						t.Fatal("basic backend published an invalid typed model", text, integer)
					}
					astTestMust(t, solver.Push())
					zero, err := m.ArithmeticNumeralRat(new(big.Rat), !real)
					astTestMust(t, err)
					conflict, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{x, zero})
					astTestMust(t, err)
					astTestMust(t, solver.Assert(conflict))
					status, err = smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if status != lbool.False {
						t.Fatal("basic backend child contradiction", status)
					}
					astTestMust(t, solver.Pop(1))
					status, err = smtSetupProfileCheck(solver)
					astTestMust(t, err)
					if status != lbool.True {
						t.Fatal("basic backend parent recovery", status)
					}
					if push {
						astTestMust(t, solver.Pop(1))
					}
				})
			}
		}
	}
}

func TestSMTBasicArithmeticSeesPreSetupParameterUpdate(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	params, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&params)) })
	name := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&params, &name, false))
	name = "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&params, &name, 0))
	solver, err := NewSMTSolver(m, &params, "ALL", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, solver.Close()) })
	astTestMust(t, cfg.Context.SetUIntName(&params, &name, 3))
	astTestMust(t, solver.UpdateParams(&params))
	zero, err := m.ArithmeticRealNumeralText("0")
	astTestMust(t, err)
	sort, err := m.GetSort(zero)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "mode-update-x"), sort)
	astTestMust(t, err)
	equality, err := m.MkEq(x, zero)
	astTestMust(t, err)
	astTestMust(t, solver.Assert(equality))
	status, err := smtSetupProfileCheck(solver)
	astTestMust(t, err)
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("pre-setup mode update retained disabled arithmetic", status)
	}
	if _, ok := solver.kernel.internalizer.theories[family].(*SMTDenseDiffTheory); !ok {
		t.Fatal("pre-setup mode update did not select dense arithmetic")
	}
}
