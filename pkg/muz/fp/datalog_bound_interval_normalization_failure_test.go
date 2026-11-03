package fp

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

func TestDatalogBoundIntervalNormalizationCompletionRetriesNativeCleanup(t *testing.T) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 28)
	var scope *boundIntervalNormalization
	var completion error
	var completionStatus, preparedStatus lbool.LBool
	active, fired := false, false
	native, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			// Exercise the production completion stage with all transformed
			// owners present while the real AST invocation refuses cleanup.
			completionStatus, completion = scope.complete(preparedStatus, nil)
		}
		return failure
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: native, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
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
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(m.RegisterDeclarationPlugins())
	e, err := NewPropositionalEngineWithServices(m, params, globals)
	must(err)
	f := &boundIntervalPublicFixture{t: t, m: m, e: e, kinds: []string{"interval_relation", "bound_relation"}}
	defer func() {
		active = false
		must(native.SetMaxCount(0))
		must(scope.Close())
		for i := len(f.roots) - 1; i >= 0; i-- {
			must(m.DelRef(&f.roots[i]))
		}
		must(e.Close())
		must(m.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(native.Close())
		if budget.Used() != 0 {
			t.Fatal("normalization cleanup leaked bytes", budget.Used())
		}
	}()
	must(e.SelectEngine("datalog"))
	family, err := m.ArithmeticFamilyID()
	must(err)
	f.integer = f.hold(m.MkFamilySort(family, 1, nil))
	f.boolean = f.hold(m.MkBoolSort())
	p, q, r, h := f.declare("cleanup-P", 2), f.declare("cleanup-Q", 1), f.declare("cleanup-R", 2), f.declare("cleanup-H", 1)
	f.fact(p, 7, 7)
	f.fact(q, 7)
	f.fact(r, 7, 7)
	x, seven := f.hold(m.MkVar(0, f.integer)), f.numeral(7)
	rule, err := e.rules.NewRule(f.app(h, x), []ast.ASTHandle{f.app(p, x, x), f.app(q, x), f.app(r, x, seven)}, nil, "native-normalization-cleanup")
	must(err)
	must(e.set.Add(rule))
	must(rule.Close())
	status, err := e.QueryPredicate(h)
	must(err)
	if status != lbool.Undef {
		t.Fatal("initial normalized query", status)
	}
	savedAnswer := f.own(e.Answer(h))
	source := e.boundIntervalDatalog
	originalSet, originalCount := source.rules, len(source.order)
	seed := source.entries[p].seed
	// Match queryNormalized's snapshot invalidation before preparing a scope.
	source.evaluated = nil
	scope, err = prepareBoundIntervalNormalization(source, []ast.ASTHandle{h})
	must(err)
	activation, transformer, localSet, localRules := scope.activation, scope.transformer, scope.set, scope.rules
	if localRules == e.rules || localSet == originalSet || len(activation.entries) <= originalCount {
		t.Fatal("fixture did not build independent normalization owners")
	}
	var fresh []ast.ASTHandle
	var ownerRoots []ast.ASTRef
	for declaration, entry := range activation.entries {
		ownerRoots = append(ownerRoots, entry.root)
		if source.entries[declaration] == nil {
			fresh = append(fresh, declaration)
			registered, cause := e.rules.IsRegisteredPredicate(declaration)
			must(cause)
			if registered || e.predicates[declaration] != nil || !boundIntervalKindsEqual(entry.kinds, f.kinds) {
				t.Fatal("local helper escaped or lost requested kind order")
			}
		}
	}
	if len(fresh) < 3 {
		t.Fatal("fixture must contain repeated/constant filters and a split", len(fresh))
	}
	preparedStatus, err = activation.queryWithWidening([]ast.ASTHandle{h}, true, false)
	must(err)
	if preparedStatus != lbool.Undef || activation.evaluated[h].Empty() {
		t.Fatal("private normalized evaluation did not produce a result", preparedStatus)
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(native.SetMaxCount(uint64(native.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, failure := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(native.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	var receipt memory.ConstructionFailure
	if failure == nil || !fired || completionStatus != lbool.Undef || !errors.As(completion, &receipt) || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) || receipt.RuntimeOwner() != native {
		t.Fatal("native completion refusal lost its retry receipt", failure, fired, completionStatus, completion)
	}
	if scope.activation != activation || scope.transformer != transformer || scope.set != localSet || scope.rules != localRules || activation.closed || source.evaluated != nil {
		t.Fatal("refused completion discarded an owner or published a result")
	}
	for _, root := range ownerRoots {
		if _, cause := m.RefGet(root); cause != nil {
			t.Fatal("refused cleanup lost a declaration owner", cause)
		}
	}
	if _, cause := source.relationFormula(h); !errors.Is(cause, ErrBoundIntervalDatalogFragment) {
		t.Fatal("refused completion exposed a result", cause)
	}
	shown, exposed, original := receipt.Error(), errors.Unwrap(receipt), receipt.OriginalCause()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.Error() != shown || errors.Unwrap(receipt) != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil || scope.activation != nil || scope.transformer != nil || scope.set != nil || scope.rules != nil || source.evaluated != nil {
		t.Fatal("cleanup retry changed diagnostics, leaked owners, or published a result")
	}
	for _, declaration := range fresh {
		if _, cause := m.GetKind(declaration); cause == nil {
			t.Fatal("local generated declaration outlived its scope")
		}
	}
	if source.rules != originalSet || len(source.entries) != originalCount || len(e.predicates) != originalCount || source.entries[p].seed != seed {
		t.Fatal("cleanup changed the persistent source program or seeds")
	}
	if count, cause := originalSet.Len(); cause != nil || count != 1 {
		t.Fatal("cleanup changed original rule count", count, cause)
	}
	stored, err := originalSet.Rule(0)
	must(err)
	positive, err := stored.PositiveTailSize()
	must(err)
	must(stored.Close())
	if positive != 3 {
		t.Fatal("normalization replaced the raw source rule", positive)
	}
	status, err = e.QueryPredicate(h)
	must(err)
	if status != lbool.Undef || len(source.evaluated) != originalCount {
		t.Fatal("query did not recover with original-only results", status)
	}
	f.checkFormula(f.own(e.Answer(h)), []int{7}, true)
	f.checkFormula(savedAnswer, []int{7}, true)
	f.checkFormula(savedAnswer, []int{8}, false)
}
