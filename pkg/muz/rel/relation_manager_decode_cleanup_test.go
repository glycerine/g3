package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDecodedFactCleanupTransportsNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var manager *ast.ASTManager
	var roots []ast.ASTRef
	var pending memory.ConstructionFailure
	active, called := false, false
	original := errors.New("query copy failed")
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !called {
			called = true
			failure := finishDecodedFact(manager, roots, original)
			roots = nil
			if !errors.Is(failure, original) || !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &pending) {
				t.Fatalf("decoded fact owner lost: %v", failure)
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
			t.Fatal("decoded fact roots leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	baseline := manager.Stats().LiveReferences
	for i := 0; i < 3; i++ {
		root, cause := manager.NewRef(truth)
		must(cause)
		roots = append(roots, root)
	}
	// A completed earlier release must not be repeated by the retry owner.
	must(manager.DelRef(&roots[1]))
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
		t.Fatal("native refusal missed decoded fact cleanup", body, called)
	}
	if pending.RuntimeOwner() != runtime || pending.CleanupCause() == nil {
		t.Fatal("cleanup owner lost runtime/cause")
	}
	origin, exposed, message := pending.OriginalCause(), errors.Unwrap(pending), pending.Error()
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.CleanupCause() != nil || pending.RuntimeOwner() != runtime {
		t.Fatal("cleanup completion lost stable ownership")
	}
	if pending.OriginalCause() != origin || errors.Unwrap(pending) != exposed || pending.Error() != message {
		t.Fatal("cleanup completion changed the decoded-fact receipt")
	}
	if manager.Stats().LiveReferences != baseline {
		t.Fatal("decoded fact retry leaked roots", baseline, manager.Stats().LiveReferences)
	}
}

func TestDecodeFactInvalidSuffixReleasesDecodedPrefix(t *testing.T) {
	f := newTableRelationFixture(t)
	baseline := f.ast.Stats().LiveReferences
	refs, err := f.relations.DecodeFact([]ast.ASTHandle{f.boolean, f.boolean}, Fact{1, 2})
	if !errors.Is(err, ErrTable) || refs != nil {
		t.Fatal("invalid suffix published decoded prefix", refs, err)
	}
	if f.ast.Stats().LiveReferences != baseline {
		t.Fatal("decoded prefix leaked")
	}
	refs, err = f.relations.DecodeFact([]ast.ASTHandle{f.boolean, f.boolean}, Fact{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	for i := range refs {
		if err = f.ast.DelRef(&refs[i]); err != nil {
			t.Fatal(err)
		}
	}
	if f.ast.Stats().LiveReferences != baseline {
		t.Fatal("decode retry leaked")
	}
}
