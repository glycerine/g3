package bmc

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestQuantifiedLinearSearchCleanupPreservesScopeTermsAndModel(t *testing.T) {
	for _, name := range []string{"scope", "terms-before-scope", "nested-before-scope", "model", "consumed-model"} {
		t.Run(name, func(t *testing.T) {
			modelOnly := name == "model" || name == "consumed-model"
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var attempt *quantifiedLinearSearchAttempt
			var failure error
			var published *model.ModelCore
			var childRefs []ast.ASTRef
			var manager *ast.ASTManager
			active := false
			var cause error
			if modelOnly {
				cause = errors.New("quantified search failed after acquiring its model")
			}
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
				if active && failure == nil {
					inner := cause
					if name == "nested-before-scope" {
						inner = disposeASTRefs(manager, childRefs, nil)
					}
					published, failure = finishQuantifiedLinearSearch(attempt, inner)
				}
				return source
			}})
			must(err)
			numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
			must(err)
			symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
			must(err)
			m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
			must(err)
			manager = m
			params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
			must(err)
			defer func() {
				must(params.Close())
				must(m.Close())
				must(numbers.Close())
				must(symbols.Close())
				must(runtime.Close())
				if budget.Used() != 0 {
					t.Fatal("quantified search cleanup leaked bytes", budget.Used())
				}
			}()
			_, err = m.RegisterArithmeticPlugin()
			must(err)
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: params})
			must(err)
			defer func() { must(solver.Close()) }()
			boolean, err := m.MkBoolSort()
			must(err)
			guardName, err := symbols.Intern("quantified-cleanup-guard")
			must(err)
			guard, err := m.MkConst(guardName, boolean)
			must(err)
			guardRef, err := m.NewRef(guard)
			must(err)
			defer func() { must(m.DelRef(&guardRef)) }()
			if name == "nested-before-scope" {
				childRef, err := m.NewRef(guard)
				must(err)
				childRefs = []ast.ASTRef{childRef}
			}
			must(solver.Push())
			must(solver.AssertFormula(guard))
			attempt = &quantifiedLinearSearchAttempt{manager: m, solver: solver}
			if !modelOnly {
				must(attempt.begin())
			}
			negated, err := m.MkNot(guard)
			must(err)
			negatedRef, err := m.NewRef(negated)
			must(err)
			defer func() { must(m.DelRef(&negatedRef)) }()
			if !modelOnly {
				must(solver.AssertFormula(negated))
			}
			if name == "terms-before-scope" {
				attempt.terms, err = newQuantifiedLinear(m, 4)
				must(err)
			}
			winning, err := model.NewModelCore(m)
			must(err)
			must(winning.IncRef())
			declaration, err := m.GetDecl(guard)
			must(err)
			truth, err := m.MkTrue()
			must(err)
			must(winning.RegisterConstant(declaration, truth))
			attempt.winning = winning
			wantReferences := uint32(1)
			if name == "consumed-model" {
				// Stage the frontier after DecRefConsumed decrements its last
				// reference but before the model's constant roots close.
				winning.References = 0
				attempt.modelConsumed = true
				wantReferences = 0
			}
			value, err := numbers.NewValue()
			must(err)
			defer func() { must(numbers.Del(&value)) }()
			must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			var invocation ast.ASTArithmeticNumeralInvocation
			active = true
			_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			must(m.CleanupArithmeticNumeralInvocation(&invocation))
			if body == nil || !errors.Is(failure, ast.ErrASTPending) || published != nil || attempt.winning != winning || winning.References != wantReferences {
				t.Fatal("refusal lost scope/model or published a result", body, failure, published, winning.References)
			}
			if cause != nil && !errors.Is(failure, cause) {
				t.Fatal("model cleanup lost original operation failure", failure)
			}
			beforeRetry, err := solver.ScopeLevel()
			must(err)
			wantLevel := uint32(2)
			if modelOnly {
				wantLevel = 1
			}
			if beforeRetry != wantLevel || name == "terms-before-scope" && (attempt.terms == nil || len(attempt.terms.refs) != 3) || name == "nested-before-scope" && childRefs[0] == (ast.ASTRef{}) {
				t.Fatal("cleanup refusal advanced a dependent owner", beforeRetry, wantLevel)
			}
			var cleanup memory.ConstructionFailure
			if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
				t.Fatal("missing search cleanup receipt", failure)
			}
			original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
			must(cleanup.Cleanup())
			must(cleanup.Cleanup())
			level, err := solver.ScopeLevel()
			must(err)
			if level != 1 || attempt.pushed || attempt.terms != nil || attempt.winning != nil || !winning.Closed || winning.References != 0 {
				t.Fatal("search retry did not restore caller scope and owners", level, attempt.pushed, winning.Closed, winning.References)
			}
			if name == "nested-before-scope" && childRefs[0] != (ast.ASTRef{}) {
				t.Fatal("nested cleanup retry left a child root")
			}
			if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message || cleanup.CleanupCause() != nil {
				t.Fatal("search retry changed original failure")
			}
			status, err := solver.CheckSat([]ast.ASTHandle{negated})
			must(err)
			if status != lbool.False {
				t.Fatal("search cleanup changed caller assertion", status)
			}
			must(solver.Pop(1))
		})
	}
}
