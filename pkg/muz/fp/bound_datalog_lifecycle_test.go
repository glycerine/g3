package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestBoundDatalogLifecycleSynchronizesAndClonesSeeds(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		fragmentMust(t, engine.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("bound-lifecycle-p", []ast.ASTHandle{integer})
	runtime, selected, err := newBoundDatalogActivation(m, engine.set, []ast.ASTHandle{p})
	fragmentMust(t, err)
	if !selected {
		t.Fatal("all-arithmetic program was not selected")
	}
	defer func() { fragmentMust(t, runtime.close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	fragmentMust(t, err)
	fragmentMust(t, runtime.addFact(p, []ast.ASTHandle{one}))
	status, err := runtime.query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("seeded bound predicate was not imprecise", status)
	}

	// A rejected complete list must not discard either seeds or the last
	// successfully published snapshot.
	finite := declare("bound-lifecycle-finite", []ast.ASTHandle{boolean})
	if cause := runtime.syncPredicates([]ast.ASTHandle{p, finite}); !errors.Is(cause, ErrBoundDatalogFragment) {
		t.Fatal("mixed late signature was accepted", cause)
	}
	if snapshot, cause := runtime.relation(p); cause != nil || snapshot.Empty() {
		t.Fatal("failed synchronization mutated the published result", snapshot, cause)
	}

	late := declare("bound-lifecycle-late", []ast.ASTHandle{integer})
	synthetic := declare("bound-lifecycle-synthetic-query", nil)
	fragmentMust(t, runtime.syncPredicates([]ast.ASTHandle{p, late, synthetic}))
	if _, cause := runtime.relation(p); !errors.Is(cause, ErrBoundDatalogFragment) {
		t.Fatal("successful predicate extension left a stale snapshot", cause)
	}
	if runtime.entries[p].seed.Empty() {
		t.Fatal("predicate extension discarded an existing seed")
	}
	fragmentMust(t, runtime.addFact(synthetic, nil))

	destination, err := base.NewRuleSet(engine.rules)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, destination.Close()) }()
	clone, err := runtime.clone(destination)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, clone.close()) }()
	fragmentMust(t, runtime.reset())
	status, err = clone.query([]ast.ASTHandle{p, synthetic})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("clone did not preserve independent arithmetic and nullary seeds", status)
	}
	status, err = runtime.query([]ast.ASTHandle{p, synthetic})
	fragmentMust(t, err)
	if status != lbool.False {
		t.Fatal("reset source shared seed state with clone", status)
	}
	fragmentMust(t, clone.close())
	fragmentMust(t, runtime.addFact(p, []ast.ASTHandle{one}))
	status, err = runtime.query([]ast.ASTHandle{p})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("closing clone affected source ownership", status)
	}
}

func TestBoundDatalogSyncStagingFailureKeepsRuntimeRetryable(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	mk := func(text string) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		return declaration
	}
	p, q, late := mk("bound-stage-p"), mk("bound-stage-q"), mk("bound-stage-late")
	fragmentMust(t, engine.RegisterPredicate(p))
	fragmentMust(t, engine.RegisterPredicate(q))
	fragmentMust(t, engine.RegisterPredicate(late))
	runtime, err := newBoundDatalogRuntime(m, engine.set, []ast.ASTHandle{p})
	fragmentMust(t, err)
	defer func() { fragmentMust(t, runtime.close()) }()

	// Hold references through a fresh 64-cell AST-ref page, then leave exactly
	// one cell for the first staged predicate. The second stage is a real arena
	// refusal and exercises rollback before any runtime map mutation.
	var held []ast.ASTRef
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			fragmentMust(t, m.DelRef(&held[i]))
		}
	}()
	used := m.BudgetField.Used()
	for m.BudgetField.Used() == used {
		root, cause := m.NewRef(p)
		fragmentMust(t, cause)
		held = append(held, root)
	}
	for i := 0; i < 62; i++ {
		root, cause := m.NewRef(p)
		fragmentMust(t, cause)
		held = append(held, root)
	}
	m.BudgetField.FailAfter(0)
	cause := runtime.syncPredicates([]ast.ASTHandle{p, q, late})
	m.BudgetField.FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("predicate staging did not reach native reference refusal", cause)
	}
	if len(runtime.entries) != 1 || len(runtime.order) != 1 || len(runtime.pending) != 0 {
		t.Fatal("failed staged registration changed runtime or lost cleanup owner", len(runtime.entries), len(runtime.order), len(runtime.pending))
	}
	fragmentMust(t, runtime.syncPredicates([]ast.ASTHandle{p, q, late}))
	if len(runtime.entries) != 3 || len(runtime.order) != 3 {
		t.Fatal("runtime could not retry staged predicate registration")
	}
}

func TestBoundDatalogActivationSelectionAndCompleteListValidation(t *testing.T) {
	f := newDirectFactFixture(t)
	m := f.manager
	engine, err := NewPropositionalEngine(m)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	family, err := m.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	fragmentMust(t, err)
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	mk := func(text string, domain []ast.ASTHandle) ast.ASTHandle {
		name, cause := m.Symbols.Intern(text)
		fragmentMust(t, cause)
		declaration, cause := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, cause)
		return declaration
	}
	arithmetic := mk("bound-selection-arithmetic", []ast.ASTHandle{integer})
	finite := mk("bound-selection-finite", []ast.ASTHandle{boolean})
	if activation, selected, cause := newBoundDatalogActivation(m, engine.set, []ast.ASTHandle{arithmetic, finite}); cause != nil || selected || activation != nil {
		t.Fatal("mixed signature selected bound activation", activation, selected, cause)
	}
	nullary := mk("bound-selection-nullary", nil)
	activation, selected, err := newBoundDatalogActivation(m, engine.set, []ast.ASTHandle{nullary})
	fragmentMust(t, err)
	if !selected || activation == nil {
		t.Fatal("explicit nullary bound activation was declined")
	}
	defer func() { fragmentMust(t, activation.close()) }()
	if cause := activation.syncPredicates([]ast.ASTHandle{nullary, nullary}); !errors.Is(cause, ErrBoundDatalogFragment) {
		t.Fatal("duplicate complete predicate list was accepted", cause)
	}
	if len(activation.order) != 1 || len(activation.entries) != 1 {
		t.Fatal("failed complete-list validation partially mutated runtime")
	}
}

func TestBoundDatalogConstructionCleanupReceiptRetriesNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	active := false
	var receipt *boundDatalogCleanupReceipt
	var refused error
	native, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && receipt != nil && refused == nil {
			refused = receipt.Cleanup()
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: native, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(native.Close())
		if budget.Used() != 0 {
			t.Fatal("bound receipt fixture leaked bytes", budget.Used())
		}
	}()
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("bound-construction-receipt")
	must(err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	ruleManager, err := base.NewRuleManager(manager)
	must(err)
	defer func() { must(ruleManager.Close()) }()
	rules, err := base.NewRuleSet(ruleManager)
	must(err)
	defer func() { must(rules.Close()) }()
	owner, err := newBoundDatalogRuntime(manager, rules, []ast.ASTHandle{declaration})
	must(err)
	receipt = &boundDatalogCleanupReceipt{original: arena.ErrInjectedFailure, owner: manager.NumeralService().RuntimeOwner(), runtime: owner}
	var standard memory.ConstructionFailure = receipt
	if !errors.Is(standard.OriginalCause(), arena.ErrInjectedFailure) || standard.CleanupCause() != nil || standard.RuntimeOwner() != native {
		t.Fatal("construction receipt does not expose standard failure causes")
	}

	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(native.SetMaxCount(uint64(native.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(native.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !errors.Is(refused, ast.ErrASTPending) || receipt.runtime == nil || receipt.runtime.entries[declaration].root == (ast.ASTRef{}) {
		t.Fatal("native cleanup refusal lost construction owner", body, refused)
	}
	if !errors.Is(standard.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("construction receipt did not publish cleanup refusal", standard.CleanupCause())
	}
	must(receipt.Cleanup())
	if receipt.runtime != nil || standard.CleanupCause() != nil {
		t.Fatal("construction cleanup receipt did not retire on retry")
	}
}
