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

var errTabQueryInjected = errors.New("tab query failed after answer construction")

func TestTabQueryAttemptPreservesSearchAndAnswerOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *tabQueryAttempt
	var answer ast.ASTRef
	var failure error
	var trigger func()
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil && trigger != nil {
			trigger()
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
	name, err := symbols.Intern("tab-query-cleanup")
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
	clause := &Clause{manager: manager, head: head, constraint: constraint}
	search := &tabSearch{manager: manager, owned: []*Clause{clause}}
	answer, err = manager.NewRef(truth)
	must(err)
	compacted, err := manager.NewRef(application)
	must(err)
	expressionAnswer, err := manager.NewRef(truth)
	must(err)
	attempt = &tabQueryAttempt{manager: manager, search: search}
	trigger = func() {
		returned, finishErr := attempt.finish(answer, errTabQueryInjected)
		if returned != (ast.ASTRef{}) {
			t.Fatal("failed query returned an answer")
		}
		failure = finishErr
	}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	invoke := func() error {
		must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
		var invocation ast.ASTArithmeticNumeralInvocation
		active = true
		_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
		active = false
		must(runtime.SetMaxCount(0))
		must(manager.CleanupArithmeticNumeralInvocation(&invocation))
		return body
	}
	body := invoke()
	if body == nil || !errors.Is(failure, errTabQueryInjected) || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("did not reach query cleanup refusal", body, failure)
	}
	owned, ok := failure.(*tabQueryCleanupFailure)
	if !ok || owned.attempt != attempt || attempt.search != search || attempt.answer != answer || search.closed || clause.closed {
		t.Fatal("refused query cleanup lost an owner")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("query failure is not retryable", failure)
	}
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if attempt.search != nil || attempt.answer != (ast.ASTRef{}) || !search.closed || !clause.closed || cleanup.CleanupCause() != nil {
		t.Fatal("query cleanup retry incomplete")
	}

	goalHead, err := manager.NewRef(application)
	must(err)
	goalConstraint, err := manager.NewRef(truth)
	must(err)
	goal := &Clause{manager: manager, head: goalHead, constraint: goalConstraint}
	goalAttempt := &tabQueryAttempt{manager: manager, goal: goal}
	failure = nil
	trigger = func() {
		returned, finishErr := goalAttempt.finish(ast.ASTRef{}, errTabQueryInjected)
		if returned != (ast.ASTRef{}) {
			t.Fatal("failed query construction returned an answer")
		}
		failure = finishErr
	}
	body = invoke()
	if body == nil || !errors.Is(failure, errTabQueryInjected) || !errors.Is(failure, ast.ErrASTPending) || goalAttempt.goal != goal || goal.closed {
		t.Fatal("query construction cleanup lost pending goal", body, failure)
	}
	cleanup, ok = failure.(memory.ConstructionFailure)
	if !ok {
		t.Fatal("query construction failure is not retryable", failure)
	}
	goalOriginal, goalExposed, goalMessage := cleanup.OriginalCause(), errors.Unwrap(cleanup), cleanup.Error()
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if goalAttempt.goal != nil || !goal.closed || cleanup.CleanupCause() != nil {
		t.Fatal("query construction goal cleanup retry incomplete")
	}
	if cleanup.OriginalCause() != goalOriginal || errors.Unwrap(cleanup) != goalExposed || cleanup.Error() != goalMessage {
		t.Fatal("query construction cleanup changed its receipt")
	}

	expressionAttempt := &tabExpressionAttempt{manager: manager, compacted: compacted}
	failure = nil
	trigger = func() {
		returned, finishErr := expressionAttempt.finish(expressionAnswer, errTabQueryInjected)
		if returned != (ast.ASTRef{}) {
			t.Fatal("failed expression query returned an answer")
		}
		failure = finishErr
	}
	body = invoke()
	if body == nil || !errors.Is(failure, errTabQueryInjected) || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("did not reach expression cleanup refusal", body, failure)
	}
	expressionFailure, ok := failure.(*tabExpressionCleanupFailure)
	if !ok || expressionFailure.attempt != expressionAttempt || expressionAttempt.compacted != compacted || expressionAttempt.answer != expressionAnswer {
		t.Fatal("refused expression cleanup lost an owner")
	}
	cleanup = expressionFailure
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if expressionAttempt.compacted != (ast.ASTRef{}) || expressionAttempt.answer != (ast.ASTRef{}) || cleanup.CleanupCause() != nil {
		t.Fatal("expression cleanup retry incomplete")
	}
}
