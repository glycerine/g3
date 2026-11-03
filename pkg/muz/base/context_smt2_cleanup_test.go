package base

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A real arithmetic allocation transport runs while the AST manager refuses
// reentry. Display cleanup must export both its current roots and any nested
// cleanup owner, then finish them once the native invocation has returned.
func TestContextSMT2OwnerNativePendingCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var manager *ast.ASTManager
	var outer, inner *contextSMT2Owner
	var pending memory.ConstructionFailure
	active, called, intact := false, false, true
	var observed []error
	original := errors.New("context display operand failed")
	nestedOriginal := errors.New("context display nested operand failed")
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if !active || called {
			return f
		}
		called = true
		outerRoots := append([]ast.ASTRef(nil), outer.refs...)
		innerRoots := append([]ast.ASTRef(nil), inner.refs...)
		nested := inner.finish(nestedOriginal)
		failure := outer.finish(errors.Join(original, nested))
		observed = append(observed, failure)
		intact = intact && errors.Is(failure, original) && errors.Is(failure, nestedOriginal) && errors.Is(failure, ast.ErrASTPending)
		if !errors.As(failure, &pending) {
			intact = false
			return f
		}
		for i := 0; i < 2; i++ {
			failure = pending.Cleanup()
			observed = append(observed, failure)
			intact = intact && errors.Is(failure, ast.ErrASTPending) && reflect.DeepEqual(outer.refs, outerRoots) && reflect.DeepEqual(inner.refs, innerRoots)
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
		active = false
		must(runtime.SetMaxCount(0))
		if pending != nil {
			must(pending.Cleanup())
		}
		if inner != nil {
			must(inner.finish(nil))
		}
		if outer != nil {
			must(outer.finish(nil))
		}
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Errorf("context display cleanup leaked %d bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	baseline := manager.Stats()
	outer = &contextSMT2Owner{manager: manager}
	inner = &contextSMT2Owner{manager: manager}
	for _, item := range []struct {
		owner *contextSMT2Owner
		name  string
	}{{outer, "context-display-rule"}, {outer, "context-display-query"}, {inner, "context-display-inner"}} {
		ref, err := item.owner.reserve()
		must(err)
		constant, err := manager.MkFreshConst(item.name, boolean)
		must(err)
		must(manager.SetRef(ref, constant))
	}
	withOwners := manager.Stats()
	if withOwners.LiveNodes <= baseline.LiveNodes || withOwners.LiveReferences != baseline.LiveReferences+3 {
		t.Fatal("fixture did not construct independent display roots", baseline, withOwners)
	}
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !called || pending == nil || !intact {
		t.Fatalf("native cleanup did not preserve display owners: body=%v callback=%t intact=%t observed=%v", body, called, intact, observed)
	}
	if pending.RuntimeOwner() != runtime || pending.CleanupCause() == nil {
		t.Fatal("context display receipt lost its runtime or cleanup cause")
	}
	origin, exposed, message := pending.OriginalCause(), errors.Unwrap(pending), pending.Error()
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.CleanupCause() != nil || pending.RuntimeOwner() != runtime || pending.OriginalCause() != origin || errors.Unwrap(pending) != exposed || pending.Error() != message {
		t.Fatal("cleanup completion changed the stable display receipt")
	}
	if len(outer.refs) != 0 || len(inner.refs) != 0 {
		t.Fatal("cleanup left reachable display roots", len(outer.refs), len(inner.refs))
	}
	after := manager.Stats()
	if after.LiveNodes != baseline.LiveNodes || after.LiveInfos != baseline.LiveInfos || after.LiveParameters != baseline.LiveParameters || after.LiveReferences != baseline.LiveReferences {
		t.Fatalf("display cleanup did not restore AST ownership: nodes %d/%d infos %d/%d parameters %d/%d refs %d/%d", after.LiveNodes, baseline.LiveNodes, after.LiveInfos, baseline.LiveInfos, after.LiveParameters, baseline.LiveParameters, after.LiveReferences, baseline.LiveReferences)
	}
}
