package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogSparseIntervalHeterogeneousOriginals(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	f, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(f, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string, sorts []ast.ASTHandle, kinds []string) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		if kinds != nil {
			must(e.SetPredicateRepresentation(d, kinds))
		}
		return d
	}
	p := declare("heterogeneous-P", []ast.ASTHandle{boolean}, []string{"tr_sparse"})
	must(e.AddTableFact(p, []uint32{1})) // Existing exact tuples must survive activation.
	i := declare("heterogeneous-I", []ast.ASTHandle{integer}, []string{"interval_relation"})
	mixed := declare("heterogeneous-M", []ast.ASTHandle{boolean, integer}, []string{"tr_sparse", "interval_relation"})
	out := declare("heterogeneous-O", []ast.ASTHandle{boolean}, nil)
	arithmeticOut := declare("heterogeneous-derived-arithmetic", []ast.ASTHandle{integer}, []string{"interval_relation"})
	empty := declare("heterogeneous-empty", []ast.ASTHandle{boolean}, []string{"tr_sparse"})
	must(e.AddTableFact(i, []uint32{7}))
	b, err := m.MkVar(0, boolean)
	must(err)
	x, err := m.MkVar(1, integer)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkApp(d, args)
		must(err)
		return h
	}
	pa, ia, ma, oa := app(p, b), app(i, x), app(mixed, b, x), app(out, b)
	body, err := m.MkAnd(pa, ia)
	must(err)
	rule, err := m.MkImplies(body, ma)
	must(err)
	must(e.AddRule(rule, "heterogeneous-join"))
	rule, err = m.MkImplies(ma, oa)
	must(err)
	must(e.AddRule(rule, "heterogeneous-project"))
	arithmeticHead := app(arithmeticOut, x)
	arithmeticRule, err := m.MkImplies(body, arithmeticHead)
	must(err)
	must(e.AddRule(arithmeticRule, "heterogeneous-arithmetic-target"))

	status, err := e.QueryPredicate(mixed)
	must(err)
	if status != lbool.True {
		t.Fatal("sieve product must report source precision", status)
	}
	status, err = e.QueryPredicate(out)
	must(err)
	if status != lbool.True {
		t.Fatal("finite original must export exact nonempty result", status)
	}
	snapshots, err := e.DatalogRelationSnapshots([]ast.ASTHandle{out})
	must(err)
	if len(snapshots) != 1 || len(snapshots[0].Facts) != 1 || len(snapshots[0].Facts[0]) != 1 || snapshots[0].Facts[0][0] != 1 {
		t.Fatal("finite output was not published", snapshots)
	}
	arithmeticStatus, err := e.QueryPredicate(arithmeticOut)
	must(err)
	if arithmeticStatus != lbool.True {
		t.Fatal("empty original arithmetic target did not adopt source sieve plugin", arithmeticStatus)
	}
	intervalStatus, err := e.QueryPredicate(i)
	must(err)
	if intervalStatus != lbool.Undef {
		t.Fatal("raw interval predicate incorrectly reported precise", intervalStatus)
	}
	intervalQueryRef, err := m.NewRef(ia)
	must(err)
	intervalStatus, err = e.QueryExpression(ia)
	must(err)
	must(m.DelRef(&intervalQueryRef))
	if intervalStatus != lbool.Undef {
		t.Fatal("single raw interval projection incorrectly reported precise", intervalStatus)
	}
	status, err = e.QueryPredicate(out)
	must(err)
	if status != lbool.True {
		t.Fatal("finite output after raw interval query", status)
	}
	answer, err := e.Answer(out)
	must(err)
	must(m.DelRef(&answer))
	yes, err := m.MkTrue()
	must(err)
	query := app(out, yes)
	queryRef, err := m.NewRef(query)
	must(err)
	status, err = e.QueryExpression(query)
	must(err)
	must(m.DelRef(&queryRef))
	if status != lbool.True {
		t.Fatal("finite fallback query precision", status)
	}
	if e.queryClone == nil || e.queryClone.sparseIntervalDatalog == e.sparseIntervalDatalog {
		t.Fatal("heterogeneous clone shares seed owner")
	}
	snapshots, err = e.DatalogRelationSnapshots([]ast.ASTHandle{out})
	must(err)
	if len(snapshots[0].Facts) != 1 || snapshots[0].Facts[0][0] != 1 {
		t.Fatal("expression publication lost finite output", snapshots)
	}
	explicitQuery := app(p, yes)
	explicitRef, err := m.NewRef(explicitQuery)
	must(err)
	status, err = e.QueryExpression(explicitQuery)
	must(err)
	must(m.DelRef(&explicitRef))
	if status != lbool.True {
		t.Fatal("requested sparse query head lost precision", status)
	}
	inherited := e.queryClone.predicates[e.queryClonePredicate].requestedRepresentations
	if len(inherited) != 1 || inherited[0] != "tr_sparse" {
		t.Fatal("query head did not inherit requested sparse kind", inherited)
	}
	// The requested finite producer cannot represent this mixed free-variable
	// signature, so source kind selection falls through to an abstract product.
	mixedQueryRef, err := m.NewRef(body)
	must(err)
	status, err = e.QueryExpression(body)
	must(err)
	must(m.DelRef(&mixedQueryRef))
	if status != lbool.True {
		t.Fatal("mixed query-head fallback", status)
	}
	arithmeticQuery, err := m.MkAnd(app(p, yes), ia)
	must(err)
	arithmeticRef, err := m.NewRef(arithmeticQuery)
	must(err)
	status, err = e.QueryExpression(arithmeticQuery)
	must(err)
	if status != lbool.True {
		t.Fatal("two-tail arithmetic projection must keep source sieve result", status)
	}
	must(m.DelRef(&arithmeticRef))
	arithmeticAnswer, err := e.ExpressionAnswer()
	must(err)
	arithmeticFormula, err := m.RefGet(arithmeticAnswer)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(m, false)
	must(err)
	for _, valuation := range []struct {
		text string
		want bool
	}{{"7", true}, {"8", false}} {
		value, err := m.ArithmeticIntegerNumeralText(valuation.text)
		must(err)
		valueRef, err := m.NewRef(value)
		must(err)
		ground, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(substitution.Substitute(arithmeticFormula, []ast.ASTHandle{value}, ground))
		expression, err := m.RefGet(ground)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(expression, nil, reduced))
		truth, err := m.RefGet(reduced)
		must(err)
		if m.IsTrue(truth) != valuation.want || !m.IsTrue(truth) && !m.IsFalse(truth) {
			t.Fatal("arithmetic query answer lost source interval", valuation.text)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&ground))
		must(m.DelRef(&valueRef))
	}
	must(substitution.Close())
	must(m.DelRef(&arithmeticAnswer))

	seven, err := m.ArithmeticIntegerNumeralText("7")
	must(err)
	sevenRef, err := m.NewRef(seven)
	must(err)
	groundProduct, err := m.MkAnd(app(p, yes), app(i, seven))
	must(err)
	groundProductRef, err := m.NewRef(groundProduct)
	must(err)
	status, err = e.QueryExpression(groundProduct)
	must(err)
	if status != lbool.True {
		t.Fatal("ground product projection lost source sieve precision", status)
	}
	inherited = e.queryClone.predicates[e.queryClonePredicate].requestedRepresentations
	if len(inherited) != 1 || inherited[0] != "tr_sparse" {
		t.Fatal("full-body precision changed first predicate kind", inherited)
	}
	must(m.DelRef(&groundProductRef))
	must(m.DelRef(&sevenRef))
	// Repeated queries must not turn derived finite rows into persistent facts.
	status, err = e.QueryPredicate(out)
	must(err)
	if status != lbool.True {
		t.Fatal("repeat finite query", status)
	}
	blocked, err := m.MkAnd(ma, app(empty, b))
	must(err)
	strengthened, err := m.MkImplies(blocked, oa)
	must(err)
	must(e.UpdateRule(strengthened, "heterogeneous-project"))
	status, err = e.QueryPredicate(out)
	must(err)
	if status != lbool.False {
		t.Fatal("derived finite output was promoted into a seed", status)
	}
	must(e.AddTableFact(empty, []uint32{1}))
	snapshots, err = e.DatalogRelationSnapshots([]ast.ASTHandle{empty})
	must(err)
	if len(snapshots[0].Facts) != 1 || snapshots[0].Facts[0][0] != 1 {
		t.Fatal("finite fact was not immediately published", snapshots)
	}
	status, err = e.QueryPredicate(out)
	must(err)
	if status != lbool.True {
		t.Fatal("finite seed insertion did not restore derived output", status)
	}
}

func TestDatalogSparseIntervalMigratesExistingIntervalSeeds(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("prior-interval")
	must(err)
	i, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(i))
	must(e.SetPredicateRepresentation(i, []string{"interval_relation"}))
	must(e.AddTableFact(i, []uint32{7}))
	if e.intervalDatalog == nil {
		t.Fatal("test did not create original interval seed owner")
	}
	name, err = m.Symbols.Intern("prior-interval-product")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	must(e.SetPredicateRepresentation(p, []string{"tr_sparse", "interval_relation"}))
	finiteName, err := m.Symbols.Intern("favourite-interval-finite")
	must(err)
	finite, err := m.MkFuncDecl(finiteName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(finite))
	must(e.SetPredicateRepresentation(finite, []string{"tr_sparse"}))
	must(e.AddTableFact(finite, []uint32{1}))

	x, err := m.MkVar(0, integer)
	must(err)
	yes, err := m.MkTrue()
	must(err)
	body, err := m.MkApp(i, []ast.ASTHandle{x})
	must(err)
	head, err := m.MkApp(p, []ast.ASTHandle{yes, x})
	must(err)
	rule, err := m.MkImplies(body, head)
	must(err)
	must(e.AddRule(rule, "prior-interval-seed"))
	result, err := e.QueryPredicate(p)
	must(err)
	if result != lbool.True {
		t.Fatal("existing interval seed was lost on mixed activation", result)
	}
	finiteApp, err := m.MkApp(finite, []ast.ASTHandle{yes})
	must(err)
	favouriteQuery, err := m.MkAnd(finiteApp, body)
	must(err)
	favouriteRef, err := m.NewRef(favouriteQuery)
	must(err)
	result, err = e.QueryExpression(favouriteQuery)
	must(err)
	must(m.DelRef(&favouriteRef))
	if result != lbool.True {
		t.Fatal("query source sieve did not preserve precision under interval favourite", result)
	}
	must(e.AddTableFact(i, []uint32{9}))
	result, err = e.QueryPredicate(p)
	must(err)
	if result != lbool.True {
		t.Fatal("interval fact update after migration", result)
	}
}
