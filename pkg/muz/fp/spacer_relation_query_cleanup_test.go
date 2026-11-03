package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerRelationQueryFinalizerPreservesUnpublishedRule(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var manager *ast.ASTManager
	var attempt *spacerRelationQueryAttempt
	var queryDeclaration ast.ASTHandle
	var pending memory.ConstructionFailure
	active, called := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			declaration, rule, failure := attempt.finish(queryDeclaration, nil)
			if !declaration.IsNull() || rule != nil || !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &pending) {
				t.Fatalf("unfinished relation rule published or lost: %v", failure)
			}
			if pending.OriginalCause() != nil || attempt.rule == nil || len(attempt.roots) != 2 {
				t.Fatal("successful construction lost its unpublished owners")
			}
			for i := 0; i < 2; i++ {
				if err := pending.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatalf("native refusal changed on retry: %v", err)
				}
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("query roots leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	engine, err := NewPropositionalEngine(manager)
	must(err)
	defer func() { must(engine.Close()) }()
	boolean, err := manager.GetSort(truth)
	must(err)
	name, err := symbols.Intern("pending-relation-query")
	must(err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean, boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(declaration))
	query, err := engine.makeRelationQueryExpression(declaration)
	must(err)
	queryHandle, err := manager.RefGet(query)
	must(err)
	queryDeclaration, headRef, err := engine.makeQueryHead(queryHandle)
	must(err)
	head, err := manager.RefGet(headRef)
	must(err)
	must(engine.rules.RegisterPredicate(queryDeclaration))
	baseline := manager.Stats().LiveReferences - 2
	rule, err := engine.rules.NewRule(head, []ast.ASTHandle{queryHandle}, nil, "pending-query-rule")
	must(err)
	attempt = &spacerRelationQueryAttempt{manager: manager, roots: []ast.ASTRef{query, headRef}, rule: rule}
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
	if body == nil || !called || pending == nil {
		t.Fatal("native refusal missed query vector cleanup", body, called)
	}
	if pending.RuntimeOwner() != runtime || pending.CleanupCause() == nil {
		t.Fatal("cleanup owner lost runtime/cause")
	}
	// Temporary cleanup refused after a real rule was constructed. The
	// finalizer must keep that same unpublished rule until cleanup can resume.
	failure, ok := pending.(*spacerRelationQueryFailure)
	if !ok || failure.attempt != attempt || attempt.rule != rule {
		t.Fatal("relation rule lost its scoped cleanup owner")
	}
	ownedHead, err := attempt.rule.Head()
	must(err)
	ownedTail, negative, err := attempt.rule.Tail(0)
	must(err)
	if ownedHead != head || ownedTail != queryHandle || negative {
		t.Fatal("refused finalization changed the unpublished rule")
	}
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.CleanupCause() != nil || pending.RuntimeOwner() != runtime || attempt.rule != nil || len(attempt.roots) != 0 {
		t.Fatal("cleanup completion lost stable ownership")
	}
	if manager.Stats().LiveReferences != baseline {
		t.Fatal("query reference retry leaked roots", baseline, manager.Stats().LiveReferences)
	}
}
