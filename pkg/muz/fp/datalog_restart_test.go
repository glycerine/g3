package fp

// Source: rel_context.cpp:133-227 uses a per-round restart timeout, stores
// partial finite relations, and multiplies the next timeout by the initial one.

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestDatalogInitialRestartTimeoutParameters(t *testing.T) {
	for _, test := range []struct {
		name, module string
		local        *uint32
		want         uint32
	}{
		{name: "default"},
		{name: "module", module: "7", want: 7},
		{name: "local-two", local: restartUint(2), want: 2},
		{name: "local-zero-over-module", module: "7", local: restartUint(0)},
		{name: "local-two-over-module", module: "7", local: restartUint(2), want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			if test.module != "" {
				must(globals.SourceSetName("fixedpoint.datalog.initial_restart_timeout", test.module))
			}
			e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
			must(err)
			t.Cleanup(func() { must(e.Close()) })
			if test.local != nil {
				local := globals.Context.EmptyValue()
				key := "datalog.initial_restart_timeout"
				must(globals.Context.SetUIntName(&local, &key, *test.local))
				must(e.SetQueryParameters(&local))
				must(globals.Context.Del(&local))
			}
			got, err := e.datalogInitialRestartTimeout()
			must(err)
			if got != test.want {
				t.Fatal("restart timeout parameter", got, test.want)
			}
		})
	}
}

func restartUint(value uint32) *uint32 { return &value }

func TestDatalogRestartTimeoutUsesInitialMultiplierAndClamps(t *testing.T) {
	for _, test := range []struct{ current, initial, want uint32 }{
		{3, 3, 9}, {9, 3, 27}, {1, 1, 1}, {0, 0, 0},
		{math.MaxUint32 / 3, 3, math.MaxUint32},
		{math.MaxUint32/3 + 1, 3, math.MaxUint32},
		{math.MaxUint32, math.MaxUint32, math.MaxUint32},
	} {
		if got := nextDatalogRestartTimeout(test.current, test.initial); got != test.want {
			t.Fatal("restart timeout multiplication", test.current, test.initial, got, test.want)
		}
	}
}

func TestDatalogRestartRoundResetsTimerAndAccumulatesOperations(t *testing.T) {
	e, edge, reach := newDatalogRestartChain(t, 4)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	valid, err := e.set.Finalize()
	must(err)
	if !valid {
		t.Fatal("restart chain is not stratified")
	}
	execution, termination, err := rel.CompileFiniteRules(e.relations, e.set)
	must(err)
	t.Cleanup(func() { must(execution.Close()); must(termination.Close()) })
	var context *rel.ExecutionContext
	timerExpired := false
	context, err = rel.NewExecutionContextWithRelations(e.relations, func() bool {
		if timerExpired {
			return false
		}
		count, err := context.RegisterCount()
		must(err)
		for index := uint32(0); index < count; index++ {
			name, _, err := context.RegisterAnnotation(index)
			must(err)
			if name != "timed-restart-reach" {
				continue
			}
			table, err := context.FiniteRegister(index)
			must(err)
			if table == nil {
				continue
			}
			rows, err := table.Len()
			must(err)
			if rows == 2 {
				// Arm a real short timeout only after compiled recursion has
				// made partial progress, independent of the host's speed. The
				// callback never requests cancellation; the timer must stop it.
				timerExpired = true
				must(context.SetTimeLimit(1))
				time.Sleep(5 * time.Millisecond)
			}
		}
		return false
	})
	must(err)
	t.Cleanup(func() { must(context.Close()) })
	must(termination.MakeAnnotations(context))
	round := datalogRestartRound{timeout: math.MaxUint32}
	complete, terminated, err := performDatalogRestartRound(execution, termination, context, &round)
	must(err)
	if complete || !terminated || !timerExpired || context.ShouldTerminate() {
		t.Fatal("timer must interrupt execution and be disabled before termination", complete, terminated, timerExpired)
	}
	partial, err := e.relations.GetFiniteRelation(reach)
	must(err)
	if rows, err := partial.Len(); err != nil || rows != 2 {
		t.Fatal("termination did not store partial recursive progress", rows, err)
	}
	for _, value := range []rel.Element{0, 1} {
		if found, err := partial.Contains(rel.Fact{value}); err != nil || !found {
			t.Fatal("termination lost a derived tuple", value, found, err)
		}
	}
	if saturated, err := e.relations.IsSaturated(edge); err != nil || !saturated {
		t.Fatal("completed lower stratum lost its mark", saturated, err)
	}
	if saturated, err := e.relations.IsSaturated(reach); err != nil || saturated {
		t.Fatal("interrupted recursive stratum was marked saturated", saturated, err)
	}
	first, err := context.Statistics()
	must(err)
	if first.Union == 0 || first.Join+first.JoinProject == 0 {
		t.Fatal("first round did not execute real recursive operations", first)
	}
	var measured statistics.Statistics
	must(context.CollectStatistics(&measured))
	must(context.Close())
	must(execution.Close())
	must(termination.Close())

	// Recompile from the saved partial result without clearing completed
	// strata. The same round owns counters from both execution contexts.
	retry, finish, err := rel.CompileFiniteRules(e.relations, e.set)
	must(err)
	t.Cleanup(func() { must(retry.Close()); must(finish.Close()) })
	healthy, err := rel.NewExecutionContextWithRelations(e.relations, nil)
	must(err)
	t.Cleanup(func() { must(healthy.Close()) })
	round.timeout = 0
	complete, terminated, err = performDatalogRestartRound(retry, finish, healthy, &round)
	must(err)
	if !complete || !terminated {
		t.Fatal("healthy retry did not complete", complete, terminated)
	}
	second, err := healthy.Statistics()
	must(err)
	if second.Union == 0 || second.Join+second.JoinProject == 0 {
		t.Fatal("retry did not execute the remaining closure", second)
	}
	must(healthy.CollectStatistics(&measured))
	sums := func(values *statistics.Statistics) map[statistics.Key]uint32 {
		result := make(map[statistics.Key]uint32)
		for index := 0; index < values.Size(); index++ {
			if !values.IsUint(index) {
				t.Fatal("relational operation count is not an integer", values.Key(index))
			}
			result[values.Key(index)] += values.UintValue(index)
		}
		return result
	}
	want, got := sums(&measured), sums(&round.statistics)
	if len(got) != len(want) {
		t.Fatal("restart operation statistic keys", got, want)
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatal("restart operation counts did not accumulate", key, got[key], value)
		}
	}
	if got["dl.total"] != first.Total+second.Total || got["dl.union"] != first.Union+second.Union {
		t.Fatal("restart counters disagree with measured execution", got, first, second)
	}
	closure, err := e.relations.GetFiniteRelation(reach)
	must(err)
	if rows, err := closure.Len(); err != nil || rows != 4 {
		t.Fatal("retry did not complete the recursive closure", rows, err)
	}
	for value := rel.Element(0); value < 4; value++ {
		if found, err := closure.Contains(rel.Fact{value}); err != nil || !found {
			t.Fatal("retry missed reachable tuple", value, found, err)
		}
	}
	for _, predicate := range []ast.ASTHandle{edge, reach} {
		if saturated, err := e.relations.IsSaturated(predicate); err != nil || !saturated {
			t.Fatal("completed retry did not mark its strata saturated", predicate, saturated, err)
		}
	}
}

func TestDatalogRestartDriverCompletesClosureAndPreservesProgram(t *testing.T) {
	const nodes = 32
	e, _, reach := newDatalogRestartChain(t, nodes)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	original := e.set
	originalCount, err := original.Len()
	must(err)
	// Public query completion must not depend on a machine-specific retry
	// count. The separate round test deterministically exercises the timer.
	status, err := e.QueryPredicate(reach)
	must(err)
	if status != lbool.True || !e.compiled {
		t.Fatal("restart driver did not complete compiled closure", status, e.compiled)
	}
	rows, err := e.predicates[reach].table.Len()
	must(err)
	if rows != nodes {
		t.Fatal("restart lost reachable rows", rows, nodes)
	}
	for value := uint32(0); value < nodes; value++ {
		found, err := e.predicates[reach].table.Contains(rel.Fact{rel.Element(value)})
		must(err)
		if !found {
			t.Fatal("restart missed reachable value", value)
		}
	}
	if e.set != original || len(e.predicates) != 2 || len(e.auxiliary) != 0 {
		t.Fatal("completed driver changed the persistent program")
	}
	if count, err := original.Len(); err != nil || count != originalCount {
		t.Fatal("completed driver changed source rules", count, originalCount, err)
	}
	if saturated, err := e.relations.IsSaturated(reach); err != nil || !saturated {
		t.Fatal("completed driver did not mark recursion saturated", saturated, err)
	}
}

func newDatalogRestartChain(t *testing.T, nodes uint32) (*PropositionalEngine, ast.ASTHandle, ast.ASTHandle) {
	t.Helper()
	m, globals := newPDRLifecycleServices(t)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	for _, key := range []string{"xform.inline_eager", "xform.inline_linear", "datalog.unbound_compressor", "datalog.similarity_compressor"} {
		must(globals.SourceSetName("fixedpoint."+key, "false"))
	}
	must(globals.SourceSetName("fixedpoint.datalog.initial_restart_timeout", "2"))
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	must(err)
	t.Cleanup(func() { must(e.Close()) })
	must(e.SelectEngine("datalog"))
	bits, err := m.BitVectorSort(8)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, arity int) ast.ASTHandle {
		symbol, err := m.Symbols.Intern("timed-restart-" + name)
		must(err)
		domain := make([]ast.ASTHandle, arity)
		for index := range domain {
			domain[index] = bits
		}
		predicate, err := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(predicate))
		return predicate
	}
	edge, reach := declare("edge", 2), declare("reach", 1)
	app := func(predicate ast.ASTHandle, arguments ...ast.ASTHandle) ast.ASTHandle {
		application, err := m.MkApp(predicate, arguments)
		must(err)
		return application
	}
	numeral := func(value uint64) ast.ASTHandle {
		result, err := m.BitVectorNumeralUint64(value, 8)
		must(err)
		return result
	}
	add := func(name string, head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, err := e.rules.NewRule(head, tails, nil, name)
		must(err)
		must(e.set.Add(rule))
		must(rule.Close())
	}
	// Source rules regenerate even a relation whose LOAD was not reached in
	// a very short first round; there is no dependency on preserved input seeds.
	for value := uint64(0); value+1 < uint64(nodes); value++ {
		add("edge-"+strconv.FormatUint(value, 10), app(edge, numeral(value), numeral(value+1)))
	}
	add("seed", app(reach, numeral(0)))
	x, err := m.MkVar(0, bits)
	must(err)
	y, err := m.MkVar(1, bits)
	must(err)
	add("step", app(reach, y), app(reach, x), app(edge, x, y))
	return e, edge, reach
}
