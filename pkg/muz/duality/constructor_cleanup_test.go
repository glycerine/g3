package duality

import (
	"errors"
	"io"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Exercise the real constructor finalizers inside an active native guard.
// Construction itself runs normally; no factory substitutes or solver mocks
// manufacture the cleanup refusal or the owned children.
func TestDualityConstructorNativeFailureKeepsOwners(t *testing.T) {
	for _, tc := range []struct {
		name          string
		nestedBackend bool
		localCleanup  bool
	}{
		{name: "direct-body-failure"},
		{name: "native-local-cleanup", localCleanup: true},
		{name: "backend-nested", nestedBackend: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 30)
			var nativeAttempt *nativeSolverConstruction
			var backendAttempt *backendConstruction
			var candidate *Backend
			var temporaryConfig *graph.Config
			var native, published solver.Solver
			var publishedBackend *Backend
			var proof *ast.ScopedProofMode
			var receipt, nestedReceipt memory.ConstructionFailure
			var failure, retryFailure error
			active, callback, intact := false, false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
				if active && !callback {
					callback = true
					cause := error(io.ErrClosedPipe)
					if tc.localCleanup {
						cause = nil
					}
					published, failure = nativeAttempt.finish(cause)
					if backendAttempt != nil {
						errors.As(failure, &nestedReceipt)
						publishedBackend, failure = backendAttempt.finish(failure)
					}
					if errors.As(failure, &receipt) {
						retryFailure = receipt.Cleanup()
					}
					intact = published == nil && nativeAttempt.native == native && nativeAttempt.proof == proof
					if backendAttempt != nil {
						intact = intact && publishedBackend == nil && nestedReceipt != nil && backendAttempt.nested == nestedReceipt && backendAttempt.backend == candidate && backendAttempt.config == temporaryConfig && !candidate.closed && candidate.queries != nil && !candidate.queries.closing
					}
				}
				return f
			}})
			must(err)
			numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
			must(err)
			symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
			must(err)
			params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
			must(err)
			catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
			must(err)
			globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
			must(err)
			must(globals.InitializeSource())
			cooperation, err := cooperate.NewCooperation(budget)
			must(err)
			manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
			must(err)
			must(manager.RegisterDeclarationPlugins())
			config := smtcore.SMTSolverConfig{Params: params, Globals: globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: runtime, Cooperation: cooperation}}
			initial, err := manager.ProofMode()
			must(err)
			cfg, err := graph.NewConfig(params, nil)
			must(err)
			context, err := graph.NewContext(manager, cfg)
			must(err)
			factory, err := smtcore.NewSMTSolverFactory(config)
			must(err)
			defer func() {
				active = false
				must(runtime.SetMaxCount(0))
				must(factory.Close())
				must(context.Close())
				must(cfg.Close())
				must(manager.Close())
				must(cooperation.Close())
				must(globals.Close())
				must(catalog.Close())
				must(params.Close())
				must(numbers.Close())
				must(symbols.Close())
				must(runtime.Close())
				if used := budget.Used(); used != 0 {
					t.Fatal("constructor fixture leaked bytes", used)
				}
			}()

			if tc.nestedBackend {
				candidate, err = NewBackend(manager, config, true)
				must(err)
				// The failed inner constructor owns this exact real solver;
				// transfer it out of the enclosing backend before its finalizer.
				native, candidate.auxiliary = candidate.auxiliary, nil
				temporaryConfig, err = graph.NewConfig(params, nil)
				must(err)
				must(temporaryConfig.SetBool("mbqi", true))
				backendAttempt = &backendConstruction{backend: candidate, config: temporaryConfig, runtime: runtime}
			} else {
				native, err = NewNativeSolver(context, params, factory, true)
				must(err)
			}
			settings := params.EmptyValue()
			borrowedSettings, err := native.Params()
			must(err)
			must(params.Assign(&settings, borrowedSettings))
			proof, err = ast.NewScopedProof(manager)
			must(err)
			nativeAttempt = &nativeSolverConstruction{native: native, params: params, settings: settings, proof: proof, runtime: runtime}
			settings = uparams.ParamsRef{}
			value, err := numbers.NewValue()
			must(err)
			defer func() { must(numbers.Del(&value)) }()
			must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
			_, err = manager.ArithmeticNumeral(&value, false)
			must(err)
			var invocation ast.ASTArithmeticNumeralInvocation
			defer func() {
				active = false
				must(runtime.SetMaxCount(0))
				must(manager.CleanupArithmeticNumeralInvocation(&invocation))
				if backendAttempt != nil {
					must(backendAttempt.cleanup())
				}
				must(nativeAttempt.cleanup())
			}()
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			active = true
			_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			wantCause := error(io.ErrClosedPipe)
			if tc.localCleanup {
				wantCause = ast.ErrASTPending
			}
			if body == nil || !callback || !intact || receipt == nil || !errors.Is(failure, wantCause) || !errors.Is(failure, ast.ErrASTPending) || !errors.Is(retryFailure, ast.ErrASTPending) {
				t.Fatal("constructor finalizer lost its refused owners", body, callback, intact, failure, retryFailure)
			}
			mode, err := manager.ProofMode()
			must(err)
			if mode != ast.ASTProofEnabled {
				t.Fatal("refusal retired the enclosing proof mode")
			}
			if tc.localCleanup {
				if nativeAttempt.settings != (uparams.ParamsRef{}) || errors.Is(failure, io.ErrClosedPipe) {
					t.Fatal("normal constructor exit did not retire settings before proof restoration refused")
				}
			} else if nativeAttempt.settings.Slot == 0 {
				t.Fatal("failed child cleanup retired enclosing constructor settings")
			}
			if receipt.RuntimeOwner() != runtime || !errors.Is(receipt.OriginalCause(), wantCause) || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
				t.Fatal("constructor receipt lost runtime or separate causes", receipt)
			}
			shown, exposed := receipt.Error(), receipt.Unwrap()
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			must(receipt.Cleanup())
			must(receipt.Cleanup())
			if nativeAttempt.native != nil || nativeAttempt.proof != nil || nativeAttempt.settings != (uparams.ParamsRef{}) || receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || receipt.RuntimeOwner() != runtime {
				t.Fatal("constructor cleanup left an owner or changed its receipt")
			}
			if backendAttempt != nil && (backendAttempt.backend != nil || backendAttempt.config != nil || backendAttempt.nested != nil || !candidate.closed || nestedReceipt.CleanupCause() != nil) {
				t.Fatal("backend cleanup lost its nested native constructor")
			}
			mode, err = manager.ProofMode()
			must(err)
			if mode != initial {
				t.Fatal("constructor cleanup did not restore proof mode", mode, initial)
			}
			if _, err := native.Params(); err == nil {
				t.Fatal("failed constructor solver remained open")
			}
			_, err = context.Manager()
			must(err)
			retry, err := NewNativeSolver(context, params, factory, false)
			must(err)
			must(retry.Close())
			retryBackend, err := NewBackend(manager, config, false)
			must(err)
			defer func() { must(retryBackend.Close()) }()
			query, err := manager.MkFalse()
			must(err)
			prepared, err := PrepareQuery(manager, nil, nil, query, nil, math.MaxUint32)
			must(err)
			defer func() { must(prepared.Close()) }()
			status, err := retryBackend.Query(prepared)
			must(err)
			if status != lbool.False {
				t.Fatal("borrowed constructor services did not support a new query", status)
			}
		})
	}
}
