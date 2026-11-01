package nlsatsmt

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestNLPurifyCooperationResourceCancellationUsesSourceMessage(t *testing.T) {
	m := astTestManager(t)
	_, _, config := nlModelEvaluatorFixture(t, m)
	goal, err := tactic.NewGoal(m, false, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	prepared, err := prepareNLPurification(goal)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, prepared.Close()) })
	smt, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, smt.Close()) })
	nonlinear, err := nlsattactic.NewNLSATTactic(m, config.Context, config.Globals, nil, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, nonlinear.Close()) })
	cooperation, err := newNLCooperation(prepared, smt, nonlinear, config)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cooperation.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	limit := m.ResourceLimit()
	astTestMust(t, limit.Push(1))
	if !limit.Inc() {
		t.Fatal("resource limit exhausted too early")
	}
	err = cooperation.solve(&result)
	limit.Pop()
	var exception tactic.TacticException
	if !errors.As(err, &exception) || exception.Message() != messages.Canceled {
		t.Fatal("cooperation cancellation", err)
	}
	limit.IncCancel()
	err = cooperation.solve(&result)
	limit.DecCancel()
	if !errors.As(err, &exception) || exception.Message() != messages.Canceled {
		t.Fatal("cooperation explicit cancellation", err)
	}
}

func TestNLPurifyCooperatingSolversMixedSatisfiability(t *testing.T) {
	for _, tc := range []struct {
		name          string
		contradiction bool
		cores         bool
	}{{"sat", false, false}, {"unsat", true, false}, {"sat-cores", false, true}, {"unsat-cores", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m := astTestManager(t)
			_, _, config := nlModelEvaluatorFixture(t, m)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			x := analysisConst(t, m, real, "solve-x")
			decl, err := m.MkFuncDecl(astTestSymbol(t, m, "solve-f"), []ast.ASTHandle{real}, real, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			h, err := m.MkApp(decl, []ast.ASTHandle{x})
			fx := analysisKeep(t, m, h, err)
			h, err = m.ArithmeticRealNumeralText("0")
			zero := analysisKeep(t, m, h, err)
			h, err = m.ArithmeticRealNumeralText("1")
			one := analysisKeep(t, m, h, err)
			h, err = m.MkArithmeticGE(fx, one)
			positive := analysisKeep(t, m, h, err)
			goal, err := tactic.NewGoal(m, false, true, tc.cores)
			astTestMust(t, err)
			defer func() { astTestMust(t, goal.Close()) }()
			var labels []ast.ASTHandle
			assert := func(formula ast.ASTHandle, name string) {
				var dep ast.ASTDependency
				if tc.cores {
					boolean, err := m.MkBoolSort()
					astTestMust(t, err)
					label := analysisConst(t, m, boolean, name)
					labels = append(labels, label)
					dep, err = m.DependencyLeaf(label)
					astTestMust(t, err)
				}
				astTestMust(t, goal.AssertWithProof(formula, ast.ASTHandle{}, dep))
			}
			assert(positive, "solve-positive")
			if tc.contradiction {
				h, err = m.MkArithmeticLE(fx, zero)
				negative := analysisKeep(t, m, h, err)
				assert(negative, "solve-negative")
			}
			prepared, err := prepareNLPurification(goal)
			astTestMust(t, err)
			defer func() { astTestMust(t, prepared.Close()) }()
			smt, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
			astTestMust(t, err)
			defer func() { astTestMust(t, smt.Close()) }()
			astTestMust(t, smt.SetProduceModels(true))
			astTestMust(t, prepared.assertSMT(smt))
			nonlinear, err := nlsattactic.NewNLSATTactic(m, config.Context, config.Globals, nil, config)
			astTestMust(t, err)
			defer func() { astTestMust(t, nonlinear.Close()) }()
			cooperation, err := newNLCooperation(prepared, smt, nonlinear, config)
			astTestMust(t, err)
			defer func() { astTestMust(t, cooperation.Close()) }()
			var result tactic.TacticResult
			defer func() { astTestMust(t, result.Close()) }()
			astTestMust(t, cooperation.solve(&result))
			if tc.contradiction && !result.DecidedUnsat() {
				t.Fatal("expected unsat", len(result.Goals))
			}
			if !tc.contradiction && (!result.DecidedSat() || result.ModelConverter == nil) {
				t.Fatal("expected sat with model", len(result.Goals), result.ModelConverter)
			}
			if tc.contradiction && tc.cores {
				leaves, err := m.DependencyLinearize(result.Core, nil)
				astTestMust(t, err)
				if len(leaves) != 2 || !((leaves[0] == labels[0] && leaves[1] == labels[1]) || (leaves[0] == labels[1] && leaves[1] == labels[0])) {
					t.Fatal("unsat dependencies", leaves, labels)
				}
			}
			if !tc.contradiction {
				var answer tactic.ModelRef
				astTestMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &answer))
				defer func() { astTestMust(t, answer.Close()) }()
				core, err := answer.Get()
				astTestMust(t, err)
				evaluator, err := model.NewModelEvaluator(core, config)
				astTestMust(t, err)
				defer func() { astTestMust(t, evaluator.Close()) }()
				value, err := evaluator.EvaluateValue(positive)
				astTestMust(t, err)
				defer func() { astTestMust(t, m.DelRef(&value)) }()
				actual, err := m.RefGet(value)
				astTestMust(t, err)
				if !m.IsTrue(actual) {
					t.Fatal("model does not satisfy input", actual)
				}
			}
		})
	}
}
