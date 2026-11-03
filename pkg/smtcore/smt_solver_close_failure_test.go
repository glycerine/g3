package smtcore

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtSolverCloseFailureFixture(t *testing.T, optimize bool, refuse func() error) (*ast.ASTManager, *SMTSolver, func() error) {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
	var refusal error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if refusal == nil {
			refusal = refuse()
		}
		return source
	}})
	astTestMust(t, err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	astTestMust(t, err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	astTestMust(t, err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	astTestMust(t, err)
	_, err = m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	astTestMust(t, err)
	var s *SMTSolver
	if optimize {
		s, err = NewOptimizationSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: params})
	} else {
		s, err = NewSMTSolver(m, nil, "QF_UF", SMTSolverConfig{Params: params})
	}
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, s.Close())
		astTestMust(t, params.Close())
		astTestMust(t, m.Close())
		astTestMust(t, numbers.Close())
		astTestMust(t, symbols.Close())
		astTestMust(t, runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("solver close left owned bytes", budget.Used())
		}
	})
	trigger := func() error {
		value, err := numbers.NewValue()
		astTestMust(t, err)
		defer func() { astTestMust(t, numbers.Del(&value)) }()
		astTestMust(t, numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
		astTestMust(t, runtime.SetMaxCount(uint64(runtime.State().GlobalCount)+1))
		var invocation ast.ASTArithmeticNumeralInvocation
		_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		astTestMust(t, runtime.SetMaxCount(0))
		astTestMust(t, m.CleanupArithmeticNumeralInvocation(&invocation))
		if body == nil || refusal == nil {
			t.Fatal("native allocation transport did not refuse cleanup", body, refusal)
		}
		return refusal
	}
	return m, s, trigger
}

func TestSMTSolverClosePreservesNativeCleanupOwners(t *testing.T) {
	for _, owner := range []string{"consequence", "result", "named", "tracked", "objective", "saved-model", "current-model", "saved-consumed-model", "current-consumed-model", "kernel", "decref"} {
		t.Run(owner, func(t *testing.T) {
			var s *SMTSolver
			consumedModel := owner == "saved-consumed-model" || owner == "current-consumed-model"
			m, value, trigger := smtSolverCloseFailureFixture(t, owner == "saved-model" || owner == "current-model" || consumedModel, func() error {
				if owner == "decref" {
					return s.DecRef()
				}
				return s.Close()
			})
			s = value
			p := smtRelevancyBool(t, m, "solver-close-owned-root")
			caller, err := m.NewRef(p)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.DelRef(&caller)) })
			astTestMust(t, s.Push())
			var md *model.ModelCore
			switch owner {
			case "consequence":
				_, err = s.RetainConsequence(p)
				astTestMust(t, err)
				// The preceding cleanup attempt consumed this final token.
				retired, err := m.NewRef(p)
				astTestMust(t, err)
				astTestMust(t, m.DelRef(&retired))
				s.consequences = append(s.consequences, retired)
			case "result":
				s.result, err = solver.NewSimpleCheckSatResult(m)
				astTestMust(t, err)
				astTestMust(t, s.result.SetUnsatCore([]ast.ASTHandle{p}))
			case "named":
				astTestMust(t, s.AssertTracked(p, p))
			case "tracked":
				root, err := m.NewRef(p)
				astTestMust(t, err)
				s.tracked = append(s.tracked, smtTrackedAssertion{formula: root})
			case "objective":
				root, err := m.NewRef(p)
				astTestMust(t, err)
				s.objectives = append(s.objectives, smtSolverObjective{term: root}, smtSolverObjective{})
			case "saved-model", "current-model", "saved-consumed-model", "current-consumed-model":
				md, err = model.NewModelCore(m)
				astTestMust(t, err)
				astTestMust(t, md.IncRef())
				decl, err := m.GetDecl(p)
				astTestMust(t, err)
				truth, err := m.MkTrue()
				astTestMust(t, err)
				astTestMust(t, md.RegisterConstant(decl, truth))
				if owner == "saved-model" || owner == "saved-consumed-model" {
					s.optimization.models = []*model.ModelCore{md}
				} else {
					s.optimization.model = md
				}
				if consumedModel {
					// The last decrement committed before a child release failed.
					md.References = 0
					s.closingModel = md
				}
			case "kernel":
				astTestMust(t, s.Assert(p))
				// The inner scope already popped before the wrapper closed.
				astTestMust(t, s.kernel.internalizer.PopScopes(1))
				s.kernel.pendingPop = &smtKernelPendingPop{count: 1, stage: smtKernelPopBVSharing}
				s.pendingPop = &smtSolverPendingPop{count: 1}
			case "decref":
				astTestMust(t, s.IncRef())
				_, err = s.RetainConsequence(p)
				astTestMust(t, err)
			}
			kernel := s.kernel
			failure := trigger()
			if !errors.Is(failure, ast.ErrASTPending) || !s.closing || s.closed || s.kernel != kernel {
				t.Fatal("refused cleanup lost its solver owner", failure, s.closing, s.closed)
			}
			astTestMust(t, s.config.Params.Check(&s.params))
			astTestMust(t, s.ownedGlobals.CheckOpen())
			if md != nil {
				wantRefs := uint32(1)
				var wantCleanup *model.ModelCore
				if consumedModel {
					wantRefs, wantCleanup = 0, md
				}
				if md.References != wantRefs || s.closingModel != wantCleanup || md.Closed {
					t.Fatal("model retirement lost its consumption state", md.References, s.closingModel, md.Closed)
				}
			}
			for _, operation := range []func() error{
				s.Push, func() error { return s.Pop(1) },
				func() error { _, err := s.CheckSat(nil); return err },
				func() error { _, err := s.NumAssumptions(); return err },
				func() error { _, err := s.Assumption(0); return err },
				func() error { _, err := s.RetainConsequence(p); return err }, s.ResetObjectives,
			} {
				if err := operation(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("closing solver accepted another operation", err)
				}
			}
			if owner == "decref" {
				if s.references != 0 {
					t.Fatal("last solver reference was not consumed", s.references)
				}
				astTestMust(t, s.DecRef())
			} else {
				astTestMust(t, s.Close())
			}
			astTestMust(t, s.Close())
			if !s.closed || md != nil && (!md.Closed || md.References != 0) {
				t.Fatal("solver retry did not finish owned cleanup")
			}
			refs, err := m.GetRefCount(p)
			astTestMust(t, err)
			if refs != 1 {
				t.Fatal("solver cleanup changed caller ownership", refs)
			}
		})
	}
}
