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

func TestExternalIntervalDatalogProgramGroundCacheCloseAfterStoreNativeRetry(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var activation *externalIntervalDatalogActivation
	var cleanupFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			cleanupFailure = activation.executeExternalIntervalProgram()
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
	declare := func(label string) ast.ASTHandle {
		name, cause := symbols.Intern("external-interval-program-ground-cache-" + label)
		must(cause)
		declaration, cause := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		return declaration
	}
	p, q := declare("p"), declare("q")
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	kinds := []string{"external_relation", "interval_relation"}
	activation, err = newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{
		{declaration: p, sorts: []ast.ASTHandle{integer}, kinds: kinds},
		{declaration: q, sorts: []ast.ASTHandle{integer}, kinds: kinds},
	})
	must(err)
	seven, err := manager.ArithmeticIntegerNumeralText("7")
	must(err)
	ground := func(head ast.ASTHandle) *externalIntervalDatalogGroundRule {
		application, cause := manager.MkApp(head, []ast.ASTHandle{seven})
		must(cause)
		return &externalIntervalDatalogGroundRule{head: head, application: application, values: []ast.ASTHandle{seven}}
	}
	program := &externalIntervalDatalogProgram{
		rules:        []externalIntervalDatalogProgramRule{{ground: ground(p)}, {ground: ground(q)}},
		declarations: []ast.ASTHandle{p, q}, schedule: []int{0, 1},
	}
	must(activation.installExternalIntervalProgram(program))
	oldP, oldQ := activation.entries[p].product, activation.entries[q].product
	callbacks.operations = nil
	registers, err := activation.loadExternalIntervalDatalogRegisters(program.declarations)
	must(err)
	registers.deferStore = true
	program.registers = registers
	for _, plan := range program.rules {
		activation.ground = plan.ground
		must(activation.executeExternalIntervalIdentity())
		program.next++
	}
	cache := registers.constants
	if cache == nil || len(cache.owned) != 1 || len(cache.unary) != 1 {
		t.Fatal("ground rules did not share one owned singleton")
	}
	singleton := cache.owned[0]
	if activation.entries[p].product != oldP || activation.entries[q].product != oldQ {
		t.Fatal("ground rule published before final STORE")
	}
	if operationCount(callbacks.operations, ast.DatalogOpStore) != 1 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 2 {
		t.Fatal("ground program did not initialize once and evaluate both rules", callbacks.operations)
	}
	callbacksBeforeRetry := len(callbacks.operations)
	number, err := numbers.NewValue()
	must(err)
	interrupt := func(value string) {
		t.Helper()
		must(numbers.SetString(&number, value))
		must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
		var invocation ast.ASTArithmeticNumeralInvocation
		active, fired, cleanupFailure = true, false, nil
		_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
		active = false
		must(runtime.SetMaxCount(0))
		must(manager.CleanupArithmeticNumeralInvocation(&invocation))
		if body == nil || !fired || !errors.Is(cleanupFailure, ast.ErrASTPending) {
			t.Fatal("native program cleanup refusal was not preserved", body, fired, cleanupFailure)
		}
	}
	interrupt("18446744073709551617/18446744073709551619")
	if !registers.published || registers.closed || !program.storing || activation.pending == nil || cache.next != 0 {
		t.Fatal("final STORE did not preserve pending retirement and cache ownership")
	}
	publishedP, publishedQ := activation.entries[p].product, activation.entries[q].product
	if publishedP == oldP || publishedQ == oldQ {
		t.Fatal("final STORE did not publish both ground results")
	}
	receipt := activation.pending
	shown, exposed, original := receipt.Error(), errors.Unwrap(receipt), receipt.OriginalCause()
	if receipt.RuntimeOwner() != runtime {
		t.Fatal("program cache receipt has the wrong runtime")
	}

	// Advance only the displaced-product frontier outside the native guard.
	// The same published bank and its real pending receipt still own the cache;
	// the next production resume must stop specifically at cached-product Close.
	for registers.retireNext >= 0 {
		must(registers.retired[registers.retireNext].Close())
		registers.retired[registers.retireNext] = nil
		registers.retireNext--
	}
	interrupt("18446744073709551621/18446744073709551623")
	if activation.pending != receipt || registers.closed || !program.storing || registers.constants != cache || cache.next != 0 || cache.owned[0] != singleton {
		t.Fatal("native shared-cache refusal lost its owner or completed STORE early")
	}
	if activation.entries[p].product != publishedP || activation.entries[q].product != publishedQ || len(callbacks.operations) != callbacksBeforeRetry {
		t.Fatal("cache cleanup refusal replaced published results or replayed callbacks", callbacks.operations)
	}
	must(activation.executeExternalIntervalProgram())
	if activation.pending != nil || program.registers != nil || program.storing || program.next != 0 || !registers.closed || registers.constants != nil || cache.next != 1 || cache.owned[0] != nil || cache.unary != nil {
		t.Fatal("program retry did not release the shared cache and finish STORE")
	}
	if receipt.Error() != shown || errors.Unwrap(receipt) != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil {
		t.Fatal("shared-cache retry changed receipt identity")
	}
	must(receipt.Cleanup())
	if activation.entries[p].product != publishedP || activation.entries[q].product != publishedQ || len(callbacks.operations) != callbacksBeforeRetry {
		t.Fatal("completed cache cleanup replayed rules or final STORE", callbacks.operations)
	}
	for _, product := range []*rel.ExternalIntervalProduct{oldP, oldQ, singleton} {
		if _, cause := product.Empty(); !errors.Is(cause, rel.ErrTableRelation) {
			t.Fatal("ground program left a displaced or cached product open", cause)
		}
	}
	for _, product := range []*rel.ExternalIntervalProduct{publishedP, publishedQ} {
		empty, cause := product.Empty()
		must(cause)
		if empty {
			t.Fatal("cache retirement changed a published ground result")
		}
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
		t.Fatal("program ground cache fixture leaked bytes", used)
	}
}
