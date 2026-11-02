package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var errNormalizeCleanupOrigin = errors.New("normalize cleanup origin")

func TestNormalizeCleanupPreservesEachPendingOwnerStage(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	var scopes []*normalizeCleanupFailure
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active {
			callbacks++
			for attempt := 0; attempt < 2; attempt++ {
				for index, scope := range scopes {
					cause := scope.Cleanup()
					intact = intact && errors.Is(cause, ast.ErrASTPending)
					switch index {
					case 0:
						intact = intact && scope.equivalence != nil
					case 1:
						intact = intact && len(scope.values) == 1 && scope.values[0] != (ast.ASTRef{})
					case 2:
						intact = intact && scope.output != (ast.ASTRef{})
					case 3:
						intact = intact && scope.rewriter != nil
					}
				}
			}
		}
		return source
	}})
	must(err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		for _, scope := range scopes {
			must(scope.Cleanup())
		}
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("normalize cleanup fixture leaked bytes", budget.Used())
		}
	}()
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	must(err)
	sort, err := manager.GetSort(zero)
	must(err)
	x, err := manager.MkFreshConst("normalize-pending-x", sort)
	must(err)
	y, err := manager.MkFreshConst("normalize-pending-y", sort)
	must(err)
	formula, err := manager.MkEq(x, y)
	must(err)
	for stage := 0; stage < 4; stage++ {
		local, err := params.NewValue()
		must(err)
		rw, err := astrewriter.NewTheoryRewriter(manager, params, globals, &local)
		must(err)
		scope := &normalizeCleanupFailure{manager: manager, context: params, local: local, localLive: true, rewriter: rw, cause: errNormalizeCleanupOrigin}
		if stage <= 2 {
			scope.output, err = manager.NewRef(formula)
			must(err)
		}
		if stage <= 1 {
			ref, err := manager.NewRef(formula)
			must(err)
			scope.values = []ast.ASTRef{ref}
		}
		if stage == 0 {
			scope.equivalence, err = ast.NewExprEquivClass(manager)
			must(err)
			must(scope.equivalence.Merge(x, y))
		}
		scopes = append(scopes, scope)
	}
	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("normalize cleanup lost a pending owner", body, callbacks, intact)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	for _, scope := range scopes {
		must(scope.Cleanup())
		must(scope.Cleanup())
		if scope.CleanupCause() != nil || !errors.Is(scope, errNormalizeCleanupOrigin) || scope.Error() == "" {
			t.Fatal("normalize cleanup retry lost its stable receipt", scope.CleanupCause(), scope)
		}
	}
}

func TestNormalizeFactorFailureNeverPublishesDanglingOutput(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	zero, err := m.ArithmeticIntegerNumeralText("0")
	depthMust(t, err)
	sort, err := m.GetSort(zero)
	depthMust(t, err)
	x, err := m.MkFreshConst("normalize-failure-x", sort)
	depthMust(t, err)
	y, err := m.MkFreshConst("normalize-failure-y", sort)
	depthMust(t, err)
	z, err := m.MkFreshConst("normalize-failure-z", sort)
	depthMust(t, err)
	xy, err := m.MkEq(x, y)
	depthMust(t, err)
	yz, err := m.MkEq(y, z)
	depthMust(t, err)
	z0, err := m.MkEq(z, zero)
	depthMust(t, err)
	formula, err := m.MkAndArray([]ast.ASTHandle{xy, yz, z0})
	depthMust(t, err)
	pin, err := m.NewRef(formula)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&pin)) }()
	stops := make([]int64, 128)
	for i := range stops {
		stops[i] = int64(i)
	}
	for stop := int64(128); stop <= 8192; stop *= 2 {
		stops = append(stops, stop)
	}
	failures, succeeded := 0, false
	for _, stop := range stops {
		before := m.Stats().LiveReferences
		m.Budget().FailAfter(stop)
		result, cause := normalizeFormula(m, globals.Context, globals, formula, false, true)
		m.Budget().FailAfter(-1)
		if cause != nil {
			if result != (ast.ASTRef{}) {
				t.Fatal("failed normalization published a reference", stop, result, cause)
			}
			var cleanup memory.ConstructionFailure
			if errors.As(cause, &cleanup) {
				depthMust(t, cleanup.Cleanup())
			}
			failures++
		} else {
			succeeded = true
			depthMust(t, m.DelRef(&result))
		}
		if got := m.Stats().LiveReferences; got != before {
			t.Fatal("normalization failure changed reference ownership", stop, got, before)
		}
		if succeeded {
			break
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal(fmt.Sprintf("missing normalize failure/success frontier: %d %v", failures, succeeded))
	}
}
