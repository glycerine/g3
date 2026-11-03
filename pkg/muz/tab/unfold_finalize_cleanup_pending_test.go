package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestUnfoldFinalizePreservesClausesOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *unfoldFinalizeAttempt
	var published *Clause
	var matched bool
	var failure error
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil {
			published, matched, failure = finishUnfoldFinalize(attempt, true, nil)
		}
		return source
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("tab-finalize-cleanup")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	application, err := manager.MkApp(declaration, nil)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	newClause := func() *Clause {
		head, cause := manager.NewRef(application)
		must(cause)
		constraint, cause := manager.NewRef(truth)
		must(cause)
		return &Clause{manager: manager, head: head, constraint: constraint}
	}
	work, result := newClause(), newClause()
	temporary, err := manager.NewRef(truth)
	must(err)
	attempt = &unfoldFinalizeAttempt{manager: manager, temps: []ast.ASTRef{temporary}, work: work, result: result}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || published != nil || matched || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("finalize cleanup refusal published a partial result", body, published, matched, failure)
	}
	owned, ok := failure.(*unfoldFinalizeFailure)
	if !ok || owned.attempt != attempt || attempt.work != work || attempt.result != result || work.closed || result.closed {
		t.Fatal("finalize cleanup refusal lost clause owners")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("finalize cleanup failure is not retryable")
	}
	original, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if attempt.work != nil || attempt.result != nil || len(attempt.temps) != 0 || !work.closed || !result.closed || cleanup.CleanupCause() != nil {
		t.Fatal("finalize cleanup retry incomplete")
	}
	if cleanup.OriginalCause() != original || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message || !errors.Is(cleanup, original) {
		t.Fatal("finalize cleanup retry changed its receipt")
	}
}
