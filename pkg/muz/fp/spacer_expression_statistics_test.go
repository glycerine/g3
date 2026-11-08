package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSpacerExpressionStatisticsSurviveCloneDisposal(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e := f.engine
	fragmentMust(t, e.SelectEngine("spacer"))
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
	seconds := func(engine *PropositionalEngine, key statistics.Key) float64 {
		t.Helper()
		stats := statistics.New()
		must(engine.CollectStatistics(stats))
		var result float64
		for i := 0; i < stats.Size(); i++ {
			if !stats.IsUint(i) && stats.Key(i) == key {
				result += stats.DoubleValue(i)
			}
		}
		return result
	}
	var solveSeconds, initSeconds float64
	checks := func(engine *PropositionalEngine) uint32 { return value(engine, "num checks") }
	var total, virtualChecks, queries, depth uint32
	for round := 0; round < 3; round++ {
		prior := e.queryClone
		currentQuery := query
		if round == 1 {
			currentQuery = fact
		}
		status, err := e.QueryExpression(currentQuery)
		must(err)
		if status != lbool.True {
			t.Fatal("query result", round, status)
		}
		if prior != nil {
			if _, err := prior.LastResult(); err == nil {
				t.Fatal("previous query clone remained open")
			}
		}
		current := checks(e.queryClone)
		if current == 0 {
			t.Fatal("query clone reported no solver checks")
		}
		solveSeconds += seconds(e.queryClone, "time.spacer.solve")
		initSeconds += seconds(e.queryClone, "time.spacer.init_rules")
		if got := seconds(e, "time.spacer.solve"); got != solveSeconds {
			t.Fatal("solve timer history lost or duplicated", got, solveSeconds)
		}
		if got := seconds(e, "time.spacer.init_rules"); got != initSeconds {
			t.Fatal("initialization timer history lost or duplicated", got, initSeconds)
		}
		total += current
		virtualChecks += value(e.queryClone, "virtual_solver.checks")
		if got := value(e, "virtual_solver.checks"); got != virtualChecks {
			t.Fatal("virtual check history was lost or duplicated", got, virtualChecks)
		}
		queries += value(e.queryClone, "SPACER num queries")
		currentDepth := value(e.queryClone, "SPACER max depth")
		if round == 0 && currentDepth == 0 {
			t.Fatal("fixture did not create a nontrivial search depth")
		}
		if currentDepth > depth {
			depth = currentDepth
		}
		for _, key := range []statistics.Key{"SPACER max query lvl", "SPACER cex depth"} {
			if got := value(e, key); got != value(e.queryClone, key) {
				t.Fatal("latest query statistic was accumulated", key, got, value(e.queryClone, key))
			}
		}
		if got := value(e, "SPACER max depth"); got != depth {
			t.Fatal("maximum depth was added across queries", got, depth)
		}
		if got := value(e, "SPACER num queries"); got != queries {
			t.Fatal("query history was lost", got, queries)
		}
		if got := value(e, "SPACER num properties"); got != value(e.queryClone, "SPACER num properties") {
			t.Fatal("disposed transformer properties escaped into current statistics", round, got)
		}
		truth, err := f.manager.MkTrue()
		must(err)
		must(e.AddCover(2, declaration, truth))
		if got := checks(e); got != total {
			t.Fatal("parent lost or duplicated query statistics", round, got, total)
		}
		if got := checks(e); got != total {
			t.Fatal("repeated collection mutated query statistics", got, total)
		}
	}
	properties := value(e.queryClone, "SPACER num properties")
	if properties == 0 || queries == 0 {
		t.Fatal("fixture did not create Spacer counters/properties")
	}
	if value(e, "virtual_solver.checks") == 0 {
		t.Fatal("fixture did not perform virtual solver checks")
	}
	must(e.ResetStatistics())
	if seconds(e, "time.spacer.solve") != 0 || seconds(e, "time.spacer.init_rules") != 0 {
		t.Fatal("statistics reset left context timer history")
	}
	if value(e, "virtual_solver.checks") != 0 {
		t.Fatal("statistics reset kept archived virtual solver checks")
	}
	if got := checks(e); got != total {
		t.Fatal("statistics reset erased cumulative kernel checks", got, total)
	}
	if value(e, "SPACER num queries") != 0 || value(e, "SPACER max depth") != 0 {
		t.Fatal("statistics reset preserved old Spacer search counters")
	}
	if got := value(e, "SPACER num properties"); got != properties {
		t.Fatal("statistics reset changed live property count", got, properties)
	}
	prior := e.queryClone
	must(e.clearQueryResources())
	if e.queryClone != nil {
		t.Fatal("query disposal left its clone active")
	}
	if _, err := prior.LastResult(); err == nil {
		t.Fatal("query disposal left the old clone open")
	}
	if got := checks(e); got != total {
		t.Fatal("clone disposal lost or duplicated cumulative checks", got, total)
	}
	if value(e, "SPACER num properties") != 0 {
		t.Fatal("disposed query left live property statistics")
	}
	must(e.clearQueryResources())
	if got := checks(e); got != total {
		t.Fatal("repeated cleanup duplicated archived statistics", got, total)
	}
	must(e.SetQueryParameters(nil))
	if got := checks(e); got != 0 {
		t.Fatal("parameter reconstruction preserved old kernel counters", got)
	}
	must(e.SetQueryParameters(nil))
	if got := checks(e); got != 0 {
		t.Fatal("repeated parameter update resurrected old counters", got)
	}
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True || checks(e) == 0 {
		t.Fatal("updated fixture did not create fresh checks", status)
	}
	must(e.ParametersUpdated())
	if got := checks(e); got != 0 {
		t.Fatal("module parameter reconstruction preserved old kernel counters", got)
	}
}
