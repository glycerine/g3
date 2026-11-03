package fp

// Source: expression-query PDR owner transfer around pdr_dl_interface::query.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRExpressionTransferredOwnerSurvivesNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var engine, clone *PropositionalEngine
	var query ast.ASTHandle
	active, callback := false, false
	var cleanupErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			status, queryErr := engine.QueryExpression(query)
			cleanupErr = queryErr
			if status != lbool.Undef || !errors.Is(cleanupErr, ast.ErrASTPending) || engine.queryClone != clone || clone.closed {
				t.Fatalf("native refusal lost transferred query owner: %v", cleanupErr)
			}
			if clone.pdr == nil {
				t.Fatal("native refusal detached the transferred PDR context")
			}
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
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if engine != nil {
			must(engine.Close())
		}
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("expression cleanup fixture leaked bytes", budget.Used())
		}
	}()
	engine, err = NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	must(engine.SelectEngine("pdr"))
	parameters, err := params.NewValue()
	must(err)
	defer func() { must(params.Del(&parameters)) }()
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(params.SetBoolName(&parameters, &name, false))
	}
	must(engine.SetQueryParameters(&parameters))
	boolean, err := manager.MkBoolSort()
	must(err)
	arithmetic, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	must(err)
	name, err := symbols.Intern("pending-expression-owner")
	must(err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	must(err)
	fact, err := manager.MkApp(predicate, []ast.ASTHandle{zero})
	must(err)
	must(engine.AddRule(fact, "pending-expression-fact"))
	variable, err := manager.MkVar(0, integer)
	must(err)
	property, err := manager.MkEq(variable, zero)
	must(err)
	propertyRef, err := manager.NewRef(property)
	must(err)
	defer func() { must(manager.DelRef(&propertyRef)) }()
	query, err = manager.MkApp(predicate, []ast.ASTHandle{variable})
	must(err)
	queryRef, err := manager.NewRef(query)
	must(err)
	defer func() { must(manager.DelRef(&queryRef)) }()
	must(engine.AddCover(2, predicate, property))
	prequery := engine.pdr
	status, err := engine.QueryExpression(query)
	must(err)
	if status != lbool.True || engine.queryClone == nil || engine.queryClone.pdr != prequery || engine.pdr != nil {
		t.Fatal("fixture did not transfer pre-query PDR owner", status)
	}
	clone = engine.queryClone

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
	if body == nil || !callback || !errors.Is(cleanupErr, ast.ErrASTPending) {
		t.Fatal("native execution guard did not refuse transferred cleanup", body, callback, cleanupErr)
	}
	if _, err = engine.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("pending cleanup published the transferred result", err)
	}
	if _, err = engine.ExpressionAnswer(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("pending cleanup published the transferred answer", err)
	}
	cover, err := engine.CoverDelta(2, predicate)
	must(err)
	coverHandle, err := manager.RefGet(cover)
	must(err)
	if coverHandle != property {
		t.Fatal("pending public cleanup changed transferred cover", coverHandle, property)
	}
	must(manager.DelRef(&cover))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	status, err = engine.QueryExpression(query)
	must(err)
	if status != lbool.True {
		t.Fatal("query did not recover after cleanup continuation", status)
	}
	if !clone.closed {
		t.Fatal("public query retry did not retire transferred clone")
	}
	if _, err = prequery.Initialized(); err == nil {
		t.Fatal("public query retry left transferred PDR context open")
	}
}
