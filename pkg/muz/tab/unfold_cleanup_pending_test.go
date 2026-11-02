package tab

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type unfoldNestedCleanupTestFailure struct {
	runtime *memory.Runtime
	pending bool
}

func (f *unfoldNestedCleanupTestFailure) Error() string {
	if f.pending {
		return ast.ErrASTPending.Error()
	}
	return "nested cleanup complete"
}
func (f *unfoldNestedCleanupTestFailure) Unwrap() error                 { return ast.ErrASTPending }
func (f *unfoldNestedCleanupTestFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *unfoldNestedCleanupTestFailure) OriginalCause() error          { return ast.ErrASTPending }
func (f *unfoldNestedCleanupTestFailure) CleanupCause() error {
	if f.pending {
		return ast.ErrASTPending
	}
	return nil
}
func (f *unfoldNestedCleanupTestFailure) Cleanup() error {
	if f.pending {
		f.pending = false
		return ast.ErrASTPending
	}
	return nil
}

func TestUnfoldConstructionPreservesResultOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *unfoldConstructionAttempt
	var published *unfoldedClause
	var matched bool
	var failure error
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil {
			published, matched, failure = finishUnfoldConstruction(attempt, true, nil)
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
	name, err := symbols.Intern("tab-unfold-cleanup")
	must(err)
	declaration, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	application, err := manager.MkApp(declaration, nil)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	head, err := manager.NewRef(application)
	must(err)
	constraint, err := manager.NewRef(truth)
	must(err)
	left, err := manager.NewRef(truth)
	must(err)
	right, err := manager.NewRef(truth)
	must(err)
	temporary, err := manager.NewRef(truth)
	must(err)
	subst, err := substitution.New(manager)
	must(err)
	unifier, err := substitution.NewUnifier(manager)
	must(err)
	result := &unfoldedClause{manager: manager, head: head, constraint: constraint}
	attempt = &unfoldConstructionAttempt{manager: manager, temporary: []ast.ASTRef{temporary}, left: left, right: right, substitution: subst, unifier: unifier, result: result}

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
		t.Fatal("unfold cleanup refusal did not suppress publication", body, published, matched, failure)
	}
	owned, ok := failure.(*unfoldConstructionFailure)
	if !ok || owned.attempt != attempt || attempt.result != result || attempt.right != right || attempt.left != left || attempt.unifier != unifier || attempt.substitution != subst || result.head != head || result.constraint != constraint {
		t.Fatal("refused unfold cleanup lost a construction owner")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("unfold failure is not retryable")
	}
	origin, exposed, message := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if attempt.result != nil || len(attempt.temporary) != 0 || attempt.right != (ast.ASTRef{}) || attempt.left != (ast.ASTRef{}) || attempt.unifier != nil || attempt.substitution != nil || cleanup.CleanupCause() != nil {
		t.Fatal("unfold cleanup retry incomplete")
	}
	if cleanup.OriginalCause() != origin || errors.Unwrap(cleanup) != exposed || cleanup.Error() != message || !errors.Is(cleanup, origin) {
		t.Fatal("unfold cleanup retry changed its receipt")
	}

	nestedRight, err := manager.NewRef(truth)
	must(err)
	nested := &unfoldNestedCleanupTestFailure{runtime: runtime, pending: true}
	nestedAttempt := &unfoldConstructionAttempt{manager: manager, right: nestedRight}
	_, _, nestedErr := finishUnfoldConstruction(nestedAttempt, false, nested)
	var nestedReceipt memory.ConstructionFailure
	if !errors.As(nestedErr, &nestedReceipt) || nestedAttempt.nested != nested || nestedAttempt.right != nestedRight {
		t.Fatal("nested unfold failure was not preserved", nestedErr)
	}
	nestedMessage := nestedReceipt.Error()
	must(nestedReceipt.Cleanup())
	if nestedAttempt.nested != nil || nestedAttempt.right != (ast.ASTRef{}) {
		t.Fatal("nested unfold cleanup stranded a construction owner")
	}
	if nestedReceipt.Error() != nestedMessage || errors.Unwrap(nestedReceipt) != nested {
		t.Fatal("nested unfold cleanup changed its outer receipt")
	}

	consumed := &unfoldConstructionAttempt{manager: manager, temporary: []ast.ASTRef{{}}}
	if err := consumed.Close(); !errors.Is(err, ast.ErrASTHandle) || len(consumed.temporary) != 0 {
		t.Fatal("consumed temporary remained in the cleanup frontier", err, consumed.temporary)
	}
	must(consumed.Close())
}
