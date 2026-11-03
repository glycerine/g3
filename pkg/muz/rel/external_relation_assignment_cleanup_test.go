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

type externalAssignmentTestChild struct {
	runtime *memory.Runtime
	cause   error
	cleaned int
	refuse  bool
}

func (f *externalAssignmentTestChild) Error() string                 { return f.cause.Error() }
func (f *externalAssignmentTestChild) Unwrap() error                 { return f.cause }
func (f *externalAssignmentTestChild) OriginalCause() error          { return f.cause }
func (f *externalAssignmentTestChild) CleanupCause() error           { return nil }
func (f *externalAssignmentTestChild) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *externalAssignmentTestChild) Cleanup() error {
	f.cleaned++
	if f.refuse && f.cleaned == 1 {
		return ast.ErrASTPending
	}
	return nil
}

var _ memory.ConstructionFailure = (*externalAssignmentTestChild)(nil)

func TestExternalAssignmentCleanupRefusalKeepsRetryOwner(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var refuse func() error
	var nativeFailure error
	active, refusing := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !refusing && nativeFailure == nil {
			refusing = true
			nativeFailure = refuse()
			refusing = false
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
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	relations, err := NewRelationManager(manager)
	must(err)
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))

	var targetHandle, sourceHandle, deltaHandle ast.ASTHandle
	var callbackCause error
	callbackKind, callbackCount := int32(-1), 0
	plugin, err := NewExternalRelationPlugin(relations, ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
		kind, cause := manager.GetDeclKind(declaration)
		if cause != nil {
			return cause
		}
		if kind != ast.DatalogOpUnion && kind != ast.DatalogOpWiden {
			return nil
		}
		callbackKind, callbackCount = kind, callbackCount+1
		if len(arguments) != 2 || len(outputs) != 2 || arguments[0] != targetHandle || arguments[1] != sourceHandle || outputs[0] != targetHandle || outputs[1] != deltaHandle {
			t.Fatal("external assignment callback shape", arguments, outputs)
		}
		return callbackCause
	}})
	must(err)
	realSort, err := manager.MkFamilySort(family, 0, nil)
	must(err)
	target, err := plugin.NewEmpty([]ast.ASTHandle{realSort})
	must(err)
	source, err := plugin.NewEmpty([]ast.ASTHandle{realSort})
	must(err)
	delta, err := plugin.NewEmpty([]ast.ASTHandle{realSort})
	must(err)
	targetHandle, err = target.Relation()
	must(err)
	sourceHandle, err = source.Relation()
	must(err)
	deltaHandle, err = delta.Relation()
	must(err)

	trigger := func(widen bool, child *externalAssignmentTestChild) (*externalAssignmentCleanupFailure, memory.ConstructionFailure) {
		t.Helper()
		callbackCause = child
		failure := target.Union(source, delta, widen)
		if !errors.Is(failure, ast.ErrASTPending) || !errors.Is(failure, child.cause) {
			t.Fatal("callback refusal did not reach UNION/WIDEN finalizer", failure)
		}
		var receipt memory.ConstructionFailure
		if !errors.As(failure, &receipt) {
			t.Fatal("external assignment refusal lost retry owner", failure, callbackKind, callbackCount)
		}
		owned, ok := receipt.(*externalAssignmentCleanupFailure)
		if !ok || owned.RuntimeOwner() != runtime || len(owned.attempt.roots) != 1 {
			t.Fatal("external assignment receipt lost app-root frontier", owned)
		}
		return owned, receipt
	}
	checkRetry := func(owned *externalAssignmentCleanupFailure, receipt memory.ConstructionFailure, child *externalAssignmentTestChild) {
		t.Helper()
		message, exposed := receipt.Error(), receipt.Unwrap()
		must(receipt.Cleanup())
		if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || len(owned.attempt.roots) != 0 || child != nil && child.cleaned != 2 {
			t.Fatal("external assignment retry changed receipt or retained app root")
		}
		must(receipt.Cleanup())
		if child != nil && child.cleaned != 2 {
			t.Fatal("external assignment retried a consumed child", child.cleaned)
		}
	}

	unionCause := errors.New("external UNION callback failure")
	child := &externalAssignmentTestChild{runtime: runtime, cause: unionCause, refuse: true}
	owned, receipt := trigger(false, child)
	if callbackKind != ast.DatalogOpUnion || callbackCount != 1 || owned.attempt.nested != child || child.cleaned != 1 || len(owned.attempt.roots) != 1 {
		t.Fatal("UNION production callback/finalizer not exercised", callbackKind, callbackCount, owned.attempt.nested)
	}
	checkRetry(owned, receipt, child)
	widenCause := errors.New("external WIDEN callback failure")
	child = &externalAssignmentTestChild{runtime: runtime, cause: widenCause, refuse: true}
	owned, receipt = trigger(true, child)
	if callbackKind != ast.DatalogOpWiden || callbackCount != 2 || owned.attempt.nested != child || child.cleaned != 1 || len(owned.attempt.roots) != 1 {
		t.Fatal("WIDEN production callback/finalizer not exercised", callbackKind, callbackCount, owned.attempt.nested)
	}
	checkRetry(owned, receipt, child)

	// Exercise the native zero-aware root frontier separately. An AST factory
	// invocation cannot enter Union far enough to reach ReduceAssign, so this
	// phase invokes the same production finalizer while numeral cleanup is
	// guarded and verifies that a consumed empty slot is never revisited.
	numericHandle, err := manager.ArithmeticNumeral(&number, false)
	must(err)
	numericRef, err := manager.NewRef(numericHandle)
	must(err)
	nativeAttempt := &externalAssignmentAttempt{manager: manager, roots: []ast.ASTRef{numericRef, {}}}
	refuse = func() error { return finishExternalAssignment(nativeAttempt, nil) }
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	active = true
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !errors.Is(nativeFailure, ast.ErrASTPending) {
		t.Fatal("native assignment finalizer refusal was not reached", body, nativeFailure)
	}
	if !errors.As(nativeFailure, &receipt) {
		t.Fatal("native assignment finalizer lost retry receipt", nativeFailure)
	}
	owned, ok := receipt.(*externalAssignmentCleanupFailure)
	if !ok || len(owned.attempt.roots) != 1 || owned.attempt.roots[0] == (ast.ASTRef{}) {
		t.Fatalf("native assignment finalizer lost live root frontier: type=%T roots=%#v", receipt, owned.attempt.roots)
	}
	checkRetry(owned, receipt, nil)
	for _, relation := range []*ExternalRelation{target, source, delta} {
		if _, err = relation.Relation(); err != nil {
			t.Fatal("assignment cleanup invalidated live relation", err)
		}
	}
	must(delta.Close())
	must(source.Close())
	must(target.Close())
	must(plugin.Close())
	must(relations.Close())
	must(numbers.Del(&number))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("external assignment cleanup fixture leaked bytes", used)
	}
}
