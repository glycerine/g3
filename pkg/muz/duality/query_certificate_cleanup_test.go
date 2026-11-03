package duality

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source display_certificate owns its annotation model through output and
// destruction. Exercise that exact finalizer while native AST work refuses
// release; a stopped numeral invocation alone does not hold this guard.
func TestDualitySafeCertificateNativeFailurePreservesModelOwner(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "direct"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 30)
			var attempt, inner *certificateModelScope
			var snapshot, innerSnapshot *model.Model
			var receipt, innerReceipt memory.ConstructionFailure
			var failure, retryFailure error
			active, callback, intact := false, false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
				if active && !callback {
					callback = true
					cause := error(io.ErrClosedPipe)
					if inner != nil {
						cause = inner.finish(cause)
						errors.As(cause, &innerReceipt)
					}
					failure = attempt.finish(cause)
					if errors.As(failure, &receipt) {
						retryFailure = receipt.Cleanup()
					}
					intact = attempt.snapshot == snapshot && !snapshot.Closed
					if inner != nil {
						intact = intact && innerReceipt != nil && attempt.nested == innerReceipt && inner.snapshot == innerSnapshot && !innerSnapshot.Closed
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
			backend, err := NewBackend(manager, config, true)
			must(err)
			defer func() {
				active = false
				must(runtime.SetMaxCount(0))
				must(backend.Close())
				must(manager.Close())
				must(cooperation.Close())
				must(globals.Close())
				must(catalog.Close())
				must(params.Close())
				must(numbers.Close())
				must(symbols.Close())
				must(runtime.Close())
				if used := budget.Used(); used != 0 {
					t.Fatal("certificate cleanup fixture leaked bytes", used)
				}
			}()

			boolean, err := manager.MkBoolSort()
			must(err)
			predicateName, err := symbols.Intern("certificate-native-P")
			must(err)
			predicate, err := manager.MkFuncDecl(predicateName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			truth, err := manager.MkTrue()
			must(err)
			falsity, err := manager.MkFalse()
			must(err)
			fact, err := manager.MkApp(predicate, []ast.ASTHandle{truth})
			must(err)
			query, err := manager.MkApp(predicate, []ast.ASTHandle{falsity})
			must(err)
			prepared, err := PrepareQuery(manager, []ast.ASTHandle{fact}, []uint32{math.MaxUint32}, query, []ast.ASTHandle{predicate}, math.MaxUint32)
			must(err)
			defer func() { must(prepared.Close()) }()
			status, err := backend.Query(prepared)
			must(err)
			if status != lbool.False {
				t.Fatal("fixture did not produce safe annotations", status)
			}
			value, err := numbers.NewValue()
			must(err)
			defer func() { must(numbers.Del(&value)) }()
			must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
			_, err = manager.ArithmeticNumeral(&value, false)
			must(err)
			before := manager.Stats().LiveReferences
			snapshot, err = backend.GetModel()
			must(err)
			attempt = &certificateModelScope{snapshot: snapshot, runtime: runtime}
			if nested {
				innerSnapshot, err = backend.GetModel()
				must(err)
				inner = &certificateModelScope{snapshot: innerSnapshot, runtime: runtime}
			}
			var invocation ast.ASTArithmeticNumeralInvocation
			defer func() {
				active = false
				must(runtime.SetMaxCount(0))
				must(manager.CleanupArithmeticNumeralInvocation(&invocation))
				must(attempt.cleanup())
				if inner != nil {
					must(inner.cleanup())
				}
			}()
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			active = true
			_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			if body == nil || !callback || !intact || receipt == nil || !errors.Is(failure, io.ErrClosedPipe) || !errors.Is(failure, ast.ErrASTPending) || !errors.Is(retryFailure, ast.ErrASTPending) {
				t.Fatal("native certificate finalization lost its model owner", body, callback, intact, failure, retryFailure)
			}
			if receipt.RuntimeOwner() != runtime || !errors.Is(receipt.OriginalCause(), io.ErrClosedPipe) || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
				t.Fatal("certificate receipt lost runtime or separate causes", receipt)
			}
			shown, exposed := receipt.Error(), receipt.Unwrap()
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			must(receipt.Cleanup())
			must(receipt.Cleanup())
			if attempt.snapshot != nil || !snapshot.Closed || attempt.nested != nil || receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || receipt.RuntimeOwner() != runtime {
				t.Fatal("certificate cleanup left an owner or changed its receipt")
			}
			if inner != nil && (inner.snapshot != nil || !innerSnapshot.Closed || innerReceipt.CleanupCause() != nil) {
				t.Fatal("certificate cleanup lost its nested snapshot")
			}
			if after := manager.Stats().LiveReferences; after != before {
				t.Fatal("certificate model roots leaked", before, after)
			}
			var output bytes.Buffer
			must(backend.WriteCertificate(&output, nil))
			if !bytes.Contains(output.Bytes(), []byte("certificate-native-P")) {
				t.Fatal("cleanup changed the active safe annotations", output.String())
			}
		})
	}
}
