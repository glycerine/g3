package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDatalogUDocFiniteUnionNativeCleanupKeepsDeltaBeforeContextDisposal(t *testing.T) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var context *datalogUDocContext
	var roots []ast.ASTRef
	var attempt *udocFiniteUnionAttempt
	var delta *rel.TypedUDocRelation
	var factory *rel.UDocRelationFactory
	var snapshot rel.FiniteTable
	active, called := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !called {
			called = true
			failure := closeASTRefs(manager, roots)
			roots = nil
			if !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &attempt.nested) {
				t.Fatal("finite union lost the native decoded-root cleanup receipt", failure)
			}
			context.finiteUnionCleanup = append(context.finiteUnionCleanup, attempt)
			for i := 0; i < 2; i++ {
				if err := context.clearFiniteUnionCleanup(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("finite union retry changed native cleanup refusal", err)
				}
				if err := context.close(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("context disposal bypassed pending finite union roots", err)
				}
				if len(context.finiteUnionCleanup) != 1 || context.finiteUnionCleanup[0] != attempt || attempt.nested == nil || attempt.delta != delta || context.factory != factory {
					t.Fatal("native refusal lost the finite union owner frontier")
				}
				if delta.Core() == nil || delta.Core().NumRecords() != 1 {
					t.Fatal("pending decoded roots did not preserve the actual delta")
				}
				if supported, err := factory.CanHandleSignature(nil); err != nil || !supported {
					t.Fatal("context retired the DOC factory before its pending delta", supported, err)
				}
				if count, err := snapshot.Len(); err != nil || count != 1 {
					t.Fatal("context retired its finite snapshot before nested cleanup", count, err)
				}
			}
		}
		return value
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
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
		if used := budget.Used(); used != 0 {
			t.Fatal("finite union native cleanup fixture leaked bytes", used)
		}
	}()
	must(manager.RegisterDeclarationPlugins())
	relations, err := rel.NewRelationManager(manager)
	must(err)
	defer func() { must(relations.Close()) }()
	context, err = newDatalogUDocContext(manager)
	must(err)
	defer func() { must(context.close()) }()
	context.relations = relations
	factory = context.factory
	sort, err := manager.BitVectorSort(2)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("finite-union-native-cleanup-p")
	must(err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	predicateRef, err := manager.NewRef(predicate)
	must(err)
	defer func() { must(manager.DelRef(&predicateRef)) }()
	signature, err := rel.NewSignature([]uint64{4})
	must(err)
	source := rel.NewHashTable(signature)
	defer func() { must(source.Close()) }()
	_, err = source.Add(rel.Fact{1})
	must(err)
	snapshot, err = source.CloneFinite()
	must(err)
	context.finiteInputs = map[ast.ASTHandle]*udocFiniteInput{predicate: {table: source, sorts: []ast.ASTHandle{sort}}}
	context.finiteResult = map[ast.ASTHandle]rel.FiniteTable{predicate: snapshot}
	target, err := factory.NewEmpty([]ast.ASTHandle{sort})
	must(err)
	context.seeds[predicate] = target
	delta, err = factory.NewEmpty([]ast.ASTHandle{sort})
	must(err)
	defer func() { must(delta.Close()) }()
	must(rel.UnionUDocFromTable(target, snapshot, relations, delta))
	attempt = &udocFiniteUnionAttempt{delta: delta}
	guard, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&guard)) }()
	must(numbers.SetString(&guard, "18446744073709551617/18446744073709551619"))
	_, err = manager.ArithmeticNumeral(&guard, false)
	must(err)
	baseline := manager.Stats().LiveReferences
	roots, err = relations.DecodeFact([]ast.ASTHandle{sort}, rel.Fact{1})
	must(err)
	root := roots[0]

	// Reenter cleanup while the arithmetic factory holds its native AST guard.
	// The pending decoded row must finish before the context disposes delta.
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&guard, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !called || attempt.nested == nil {
		t.Fatal("native guard did not reach finite union cleanup", body, called)
	}
	nested := attempt.nested
	if nested.RuntimeOwner() != runtime || nested.CleanupCause() == nil {
		t.Fatal("finite union nested receipt lost runtime or cleanup cause")
	}
	if _, err = manager.RefGet(root); err != nil {
		t.Fatal("native refusal consumed the decoded row", err)
	}
	shown, exposed := nested.Error(), nested.Unwrap()
	must(context.clearFiniteUnionCleanup())
	must(context.clearFiniteUnionCleanup())
	if len(context.finiteUnionCleanup) != 0 || attempt.delta != nil || attempt.nested != nil || delta.Core() != nil || nested.CleanupCause() != nil || nested.Error() != shown || nested.Unwrap() != exposed {
		t.Fatal("finite union cleanup left an owner or changed its stable receipt")
	}
	if context.factory != factory || target.Core() == nil {
		t.Fatal("delta cleanup consumed the surrounding DOC context")
	}
	if count, err := snapshot.Len(); err != nil || count != 1 {
		t.Fatal("delta cleanup consumed the context's finite snapshot", count, err)
	}
	if _, err = manager.RefGet(root); err == nil || manager.Stats().LiveReferences != baseline {
		t.Fatal("finite union cleanup leaked decoded references", err, baseline, manager.Stats())
	}
	must(context.close())
	must(context.close())
	if _, err = factory.CanHandleSignature(nil); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("completed context disposal left its factory open", err)
	}
	if _, err = snapshot.Len(); err == nil {
		t.Fatal("completed context disposal left its owned finite snapshot open")
	}
	if found, err := source.Contains(rel.Fact{1}); err != nil || !found {
		t.Fatal("context disposal changed its borrowed extensional input", found, err)
	}
}
