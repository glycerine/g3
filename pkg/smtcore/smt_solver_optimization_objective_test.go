package smtcore

import (
	"errors"
	"math/big"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func optimizationRepairFixture(t *testing.T, integer bool) (*SMTSolver, *arithmeticObjectiveFixture, *tactic.FilterModelConverter) {
	t.Helper()
	return optimizationRepairFixtureWithHook(t, integer, nil)
}

func optimizationRepairFixtureWithHook(t *testing.T, integer bool, hook *optimizationValueChangeTheory) (*SMTSolver, *arithmeticObjectiveFixture, *tactic.FilterModelConverter) {
	t.Helper()
	m, cfg := smtModelGeneratorFixture(t)
	p, err := cfg.Context.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, cfg.Context.Del(&p)) })
	key := "auto_config"
	astTestMust(t, cfg.Context.SetBoolName(&p, &key, false))
	key = "arith.solver"
	astTestMust(t, cfg.Context.SetUIntName(&p, &key, 2))
	logic, kind := "QF_UFLRA", int32(0)
	if integer {
		logic, kind = "QF_UFLIA", 1
	}
	s, err := NewOptimizationSMTSolver(m, &p, logic, SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.preprocess = false
	astTestMust(t, s.SetProduceModels(true))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	f := &arithmeticObjectiveFixture{t: t, m: m, s: s.kernel.internalizer, sort: sort, integer: integer}
	if hook != nil {
		astTestMust(t, s.kernel.RegisterTheory(hook))
	}
	startSMTModelUpdate(t, s, f)
	if hook != nil {
		hook.arithmetic = f.a
	}
	filter, err := tactic.NewFilterModel(m)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, filter.Close()) })
	return s, f, filter
}

func TestOptimizationObjectiveSharedRepairDecrementsRealOrInteger(t *testing.T) {
	for _, integer := range []bool{false, true} {
		s, f, filter := optimizationRepairFixture(t, integer)
		x := f.variable("repair-shared-x")
		five := f.number("5")
		decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "repair-shared-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
		astTestMust(t, err)
		fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
		f5 := f.pin(f.m.MkApp(decl, []ast.ASTHandle{five}))
		eq := f.pin(f.m.MkEq(fx, f5))
		astTestMust(t, s.Assert(f.pin(f.m.MkNot(eq))))
		astTestMust(t, s.Assert(f.app(smtArithOpLE, x, five)))
		index, err := s.AddObjective(x)
		astTestMust(t, err)
		status, err := s.CheckSat(nil)
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal(status)
		}
		old := acquireUpdateModel(t, s)
		oldValue := modelUpdateNumber(t, f.m, old, x)
		checks, scopes := s.kernel.internalizer.statistics.NumChecks, s.kernel.ScopeLevel()
		value, blocker, err := s.MaximizeOptimizationObjective(index, filter)
		astTestMust(t, err)
		defer func() { astTestMust(t, f.m.DelRef(&blocker)) }()
		main, epsilon := "5", "-1"
		if integer {
			main, epsilon = "4", "0"
		}
		checkArithmeticObjective(t, value, main, epsilon)
		valid, err := s.OptimizationObjectiveModelValid(index)
		astTestMust(t, err)
		if valid || s.kernel.internalizer.statistics.NumChecks != checks+1 || s.kernel.ScopeLevel() != scopes {
			t.Fatal("missing failed scoped repair", integer, valid, checks, s.kernel.internalizer.statistics.NumChecks)
		}
		if s.optimization.models[index] != old || modelUpdateNumber(t, f.m, s.optimization.models[index], x) != oldValue {
			t.Fatal("failed repair replaced last valid model")
		}
		saved, err := s.SavedObjectiveValue(index)
		astTestMust(t, err)
		checkArithmeticObjective(t, saved, main, epsilon)
		saved.Finite.Main.SetInt64(999)
		again, err := s.SavedObjectiveValue(index)
		astTestMust(t, err)
		checkArithmeticObjective(t, again, main, epsilon)
		condition, err := f.m.RefGet(blocker)
		astTestMust(t, err)
		if condition.IsNull() {
			t.Fatal("repair lost independently owned blocker")
		}
	}
}

func TestOptimizationObjectiveUnsharedRefreshAndSavedOwnership(t *testing.T) {
	s, f, filter := optimizationRepairFixture(t, false)
	x := f.variable("repair-unshared-x")
	astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
	index, err := s.AddObjective(x)
	astTestMust(t, err)
	initial, err := s.SavedObjectiveValue(index)
	astTestMust(t, err)
	if initial.Finite.Infinity.Sign() != -1 {
		t.Fatal("initial objective sentinel", initial)
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	old := acquireUpdateModel(t, s)
	oldValue := modelUpdateNumber(t, f.m, old, x)
	checks := s.kernel.internalizer.statistics.NumChecks
	value, blocker, err := s.MaximizeOptimizationObjective(index, filter)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&blocker)) }()
	checkArithmeticObjective(t, value, "5", "0")
	valid, err := s.OptimizationObjectiveModelValid(index)
	astTestMust(t, err)
	if !valid || s.kernel.internalizer.statistics.NumChecks != checks {
		t.Fatal("unshared optimization ran repair", valid)
	}
	current := acquireUpdateModel(t, s)
	if current == old || s.optimization.models[index] != current || modelUpdateNumber(t, f.m, current, x) != "5" || modelUpdateNumber(t, f.m, old, x) != oldValue {
		t.Fatal("saved/current/prior model ownership")
	}
	astTestMust(t, s.Close())
	if modelUpdateNumber(t, f.m, current, x) != "5" {
		t.Fatal("caller model lost after solver close")
	}
}

func TestOptimizationObjectiveSatisfiableRepairUsesKernelModelWithoutDump(t *testing.T) {
	optimizationDumpTestDirectory(t)
	s, f, filter := optimizationRepairFixture(t, false)
	x := f.variable("repair-sat-x")
	astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
	index, err := s.AddObjective(x)
	astTestMust(t, err)
	optimizationDumpParameters(t, s, true)
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	value := SMTArithmeticObjectiveValue{}
	value.Finite.Main.SetInt64(5)
	checks, count, scopes := s.kernel.internalizer.statistics.NumChecks, optimizationDumpCount, s.kernel.ScopeLevel()
	status, err = s.decrementOptimizationValue(index, filter, &value)
	astTestMust(t, err)
	if status != lbool.True || optimizationDumpCount != count || s.kernel.internalizer.statistics.NumChecks != checks+1 || s.kernel.ScopeLevel() != scopes {
		t.Fatal("internal recheck wrapper/scope", status, optimizationDumpCount, count)
	}
	checkArithmeticObjective(t, value, "5", "0")
	if s.optimization.models[index] == nil || modelUpdateNumber(t, f.m, s.optimization.models[index], x) != "5" {
		t.Fatal("repair saved stale facade model")
	}
	current := acquireUpdateModel(t, s)
	if current != s.optimization.models[index] {
		t.Fatal("repair model was not cached across pop")
	}
}

func TestOptimizationObjectiveSharedRepairAllocationFrontier(t *testing.T) {
	success, failures := false, 0
	for after := int64(0); after < 256; after++ {
		t.Run(strconv.FormatInt(after, 10), func(t *testing.T) {
			s, f, filter := optimizationRepairFixture(t, false)
			x := f.variable("repair-frontier-x")
			five := f.number("5")
			decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "repair-frontier-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
			f5 := f.pin(f.m.MkApp(decl, []ast.ASTHandle{five}))
			eq := f.pin(f.m.MkEq(fx, f5))
			astTestMust(t, s.Assert(f.pin(f.m.MkNot(eq))))
			astTestMust(t, s.Assert(f.app(smtArithOpLE, x, five)))
			index, err := s.AddObjective(x)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			prior := acquireUpdateModel(t, s)
			priorValue := modelUpdateNumber(t, f.m, prior, x)
			f.m.Budget().FailAfter(after)
			value, blocker, err := s.MaximizeOptimizationObjective(index, filter)
			f.m.Budget().FailAfter(-1)
			if blocker != (ast.ASTRef{}) {
				astTestMust(t, f.m.DelRef(&blocker))
			}
			if err != nil {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("unexpected repair refusal", after, err)
				}
				failures++
			} else if s.kernel.lastFailure == SMTFailureMemory {
				// A source memory-unknown repair is an exercised refusal, not
				// evidence that the complete reservation sequence succeeded.
				failures++
			} else {
				success = true
				checkArithmeticObjective(t, value, "5", "-1")
			}
			if modelUpdateNumber(t, f.m, prior, x) != priorValue {
				t.Fatal("repair refusal changed caller model")
			}
		})
		if t.Failed() {
			t.Fatalf("shared repair frontier %d", after)
		}
		if success {
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("shared repair frontier incomplete", success, failures)
	}
}

type optimizationValueChangeTheory struct {
	*smtInternalizerTestTheory
	arithmetic *SMTLinearArithmeticTheory
	column     uint32
	enabled    bool
	fired      int
	next       simplex.Infinitesimal
}

func (t *optimizationValueChangeTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	t.finalCheckCalls++
	if t.enabled {
		t.enabled = false
		t.fired++
		t.arithmetic.solver.SetValue(t.column, t.next)
	}
	return SMTFinalCheckDone, nil
}

func TestOptimizationObjectiveSuccessfulSharedRefreshRechecksChangedValue(t *testing.T) {
	for _, test := range []struct {
		name          string
		main, epsilon int64
		repair        bool
	}{
		{"rational", 4, 0, true}, {"epsilon", 5, -1, true}, {"unchanged", 5, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			base, err := NewSMTTheoryBase(ast.FamilyID(73))
			astTestMust(t, err)
			hook := &optimizationValueChangeTheory{smtInternalizerTestTheory: &smtInternalizerTestTheory{SMTTheoryBase: base}}
			hook.next = simplex.RationalInfinitesimal(big.NewRat(test.main, 1))
			hook.next.Epsilon.SetInt64(test.epsilon)
			s, f, filter := optimizationRepairFixtureWithHook(t, false, hook)
			x := f.variable("repair-changed-x")
			astTestMust(t, s.Assert(f.app(smtArithOpGE, x, f.number("1"))))
			astTestMust(t, s.Assert(f.app(smtArithOpLE, x, f.number("5"))))
			decl, err := f.m.MkFuncDecl(astTestSymbol(t, f.m, "repair-changed-f"), []ast.ASTHandle{f.sort}, f.sort, ast.ASTFuncDeclInfo{})
			astTestMust(t, err)
			fx := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
			astTestMust(t, s.Assert(f.pin(f.m.MkEq(fx, f.number("0")))))
			index, err := s.AddObjective(x)
			astTestMust(t, err)
			status, err := s.CheckSat(nil)
			astTestMust(t, err)
			if status != lbool.True {
				t.Fatal(status)
			}
			hook.column = f.a.simplexVariables[s.objectives[index].variable]
			hook.enabled = true
			checks := s.kernel.internalizer.statistics.NumChecks
			value, blocker, err := s.MaximizeOptimizationObjective(index, filter)
			astTestMust(t, err)
			defer func() { astTestMust(t, f.m.DelRef(&blocker)) }()
			expectedChecks := checks
			if test.repair {
				expectedChecks++
			}
			if hook.fired != 1 || s.kernel.internalizer.statistics.NumChecks != expectedChecks {
				t.Fatal("successful shared refresh skipped changed-value repair", hook.fired, checks, s.kernel.internalizer.statistics.NumChecks)
			}
			checkArithmeticObjective(t, value, "5", "0")
			valid, err := s.OptimizationObjectiveModelValid(index)
			astTestMust(t, err)
			if !valid || s.optimization.models[index] == nil || modelUpdateNumber(t, f.m, s.optimization.models[index], x) != "5" {
				t.Fatal("changed-value repair did not save verified bound model", valid)
			}
		})
	}
}
