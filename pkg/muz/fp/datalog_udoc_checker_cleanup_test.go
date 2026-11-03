package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDatalogDocCheckerScopeNativeCleanupKeepsFactoryUntilNestedRootsRelease(t *testing.T) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 28)
	services := &pdrLifecycleServices{}
	var scope *datalogCheckRelationScope
	var roots []ast.ASTRef
	var pending memory.ConstructionFailure
	var doc *rel.UDocNullaryTable
	var factory *rel.UDocRelationFactory
	var state *predicateState
	original := errors.New("DOC scope construction failed")
	active, called := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !called {
			called = true
			nested := closeASTRefs(services.manager, roots)
			roots = nil
			cause := errors.Join(original, nested)
			if !errors.As(cause, &scope.docNested) {
				t.Fatal("DOC scope lost native nested cleanup receipt", cause)
			}
			cleanup := scope.dispose()
			if !errors.Is(cleanup, ast.ErrASTPending) {
				t.Fatal("DOC scope did not reach native AST cleanup refusal", cleanup)
			}
			pending = &datalogDocScopeConstructionFailure{original: cause, cleanup: cleanup, scope: scope, reported: errors.Join(cause, cleanup)}
			for i := 0; i < 2; i++ {
				if err := pending.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
					t.Fatal("DOC scope retry changed native refusal", err)
				}
				if scope.docFactory != factory || scope.docNested == nil || scope.entries[0].doc != doc || state.table != doc {
					t.Fatal("nested refusal retired DOC factory, entry, or current state")
				}
				if supported, err := factory.CanHandleSignature(nil); err != nil || !supported {
					t.Fatal("DOC factory closed before nested cleanup completed", supported, err)
				}
				if _, err := doc.Signature(); err != nil {
					t.Fatal("DOC entry closed before nested cleanup completed", err)
				}
			}
		}
		return value
	}})
	must(err)
	services.numerals, err = numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	services.symbols, err = symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	services.params, err = uparams.NewParamsContext(services.symbols, services.numerals, uparams.ParamsConfig{Budget: budget})
	must(err)
	services.catalog, err = uparams.NewGlobalParamsCatalog(services.symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	services.globals, err = uparams.NewGlobalParamsService(services.symbols, services.params, uparams.GlobalParamsConfig{Budget: budget, Catalog: services.catalog})
	must(err)
	must(services.globals.InitializeSource())
	services.manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: services.symbols, Numerals: services.numerals, Budget: budget})
	must(err)
	defer func() {
		must(services.Close())
		must(runtime.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("DOC scope native cleanup fixture leaked bytes", used)
		}
	}()
	m := services.manager
	must(m.RegisterDeclarationPlugins())
	engine, err := NewPropositionalEngineWithServices(m, services.params, services.globals)
	must(err)
	defer func() { must(engine.Close()) }()
	must(engine.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("doc-scope-native-cleanup-p")
	must(err)
	declaration, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(declaration))
	must(engine.AddFact(declaration, nil))
	state = engine.predicates[declaration]
	originalTable := state.table
	if originalTable == nil {
		t.Fatal("fixture did not start with persistent finite storage")
	}
	guard, err := services.numerals.NewValue()
	must(err)
	defer func() { must(services.numerals.Del(&guard)) }()
	must(services.numerals.SetString(&guard, "18446744073709551617/18446744073709551619"))
	_, err = m.ArithmeticNumeral(&guard, false)
	must(err)
	baseline := m.Stats().LiveReferences
	scope, err = engine.beginDatalogDocNullaries()
	must(err)
	defer func() { must(scope.dispose()) }()
	if len(scope.entries) != 1 || scope.docFactory == nil || !scope.docFactory.FormulaChecked() {
		t.Fatal("fixture did not create an actual checked DOC scope")
	}
	factory, doc = scope.docFactory, scope.entries[0].doc
	if doc == nil || state.table != doc || scope.entries[0].original != originalTable {
		t.Fatal("checked DOC scope did not exchange the original table")
	}
	if found, err := doc.Contains(nil); err != nil || !found {
		t.Fatal("checked DOC scope did not import nullary truth", found, err)
	}
	root, err := m.NewRef(declaration)
	must(err)
	roots = []ast.ASTRef{root}

	// The arithmetic factory's native guard makes real AST releases refuse.
	// Only cleanup runs inside its transport; no fake relation or owner is used.
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&guard, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !called || pending == nil {
		t.Fatal("native guard did not reach DOC scope cleanup", body, called)
	}
	if pending.RuntimeOwner() != runtime || !errors.Is(pending.OriginalCause(), original) || pending.CleanupCause() == nil {
		t.Fatal("DOC scope receipt lost runtime or separate causes")
	}
	if _, err = m.RefGet(root); err != nil {
		t.Fatal("native refusal consumed the nested root", err)
	}
	shown, exposed, origin := pending.Error(), pending.Unwrap(), pending.OriginalCause()
	must(pending.Cleanup())
	must(pending.Cleanup())
	if pending.Error() != shown || pending.Unwrap() != exposed || pending.OriginalCause() != origin || pending.CleanupCause() != nil || pending.RuntimeOwner() != runtime {
		t.Fatal("DOC scope cleanup changed its stable receipt")
	}
	if scope.docNested != nil || scope.docFactory != nil || scope.entries[0].doc != nil || state.table != originalTable {
		t.Fatal("DOC scope retry left owners or failed to restore original storage")
	}
	if _, err = factory.CanHandleSignature(nil); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("DOC factory survived completed scope cleanup", err)
	}
	if _, err = doc.Signature(); !errors.Is(err, rel.ErrDOC) {
		t.Fatal("DOC relation survived completed scope cleanup", err)
	}
	if _, err = m.RefGet(root); err == nil {
		t.Fatal("DOC scope retry left its nested root live")
	}
	if found, err := originalTable.Contains(nil); err != nil || !found {
		t.Fatal("DOC scope retry changed persistent nullary truth", found, err)
	}
	if m.Stats().LiveReferences != baseline {
		t.Fatal("DOC scope retry leaked AST roots", baseline, m.Stats().LiveReferences)
	}
}
