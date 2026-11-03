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

func TestExternalIntervalDatalogRegistersLoadStoreAndRepeat(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-register-p-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-register-q-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			seven, err := manager.ArithmeticIntegerNumeralText("7")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{seven}))
			activation := engine.externalIntervalDatalog
			if activation == nil {
				t.Fatal("fact did not construct product activation")
			}

			// Validate the complete register set before the first callback.
			callbacks.operations = nil
			_, err = activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{p, ast.ASTHandle{}})
			if !errors.Is(err, ErrExternalIntervalDatalogFragment) || len(callbacks.operations) != 0 {
				t.Fatal("register preflight invoked callbacks or accepted an unknown predicate", err, callbacks.operations)
			}

			callbacks.operations = nil
			oldP, oldQ := activation.entries[p].product, activation.entries[q].product
			registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{p, q, p})
			fragmentMust(t, err)
			if len(registers.slots) != 2 {
				t.Fatal("duplicate predicate allocated another register", len(registers.slots))
			}
			loadedP, err := registers.relation(p)
			fragmentMust(t, err)
			loadedQ, err := registers.relation(q)
			fragmentMust(t, err)
			if loadedP == nil || loadedP == oldP || loadedQ != nil || activation.entries[p].product != oldP || activation.entries[q].product != oldQ {
				t.Fatal("LOAD did not clone only the nonempty persistent product")
			}
			wantLoad := []int32{ast.DatalogOpIsEmpty, ast.DatalogOpClone}
			if kinds[0] == "external_relation" {
				wantLoad = []int32{ast.DatalogOpIsEmpty, ast.DatalogOpClone, ast.DatalogOpIsEmpty}
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, wantLoad)

			fragmentMust(t, registers.store())
			wantStore := append(append([]int32(nil), wantLoad...), ast.DatalogOpEmpty)
			assertExternalIntervalRegisterOperations(t, callbacks.operations, wantStore)
			if activation.entries[p].product != loadedP || activation.entries[p].product == oldP || activation.entries[q].product == oldQ {
				t.Fatal("STORE did not move the clone and fresh empty product into activation")
			}
			if _, err = oldP.Empty(); !errors.Is(err, rel.ErrTableRelation) {
				t.Fatal("STORE did not retire the displaced nonempty product", err)
			}
			if _, err = oldQ.Empty(); !errors.Is(err, rel.ErrTableRelation) {
				t.Fatal("STORE did not retire the displaced empty product", err)
			}

			// A second execution loads the newly published products. Supplying q
			// first proves that callback order follows first encounter, not map order.
			callbacks.operations = nil
			second, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{q, p})
			fragmentMust(t, err)
			secondP, err := second.relation(p)
			fragmentMust(t, err)
			secondQ, err := second.relation(q)
			fragmentMust(t, err)
			if secondP == nil || secondP == loadedP || secondQ != nil {
				t.Fatal("repeated LOAD did not clone the moved snapshot independently")
			}
			wantSecondLoad := []int32{ast.DatalogOpIsEmpty, ast.DatalogOpIsEmpty, ast.DatalogOpClone}
			if kinds[0] != "external_relation" {
				wantSecondLoad = []int32{ast.DatalogOpIsEmpty, ast.DatalogOpClone}
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, wantSecondLoad)
			fragmentMust(t, second.store())
			wantSecondStore := append(append([]int32(nil), wantSecondLoad...), ast.DatalogOpEmpty)
			assertExternalIntervalRegisterOperations(t, callbacks.operations, wantSecondStore)
			if activation.entries[p].product != secondP {
				t.Fatal("repeated STORE did not publish the second independent clone")
			}
		})
	}
}

func TestExternalIntervalDatalogRegisterCloneRefusalPublishesNothing(t *testing.T) {
	for _, kinds := range [][]string{
		{"external_relation", "interval_relation"},
		{"interval_relation", "external_relation"},
	} {
		t.Run(kinds[0]+"-first", func(t *testing.T) {
			manager, engine, callbacks := newExternalIntervalDatalogFixture(t)
			family, err := manager.RegisterArithmeticPlugin()
			fragmentMust(t, err)
			integer, err := manager.MkFamilySort(family, 1, nil)
			fragmentMust(t, err)
			p := declareExternalIntervalPredicate(t, manager, engine, "external-interval-register-refuse-p-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			q := declareExternalIntervalPredicate(t, manager, engine, "external-interval-register-refuse-q-"+kinds[0], []ast.ASTHandle{integer}, kinds)
			one, err := manager.ArithmeticIntegerNumeralText("1")
			fragmentMust(t, err)
			two, err := manager.ArithmeticIntegerNumeralText("2")
			fragmentMust(t, err)
			fragmentMust(t, engine.AddFact(p, []ast.ASTHandle{one}))
			fragmentMust(t, engine.AddFact(q, []ast.ASTHandle{two}))
			activation := engine.externalIntervalDatalog
			oldP, oldQ := activation.entries[p].product, activation.entries[q].product
			refusal := errors.New("external register clone refused")
			callbacks.operations = nil
			callbacks.refusalOperation = ast.DatalogOpClone
			callbacks.refusalAt = 2
			callbacks.refusal = refusal
			registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{p, q})
			if registers != nil || !errors.Is(err, refusal) {
				t.Fatal("second CLONE refusal did not fail LOAD", registers, err)
			}
			assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
				ast.DatalogOpIsEmpty, ast.DatalogOpClone,
			})
			if activation.entries[p].product != oldP || activation.entries[q].product != oldQ {
				t.Fatal("failed LOAD published a register prefix")
			}
			for _, product := range []*rel.ExternalIntervalProduct{oldP, oldQ} {
				empty, productErr := product.Empty()
				fragmentMust(t, productErr)
				if empty {
					t.Fatal("failed LOAD changed a persistent product")
				}
			}

			callbacks.operations = nil
			callbacks.refusal = nil
			retry, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{p, q})
			fragmentMust(t, err)
			fragmentMust(t, retry.Close())
			if activation.entries[p].product != oldP || activation.entries[q].product != oldQ {
				t.Fatal("closing successful LOAD consumed persistent products")
			}
		})
	}
}

func TestExternalIntervalDatalogRegisterStorePublishesBeforeRetirementRetry(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var registers *externalIntervalDatalogRegisters
	var storeFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			storeFailure = registers.store()
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
	name, err := symbols.Intern("external-interval-register-retirement")
	must(err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	activation, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{{
		declaration: predicate,
		sorts:       []ast.ASTHandle{integer},
		kinds:       []string{"external_relation", "interval_relation"},
	}})
	must(err)
	value, err := manager.ArithmeticIntegerNumeralText("13")
	must(err)
	must(activation.addFact(predicate, []ast.ASTHandle{value}))
	old := activation.entries[predicate].product
	registers, err = activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{predicate})
	must(err)
	loaded, err := registers.relation(predicate)
	must(err)
	if loaded == nil || loaded == old {
		t.Fatal("retirement fixture did not load an independent clone")
	}
	callbacksBeforeStore := len(callbacks.operations)

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
	if body == nil || !fired || storeFailure == nil {
		t.Fatal("native retirement fixture did not invoke STORE", body, fired, storeFailure)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(storeFailure, &receipt) || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("STORE retirement refusal lost typed retry owner", storeFailure)
	}
	if activation.entries[predicate].product != loaded || len(callbacks.operations) != callbacksBeforeStore {
		t.Fatal("retirement refusal failed to publish once or replayed a callback", callbacks.operations)
	}
	empty, err := activation.entries[predicate].product.Empty()
	must(err)
	if empty {
		t.Fatal("published replacement lost the cloned nonempty state")
	}
	callbacksAfterProbe := len(callbacks.operations)
	shown, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil || !registers.closed || len(callbacks.operations) != callbacksAfterProbe {
		t.Fatal("retirement retry changed receipt identity or left register owner open")
	}
	if _, err = old.Empty(); !errors.Is(err, rel.ErrTableRelation) {
		t.Fatal("retirement retry left the displaced product usable", err)
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
		t.Fatal("external interval register retirement fixture leaked bytes", used)
	}
}

func assertExternalIntervalRegisterOperations(t *testing.T, got, want []int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatal("external register callback count", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal("external register callback order", got, want)
		}
	}
}
