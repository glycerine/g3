package fp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogConfiguredIntervalFactsLatePredicatesAndExpressionOwners(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		sym, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("configured-interval-P")
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	oneRef, err := m.NewRef(one)
	must(err)
	defer func() { must(m.DelRef(&oneRef)) }()
	must(e.Push())
	must(e.AddFact(p, []ast.ASTHandle{one}))
	must(e.AddTableFact(p, []uint32{1}))
	if err := e.Pop(); err == nil {
		t.Fatal("interval fact did not instantiate source engine")
	}
	q := declare("configured-interval-late-Q")
	x, err := m.MkVar(0, integer)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qx, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.AddRule(rule, "interval-step"))
	// Favourite selection belongs to the engine lifetime, not later params.
	must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "pentagon"))
	must(e.SetQueryParameters(nil))
	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.Undef {
		t.Fatal("nonempty interval should be unknown", status)
	}
	answer, err := e.Answer(q)
	must(err)
	defer func() { must(m.DelRef(&answer)) }()
	value, err := e.intervalDatalog.relation(q)
	must(err)
	found, err := value.ContainsFactExpressions(m, []ast.ASTHandle{one})
	must(err)
	if !found {
		t.Fatal("late predicate lost seed")
	}
	query, err := m.MkApp(q, []ast.ASTHandle{one})
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	status, err = e.QueryExpression(query)
	must(err)
	if status != lbool.Undef {
		t.Fatal("ground interval abstraction should be unknown", status)
	}
	clone := e.queryClone
	if clone.intervalDatalog == nil || clone.intervalDatalog == e.intervalDatalog {
		t.Fatal("expression clone lacks independent interval owner")
	}
	expression, err := e.ExpressionAnswer()
	must(err)
	defer func() { must(m.DelRef(&expression)) }()
	must(e.clearQueryResources())
	if _, err = m.RefGet(expression); err != nil {
		t.Fatal("owning answer lost with clone", err)
	}
	two, err := m.ArithmeticIntegerNumeralText("2")
	must(err)
	absent, err := m.MkApp(q, []ast.ASTHandle{two})
	must(err)
	status, err = e.QueryExpression(absent)
	must(err)
	if status != lbool.False {
		t.Fatal("interval contradiction did not prove empty", status)
	}
}

func TestDatalogConfiguredIntervalNullaryAndUnsupportedFragment(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	status, err := e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.False {
		t.Fatal(status)
	}
	must(e.AddTableFact(f.declarations["p"], nil))
	status, err = e.QueryExpression(f.applications["p"])
	must(err)
	if status != lbool.Undef {
		t.Fatal("explicit nullary interval plugin lost imprecision", status)
	}
	neg, err := m.MkNot(f.applications["p"])
	must(err)
	rule, err := m.MkImplies(neg, f.applications["q"])
	must(err)
	must(e.AddRule(rule, "unsupported-negative"))
	_, err = e.QueryPredicate(f.declarations["q"])
	if !errors.Is(err, ErrIntervalDatalogFragment) {
		t.Fatal("selected unsupported rule silently fell back", err)
	}
	if _, err = e.Answer(f.declarations["q"]); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed interval query exposed stale answer", err)
	}
}

func TestDatalogConfiguredIntervalEmptyProgramQuery(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	fragmentMust(t, g.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
	fragmentMust(t, e.SelectEngine("datalog"))
	status, err := e.QueryPredicates(nil)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("empty interval query request", status)
	}
}
