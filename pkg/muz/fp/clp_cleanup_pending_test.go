package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestCLPBackendAttemptPreservesRefusedCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 29)
	var attempt *clpBackendAttempt
	var failure error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			failure = attempt.dispose()
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	root, err := NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if attempt != nil {
			must(attempt.close())
		}
		must(root.Close())
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("CLP cleanup fixture leaked bytes", budget.Used())
		}
	}()
	must(root.SelectEngine("clp"))
	boolean, err := manager.MkBoolSort()
	must(err)
	declare := func(label string) (ast.ASTHandle, ast.ASTHandle) {
		name, cause := symbols.Intern("clp-cleanup-" + label)
		must(cause)
		declaration, cause := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(root.RegisterPredicate(declaration))
		application, cause := manager.MkApp(declaration, nil)
		must(cause)
		return declaration, application
	}
	_, pApp := declare("p")
	q, qApp := declare("q")
	must(root.addRuleEager(pApp, "fact"))
	edge, err := manager.MkImplies(pApp, qApp)
	must(err)
	must(root.addRuleEager(edge, "edge"))

	prepared, err := root.preprocessHornRulesWithQueryRule(q, nil, true, true, false, false)
	must(err)
	attempt = &clpBackendAttempt{manager: manager, preprocessed: prepared}
	queryDeclaration, err := prepared.rules.DerivedPredicate(q)
	must(err)
	rules, err := prepared.rules.PredicateRules(queryDeclaration)
	must(err)
	if len(rules) == 0 {
		t.Fatal("preprocessing removed the CLP query")
	}
	query, err := rules[0].Head()
	must(err)
	// The kernel belongs to the engine (clp::imp::m_solver), not the attempt.
	backend, err := root.bmcQuerySolver()
	must(err)
	attempt.engine, err = clp.NewEngine(manager, prepared.rules, backend, 20)
	must(err)
	status, err := attempt.engine.Query(query)
	must(err)
	if status != lbool.True {
		t.Fatal("CLP fixture query", status)
	}
	for i := len(rules) - 1; i >= 0; i-- {
		must(rules[i].Close())
	}

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
	if body == nil || !callback || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("expected actual backend cleanup refusal", body, callback, failure)
	}
	// Without an attempt-owned kernel, the preprocessing owners refuse.
	if attempt.engine != nil || attempt.preprocessed != prepared {
		t.Fatal("refused cleanup lost or advanced owners")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime || cleanup.OriginalCause() == nil || cleanup.CleanupCause() == nil {
		t.Fatal("backend cleanup refusal is not retryable", failure)
	}
	must(cleanup.Cleanup())
	if attempt.engine != nil || attempt.preprocessed != nil || cleanup.CleanupCause() != nil {
		t.Fatal("backend cleanup retry did not retire all owners")
	}
}
