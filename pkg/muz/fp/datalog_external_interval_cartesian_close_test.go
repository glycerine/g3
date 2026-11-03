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

func TestExternalIntervalDatalogCartesianJoinedCloseBeforePublicationRetry(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var continuation *externalIntervalDatalogCartesianContinuation
	var closeFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			closeFailure = continuation.resume()
		}
		return failure
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
	must(manager.RegisterDeclarationPlugins())
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	declare := func(label string, sorts []ast.ASTHandle) ast.ASTHandle {
		name, cause := symbols.Intern("external-interval-cartesian-close-" + label)
		must(cause)
		declaration, cause := manager.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		return declaration
	}
	p := declare("p", []ast.ASTHandle{integer})
	q := declare("q", []ast.ASTHandle{integer})
	h := declare("h", []ast.ASTHandle{integer, integer})
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	kinds := []string{"external_relation", "interval_relation"}
	activation, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{
		{declaration: h, sorts: []ast.ASTHandle{integer, integer}, kinds: kinds},
		{declaration: p, sorts: []ast.ASTHandle{integer}, kinds: kinds},
		{declaration: q, sorts: []ast.ASTHandle{integer}, kinds: kinds},
	})
	must(err)
	must(activation.installExternalIntervalRulePlans(nil, nil, &externalIntervalDatalogCartesianRule{head: h, left: p, right: q}))
	one, err := manager.ArithmeticIntegerNumeralText("1")
	must(err)
	must(activation.addFact(p, []ast.ASTHandle{one}))
	two, err := manager.ArithmeticIntegerNumeralText("2")
	must(err)
	must(activation.addFact(q, []ast.ASTHandle{two}))

	oldHead := activation.entries[h].product
	registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{h, p, q})
	must(err)
	left, err := registers.relation(p)
	must(err)
	right, err := registers.relation(q)
	must(err)
	joined, err := rel.JoinExternalIntervalProducts(left, right, nil, nil)
	must(err)
	target, err := registers.ensureRelation(h)
	must(err)
	must(target.Union(joined, nil, false))
	continuation = &externalIntervalDatalogCartesianContinuation{
		registers: registers,
		temporary: joined,
		runtime:   runtime,
		publish:   true,
	}
	callbacksBeforeClose := len(callbacks.operations)

	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !fired || !errors.Is(closeFailure, ast.ErrASTPending) || continuation.registers != registers || registers.closed || registers.published || continuation.temporary != joined {
		t.Fatal("native Cartesian joined close refusal was not preserved", body, fired, closeFailure)
	}
	if activation.entries[h].product != oldHead || len(callbacks.operations) != callbacksBeforeClose {
		t.Fatal("Cartesian joined close refusal reached STORE or replayed callbacks", callbacks.operations)
	}
	if empty, cause := oldHead.Empty(); cause != nil || !empty {
		t.Fatal("Cartesian joined close refusal replaced the original empty head", empty, cause)
	}
	callbacksBeforeClose = len(callbacks.operations)

	pending := newExternalIntervalDatalogCartesianFailure(closeFailure, continuation)
	activation.pending = pending
	activation.pendingCompletesRule = true
	activation.pendingCartesian = activation.cartesian
	var receipt memory.ConstructionFailure
	if !errors.As(pending, &receipt) || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("Cartesian joined close refusal lost typed continuation", pending)
	}
	shown, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(activation.executeExternalIntervalIdentity())
	if activation.pending != nil || activation.pendingCompletesRule || activation.pendingCartesian != nil || continuation.temporary != nil || continuation.registers != nil || !registers.closed {
		t.Fatal("Cartesian continuation retry did not finish joined close")
	}
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil {
		t.Fatal("Cartesian continuation retry changed receipt identity")
	}
	if activation.entries[h].product != target || len(callbacks.operations) != callbacksBeforeClose ||
		operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("Cartesian continuation retry replayed or replaced the committed rule", callbacks.operations)
	}
	if _, err = oldHead.Empty(); !errors.Is(err, rel.ErrTableRelation) {
		t.Fatal("Cartesian STORE did not retire the displaced head after DEALLOC", err)
	}
	if _, err = joined.Empty(); !errors.Is(err, rel.ErrTableRelation) {
		t.Fatal("Cartesian continuation STORE preceded joined DEALLOC", err)
	}
	must(receipt.Cleanup())
	if len(callbacks.operations) != callbacksBeforeClose || operationCount(callbacks.operations, ast.DatalogOpJoin) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("completed Cartesian cleanup replayed callbacks", callbacks.operations)
	}

	empty, err := target.Empty()
	must(err)
	if empty {
		t.Fatal("published Cartesian union target became empty")
	}
	must(numbers.Del(&number))
	must(activation.close())
	must(plugin.Close())
	must(relations.Close())
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("Cartesian joined close fixture leaked bytes", used)
	}
}

func TestExternalIntervalDatalogCartesianReplacementCloseRefusalOwnsBothProducts(t *testing.T) {
	for _, operation := range []string{"project", "rename"} {
		t.Run(operation, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			budget := arena.NewBudget(1 << 28)
			var continuation *externalIntervalDatalogCartesianContinuation
			var successor *rel.ExternalIntervalProduct
			var closeFailure error
			active, fired := false, false
			runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
				if active && !fired {
					fired = true
					closeFailure = continuation.replaceTemporary(successor)
				}
				return failure
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
			must(manager.RegisterDeclarationPlugins())
			family, err := manager.RegisterArithmeticPlugin()
			must(err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			must(err)
			relations, err := rel.NewRelationManager(manager)
			must(err)
			callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
			plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
			must(err)
			original, selected, err := rel.NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{integer, integer}, []string{"external_relation", "interval_relation"})
			must(err)
			if !selected {
				t.Fatal("replacement fixture product was not selected")
			}
			one, err := manager.ArithmeticIntegerNumeralText("1")
			must(err)
			two, err := manager.ArithmeticIntegerNumeralText("2")
			must(err)
			must(original.AddFact([]ast.ASTHandle{one, two}))
			predecessor, err := original.Clone()
			must(err)
			if operation == "project" {
				successor, err = predecessor.Project([]uint32{0})
			} else {
				successor, err = predecessor.Rename([]uint32{0, 1})
			}
			must(err)
			continuation = &externalIntervalDatalogCartesianContinuation{temporary: predecessor, runtime: runtime}
			callbacksBeforeClose := len(callbacks.operations)

			number, err := numbers.NewValue()
			must(err)
			must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
			must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
			var invocation ast.ASTArithmeticNumeralInvocation
			active = true
			_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
			active = false
			must(runtime.SetMaxCount(0))
			must(manager.CleanupArithmeticNumeralInvocation(&invocation))
			if body == nil || !fired || !errors.Is(closeFailure, ast.ErrASTPending) || continuation.retiring != predecessor || continuation.temporary != successor {
				t.Fatal("replacement refusal lost either temporary owner", body, fired, closeFailure)
			}
			if len(callbacks.operations) != callbacksBeforeClose {
				t.Fatal("replacement close refusal replayed callback", callbacks.operations)
			}
			if _, err = predecessor.Sorts(); err != nil {
				t.Fatal("replacement refusal closed predecessor early", err)
			}
			if _, err = successor.Sorts(); err != nil {
				t.Fatal("replacement refusal lost constructed successor", err)
			}
			receipt := newExternalIntervalDatalogCartesianFailure(closeFailure, continuation)
			shown, exposed, originalCause := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
			must(receipt.Cleanup())
			must(receipt.Cleanup())
			if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != originalCause || receipt.CleanupCause() != nil || continuation.temporary != nil || continuation.retiring != nil {
				t.Fatal("replacement cleanup changed receipt or left temporary ownership")
			}
			if _, err = predecessor.Sorts(); !errors.Is(err, rel.ErrTableRelation) {
				t.Fatal("replacement cleanup did not close predecessor", err)
			}
			if _, err = successor.Sorts(); !errors.Is(err, rel.ErrTableRelation) {
				t.Fatal("replacement cleanup did not close successor", err)
			}
			if len(callbacks.operations) != callbacksBeforeClose {
				t.Fatal("replacement cleanup replayed transform", callbacks.operations)
			}
			if empty, cause := original.Empty(); cause != nil || empty {
				t.Fatal("replacement cleanup damaged borrowed source", empty, cause)
			}
			must(numbers.Del(&number))
			must(original.Close())
			must(plugin.Close())
			must(relations.Close())
			must(manager.Close())
			must(numbers.Close())
			must(symbols.Close())
			must(runtime.Close())
			if used := budget.Used(); used != 0 {
				t.Fatal("replacement cleanup leaked bytes", used)
			}
		})
	}
}
