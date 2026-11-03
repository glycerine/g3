package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
	"math"
	"testing"
)

func TestQueryCloneStatisticsSurviveNativeLateCleanup(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		for _, completion := range []string{"clear", "statistics-reset"} {
			t.Run(backend+"/"+completion, func(t *testing.T) { testQueryCloneStatisticsNativeCleanup(t, backend, completion) })
		}
	}
}

func testQueryCloneStatisticsNativeCleanup(t *testing.T, backend, completion string) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 29)
	rt, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: rt, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	sy, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(sy, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(sy, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(sy, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: numbers, Budget: budget})
	must(err)
	must(m.RegisterDeclarationPlugins())
	e, err := NewPropositionalEngineWithServices(m, params, globals)
	must(err)
	must(e.SelectEngine(backend))
	for _, key := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		must(globals.SourceSetName(key, "false"))
	}
	boolean, err := m.MkBoolSort()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	intSort, err := m.MkFamilySort(family, 1, nil)
	must(err)
	name, err := sy.Intern("native-clone-statistics-P")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{intSort}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	fact, err := m.MkApp(p, []ast.ASTHandle{zero})
	must(err)
	must(e.AddRule(fact, "native-statistics-fact"))
	variable, err := m.MkVar(0, intSort)
	must(err)
	current, err := m.MkApp(p, []ast.ASTHandle{variable})
	must(err)
	nextValue, err := m.MkArithmeticAdd(variable, one)
	must(err)
	next, err := m.MkApp(p, []ast.ASTHandle{nextValue})
	must(err)
	bound, err := m.MkArithmeticLT(variable, two)
	must(err)
	body, err := m.MkAnd(current, bound)
	must(err)
	edge, err := m.MkImplies(body, next)
	must(err)
	must(e.AddRule(edge, "native-statistics-successor"))
	query, err := m.MkApp(p, []ast.ASTHandle{two})
	must(err)
	queryRoot, err := m.NewRef(query)
	must(err)
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True || e.queryClone == nil || backend == "pdr" && e.queryClone.pdr == nil || backend == "spacer" && e.queryClone.spacer == nil {
		t.Fatal("no actual expression owner", status)
	}
	clone := e.queryClone
	answer, err := e.ExpressionAnswer()
	must(err)
	count := func(key statistics.Key) uint32 {
		t.Helper()
		s := statistics.New()
		must(e.CollectStatistics(s))
		var value uint32
		for i := 0; i < s.Size(); i++ {
			if s.Key(i) == key && s.IsUint(i) {
				value += s.UintValue(i)
			}
		}
		return value
	}
	before := count("num checks")
	if before == 0 {
		t.Fatal("query produced no solver checks")
	}
	searchKey := statistics.Key("PDR num unfoldings")
	if backend == "spacer" {
		searchKey = "SPACER num queries"
	}
	searchBefore := count(searchKey)
	if searchBefore == 0 {
		t.Fatal("query produced no search counters", searchKey)
	}

	// The clone's ordinary background assertion is released after its backend.
	// A real rational declaration parameter supplies a fallible final destructor.
	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	words := make([]uint32, 262144)
	words[0], words[len(words)-1] = 17, 1
	must(numbers.Manager.Z.SetLimbs(integer, words))
	value, err := numbers.NewFromScopedInt(scoped)
	must(err)
	parameter, err := m.NewParameterRational(&value)
	must(err)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	label, err := sy.Intern("native-clone-late-background")
	must(err)
	declaration, err := m.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := m.MkApp(declaration, nil)
	must(err)
	must(clone.AssertBackground(formula))
	must(m.DelFuncDeclInfo(&info))
	must(m.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	if refs, cause := m.GetRefCount(declaration); cause != nil || refs != 1 {
		t.Fatal("background is not sole metadata owner", refs, cause)
	}

	a, err := rt.Allocate(8)
	must(err)
	b, err := rt.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = rt.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	must(rt.Deallocate(&a))
	delta = math.MinInt64 + 262144 - rt.State().GlobalSize + 16
	if err = rt.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
	must(rt.Deallocate(&b))
	if state := rt.State(); state.GlobalSize != math.MinInt64+262144 || state.LocalSize != 0 {
		t.Fatal("TLS fixture", state)
	}
	cause := e.clearQueryResources()
	if !errors.Is(cause, arena.ErrBounds) || backend == "pdr" && clone.pdr != nil || backend == "spacer" && clone.spacer != nil || e.queryClone != clone {
		t.Fatal("did not reach late backend-disposed cleanup", cause, clone.pdr, clone.closed, e.queryClone == clone)
	}
	recovery, err := rt.Allocate(4 << 20)
	must(err)
	gotPending := count("num checks")
	searchPending := count(searchKey)
	if completion == "statistics-reset" {
		must(e.ResetStatistics())
	} else {
		must(e.clearQueryResources())
	}
	gotFinished := count("num checks")
	searchFinished := count(searchKey)
	must(e.clearQueryResources())
	gotRepeated := count("num checks")
	searchRepeated := count(searchKey)
	if e.queryClone != nil || e.queryCloneDisposal != nil || !clone.closed {
		t.Fatal("statistics cache concealed an unfinished clone disposal")
	}
	if _, cause = m.RefGet(answer); cause != nil {
		t.Fatal("caller answer invalidated", cause)
	}
	must(m.DelRef(&answer))
	must(m.DelRef(&queryRoot))
	must(e.Close())
	finish := func(close func() error) {
		t.Helper()
		for i := 0; i < 128; i++ {
			old := rt.Stats().LiveBlocks
			cause := close()
			if cause == nil {
				return
			}
			if !errors.Is(cause, arena.ErrBounds) || rt.Stats().LiveBlocks >= old {
				t.Fatal("cleanup made no progress", cause, rt.State())
			}
		}
		t.Fatal("cleanup horizon")
	}
	finish(m.Close)
	finish(globals.Close)
	finish(catalog.Close)
	finish(params.Close)
	finish(numbers.Close)
	finish(sy.Close)
	if cause = rt.Deallocate(&recovery); cause != nil && (!errors.Is(cause, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(cause)
	}
	finish(rt.Close)
	if budget.Used() != 0 {
		t.Fatal("budget leak", budget.Used())
	}
	if gotPending != before || gotFinished != before || gotRepeated != before {
		t.Fatal("late cleanup lost cumulative checks", before, gotPending, gotFinished, gotRepeated)
	}
	searchWant := searchBefore
	if completion == "statistics-reset" {
		searchWant = 0
	}
	if searchPending != searchBefore || searchFinished != searchWant || searchRepeated != searchWant {
		t.Fatal("late cleanup lost or resurrected search counters", searchKey, searchBefore, searchPending, searchFinished, searchRepeated)
	}
}
