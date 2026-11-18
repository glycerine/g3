package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestPDRExpressionStatisticsAccumulateInContext(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e := f.engine
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(e.params.SetBoolName(&parameters, &name, false))
	}
	must(e.SetQueryParameters(&parameters))
	m := f.manager
	arithmetic, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("statistics-integer-chain")
	must(err)
	declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(declaration))
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	fact, err := m.MkApp(declaration, []ast.ASTHandle{zero})
	must(err)
	must(e.addRuleEager(fact, "statistics-zero"))
	variable, err := m.MkVar(0, integer)
	must(err)
	current, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	must(err)
	nextValue, err := m.MkArithmeticAdd(variable, one)
	must(err)
	next, err := m.MkApp(declaration, []ast.ASTHandle{nextValue})
	must(err)
	edge, err := m.MkImplies(current, next)
	must(err)
	must(e.addRuleEager(edge, "statistics-successor"))
	query, err := m.MkApp(declaration, []ast.ASTHandle{three})
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	value := func(engine *PropositionalEngine, key statistics.Key) uint32 {
		t.Helper()
		stats := statistics.New()
		must(engine.CollectStatistics(stats))
		var count uint32
		for index := 0; index < stats.Size(); index++ {
			if stats.IsUint(index) && stats.Key(index) == key {
				count += stats.UintValue(index)
			}
		}
		return count
	}
	checks := func(engine *PropositionalEngine) uint32 { return value(engine, "num checks") }
	// dl_interface keeps one pdr::context: its counters and kernels persist
	// across the queries (context::reset keeps both).
	var total, unfoldings, depth uint32
	context := e.pdr
	for round := 0; round < 3; round++ {
		status, err := e.QueryExpression(query)
		must(err)
		if status != lbool.True {
			t.Fatal("query result", round, status)
		}
		if round > 0 && e.pdr != context {
			t.Fatal("expression query replaced the interface's context", round)
		}
		context = e.pdr
		current := checks(e)
		if current <= total {
			t.Fatal("query reported no new solver checks", round, current, total)
		}
		total = current
		currentUnfoldings := value(e, "PDR num unfoldings")
		if currentUnfoldings < unfoldings {
			t.Fatal("unfolding history was lost", currentUnfoldings, unfoldings)
		}
		unfoldings = currentUnfoldings
		currentDepth := value(e, "PDR max depth")
		if currentDepth == 0 {
			t.Fatal("fixture did not create a nontrivial search depth")
		}
		if currentDepth < depth {
			t.Fatal("maximum depth decreased across queries", currentDepth, depth)
		}
		depth = currentDepth
		truth, err := f.manager.MkTrue()
		must(err)
		must(e.AddCover(2, declaration, truth))
		if got := checks(e); got != total {
			t.Fatal("cover lost or duplicated query statistics", round, got, total)
		}
		if got := checks(e); got != total {
			t.Fatal("repeated collection mutated query statistics", got, total)
		}
	}
	properties := value(e, "PDR num properties")
	if properties == 0 || unfoldings == 0 {
		t.Fatal("fixture did not create PDR counters/properties")
	}
	must(e.ResetStatistics())
	if got := checks(e); got != total {
		t.Fatal("statistics reset erased cumulative kernel checks", got, total)
	}
	if value(e, "PDR num unfoldings") != 0 || value(e, "PDR max depth") != 0 {
		t.Fatal("statistics reset preserved old PDR search counters")
	}
	if got := value(e, "PDR num properties"); got != properties {
		t.Fatal("statistics reset changed live property count", got, properties)
	}
	prior := e.pdr
	must(e.SetQueryParameters(nil))
	if _, err := prior.LastResult(); err == nil {
		t.Fatal("parameter update left the old context open")
	}
	// pdr::dl_interface::updt_params deallocates the pdr::context, and with
	// it every counter of its pdr::manager (AUDIT.md muz_fp_04-F1).
	if got := checks(e); got != 0 {
		t.Fatal("parameter update kept the replaced context's checks", got, total)
	}
	if value(e, "PDR num properties") != 0 {
		t.Fatal("disposed query left live property statistics")
	}
	must(e.SetQueryParameters(nil))
	if got := checks(e); got != 0 {
		t.Fatal("repeated cleanup resurrected archived statistics", got)
	}
}
