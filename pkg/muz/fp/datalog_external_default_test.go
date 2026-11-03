package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDatalogExternalDefaultRegistrationAndFiniteSeedMigration(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(globals.SourceSetName("fixedpoint.datalog.default_relation", "external_relation"))
	newEngine := func() *PropositionalEngine {
		e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
		must(err)
		must(e.SelectEngine("datalog"))
		return e
	}
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(e *PropositionalEngine, name string) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		must(cause)
		declaration, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}

	withoutPlugin := newEngine()
	p := declare(withoutPlugin, "external-default-without-plugin")
	selected, err := withoutPlugin.ensureDefaultExternalDatalog()
	must(err)
	if selected || withoutPlugin.predicates[p].external != nil || withoutPlugin.predicates[p].table == nil {
		t.Fatal("external name alone registered callback relation")
	}
	must(withoutPlugin.Close())

	e := newEngine()
	defer func() { must(e.Close()) }()
	p = declare(e, "external-default-with-plugin")
	truth, err := m.MkTrue()
	must(err)
	fact, err := e.relations.EncodeFact(e.predicates[p].sorts, []ast.ASTHandle{truth})
	must(err)
	_, err = e.predicates[p].table.Add(fact)
	must(err)
	nonempty := make(map[ast.ASTHandle]bool)
	stores := 0
	callbacks := rel.ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, arguments, outputs []ast.ASTHandle) error {
			kind, cause := m.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			switch kind {
			case ast.DatalogOpEmpty:
				nonempty[outputs[0]] = false
			case ast.DatalogOpStore:
				stores++
				nonempty[outputs[0]] = true
			}
			return nil
		},
		Reduce: func(_ ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			if nonempty[arguments[0]] {
				return m.MkFalse()
			}
			return m.MkTrue()
		},
	}
	must(e.SetExternalRelationCallbacks(callbacks))
	selected, err = e.ensureDefaultExternalDatalog()
	must(err)
	if !selected || stores != 1 || e.predicates[p].external == nil || e.predicates[p].table != nil {
		t.Fatal("external default did not migrate finite seed", selected, stores)
	}
	if status, cause := e.QueryPredicate(p); cause != nil || status != lbool.True {
		t.Fatal("migrated external seed query", status, cause)
	}

	explicit := newEngine()
	defer func() { must(explicit.Close()) }()
	explicitPredicate := declare(explicit, "external-default-explicit-finite")
	must(explicit.SetExternalRelationCallbacks(callbacks))
	must(explicit.SetPredicateRepresentation(explicitPredicate, []string{"tr_sparse"}))
	selected, err = explicit.ensureDefaultExternalDatalog()
	must(err)
	if selected || explicit.predicates[explicitPredicate].external != nil || explicit.predicates[explicitPredicate].table == nil {
		t.Fatal("external favourite overrode explicit finite representation")
	}
}

func TestExternalDefaultDecodedCleanupReceiptRetriesNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	active := false
	var owner *externalDefaultDecodedOwner
	var construction error
	native, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && construction == nil {
			construction = owner.finish(arena.ErrInjectedFailure)
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
			t.Fatal("external decoded receipt fixture leaked bytes", budget.Used())
		}
	}()
	must(manager.RegisterDeclarationPlugins())
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("external-default-decoded-cleanup")
	must(err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	root, err := manager.NewRef(declaration)
	must(err)
	owner = &externalDefaultDecodedOwner{manager: manager, refs: [][]ast.ASTRef{{root}}}

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
	var failure memory.ConstructionFailure
	if body == nil || !errors.As(construction, &failure) {
		t.Fatal("native decoded cleanup refusal did not return construction receipt", body, construction)
	}
	shown := construction.Error()
	if !errors.Is(failure.OriginalCause(), arena.ErrInjectedFailure) || !errors.Is(failure.CleanupCause(), ast.ErrASTPending) || failure.RuntimeOwner() != native {
		t.Fatal("decoded cleanup receipt did not separate operation and cleanup causes", failure.OriginalCause(), failure.CleanupCause())
	}
	if !errors.Is(construction, arena.ErrInjectedFailure) || !errors.Is(construction, ast.ErrASTPending) {
		t.Fatal("decoded cleanup receipt did not report both failures", construction)
	}
	must(failure.Cleanup())
	if failure.CleanupCause() != nil || construction.Error() != shown || !errors.Is(failure.OriginalCause(), arena.ErrInjectedFailure) {
		t.Fatal("decoded cleanup retry changed stable failure presentation", failure.CleanupCause(), construction)
	}
}
