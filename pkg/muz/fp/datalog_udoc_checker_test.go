package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogDocFormulaCheckerExecutesActualJoinsAndProjection(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.check_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	small, err := m.BitVectorSort(2)
	must(err)
	wide, err := m.BitVectorSort(4)
	must(err)
	declare := func(name string, sorts ...ast.ASTHandle) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, sorts, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("checked-doc-p", small, wide)
	q := declare("checked-doc-q", small)
	r := declare("checked-doc-r", wide)
	goal := declare("checked-doc-goal")
	must(e.AddTableFact(p, []uint32{2, 9}))
	must(e.AddTableFact(p, []uint32{1, 8}))
	must(e.AddTableFact(q, []uint32{2}))
	if e.udocDatalog == nil || !e.udocDatalog.factory.FormulaChecked() || e.predicates[p].table != nil {
		t.Fatal("doc checker did not select actual checked UDoc storage")
	}
	x, err := m.MkVar(0, small)
	must(err)
	y, err := m.MkVar(1, wide)
	must(err)
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		a, err := m.MkApp(d, args)
		must(err)
		return a
	}
	pa, qa, ra := app(p, x, y), app(q, x), app(r, y)
	nine, err := m.BitVectorNumeralUint64(9, 4)
	must(err)
	equal, err := m.MkEq(y, nine)
	must(err)
	body, err := m.MkAndArgs([]ast.ASTHandle{pa, qa, equal})
	must(err)
	rule, err := m.MkImplies(body, ra)
	must(err)
	must(e.AddRule(rule, "checked-doc-join-filter-project"))
	goalApp := app(goal)
	rule, err = m.MkImplies(ra, goalApp)
	must(err)
	must(e.AddRule(rule, "checked-doc-project-nullary"))
	for run := 0; run < 2; run++ {
		status, err := e.QueryPredicate(r)
		must(err)
		if status != lbool.True {
			t.Fatal("checked UDoc result", run, status)
		}
		for _, d := range []ast.ASTHandle{p, q, r, goal} {
			if result := e.udocDatalog.result[d]; result == nil || !result.FormulaChecked() {
				t.Fatal("derived relation lost doc formula checking", d)
			}
		}
		nine, err = m.BitVectorNumeralUint64(9, 4)
		must(err)
		if found, err := e.udocDatalog.result[r].ContainsFact([]ast.ASTHandle{nine}); err != nil || !found {
			t.Fatal("joined checked result", found, err)
		}
		eight, err := m.BitVectorNumeralUint64(8, 4)
		must(err)
		if found, err := e.udocDatalog.result[r].ContainsFact([]ast.ASTHandle{eight}); err != nil || found {
			t.Fatal("join admitted unmatched tuple", found, err)
		}
		if found, err := e.udocDatalog.result[goal].ContainsFact(nil); err != nil || !found {
			t.Fatal("nullary checked result", found, err)
		}
	}
	status, err := e.QueryExpression(ra)
	must(err)
	if status != lbool.True || e.queryClone == nil || e.queryClone.udocDatalog == nil || !e.queryClone.udocDatalog.factory.FormulaChecked() {
		t.Fatal("expression clone lost actual doc checker", status)
	}
}

func TestDatalogDocFormulaCheckerKeepsBooleanFallbackAndNullaryDOC(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "doc"))
	boolean, err := m.MkBoolSort()
	must(err)
	n, err := m.Symbols.Intern("checked-doc-bool-fallback")
	must(err)
	p, err := m.MkFuncDecl(n, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	truth, err := m.MkTrue()
	must(err)
	must(e.AddFact(p, []ast.ASTHandle{truth}))
	original := e.predicates[p].table
	scope, err := e.beginDatalogCheckRelation()
	must(err)
	if scope == nil {
		t.Fatal("nullary doc scope was omitted")
	}
	if e.predicates[p].table != original || e.predicates[p].checked != nil {
		t.Fatal("doc accepted unsupported Boolean signature")
	}
	goal := e.predicates[f.declarations["q"]]
	doc, ok := goal.table.(*rel.UDocNullaryTable)
	if !ok {
		t.Fatal("source-capable nullary did not use actual doc", goal.table)
	}
	if added, err := doc.Add(nil); err != nil || !added {
		t.Fatal("nullary doc insertion", added, err)
	}
	if count, err := doc.Len(); err != nil || count != 1 {
		t.Fatal("nullary doc truth", count, err)
	}
	must(scope.complete(false))
	if e.predicates[p].table != original {
		t.Fatal("fallback scope changed original Boolean storage")
	}
	if _, ok := goal.table.(*rel.UDocNullaryTable); ok {
		t.Fatal("temporary doc scope escaped")
	}
}

func TestDatalogDocFormulaCheckerPreservesLegacyMixedPositiveProgram(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.check_relation", "doc"))
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	bv, err := m.BitVectorSort(3)
	must(err)
	declare := func(name string, sort ast.ASTHandle) ast.ASTHandle {
		n, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(n, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p := declare("legacy-doc-mixed-bool", boolean)
	q := declare("legacy-doc-mixed-bv", bv)
	r := declare("legacy-doc-mixed-result", bv)
	truth, err := m.MkTrue()
	must(err)
	value, err := m.BitVectorNumeralUint64(5, 3)
	must(err)
	must(e.AddFact(p, []ast.ASTHandle{truth}))
	must(e.AddFact(q, []ast.ASTHandle{value}))
	x, err := m.MkVar(0, boolean)
	must(err)
	y, err := m.MkVar(1, bv)
	must(err)
	pa, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qa, err := m.MkApp(q, []ast.ASTHandle{y})
	must(err)
	ra, err := m.MkApp(r, []ast.ASTHandle{y})
	must(err)
	body, err := m.MkAndArgs([]ast.ASTHandle{pa, qa})
	must(err)
	rule, err := m.MkImplies(body, ra)
	must(err)
	must(e.AddRule(rule, "legacy-doc-mixed-positive"))
	for run := 0; run < 2; run++ {
		status, err := e.QueryPredicate(r)
		must(err)
		if status != lbool.True {
			t.Fatal("previously supported mixed program", run, status)
		}
		if e.udocDatalog != nil {
			t.Fatal("mixed legacy fallback claimed homogeneous DOC execution")
		}
		facts, err := e.predicates[r].table.Facts()
		must(err)
		if len(facts) != 1 || len(facts[0]) != 1 || facts[0][0] != 5 {
			t.Fatal("mixed finite result", facts)
		}
	}
}

func TestDatalogDocFormulaCheckerParameterClearPreservesAdoptedFacts(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	boolean, err := m.MkBoolSort()
	must(err)
	bv, err := m.BitVectorSort(3)
	must(err)
	name, err := m.Symbols.Intern("doc-local-parameter-fact")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{bv}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	must(e.AddTableFact(p, []uint32{5}))
	local, err := e.params.NewValue()
	must(err)
	doc, err := m.Symbols.Intern("doc")
	must(err)
	key := "datalog.check_relation"
	must(e.params.SetSymbolName(&local, &key, doc))
	must(e.PrepareQueryParameters(&local))
	must(e.params.Del(&local))
	status, err := e.QueryPredicate(p)
	must(err)
	if status != lbool.True || e.udocDatalog == nil {
		t.Fatal("initial checked doc query", status)
	}
	owner := e.udocDatalog
	must(e.PrepareQueryParameters(nil))
	status, err = e.QueryPredicate(p)
	must(err)
	if status != lbool.True || e.udocDatalog != owner {
		t.Fatal("parameter clearing discarded adopted facts", status)
	}
	value, err := m.BitVectorNumeralUint64(5, 3)
	must(err)
	if present, err := owner.result[p].ContainsFact([]ast.ASTHandle{value}); err != nil || !present {
		t.Fatal("cached relation lost its original tuple", present, err)
	}
	must(e.AddTableFact(p, []uint32{6}))
	status, err = e.QueryPredicate(p)
	must(err)
	if status != lbool.True {
		t.Fatal("query after parameter clearing/fact addition", status)
	}
	for _, number := range []uint64{5, 6} {
		value, err := m.BitVectorNumeralUint64(number, 3)
		must(err)
		if present, err := owner.result[p].ContainsFact([]ast.ASTHandle{value}); err != nil || !present {
			t.Fatal("cached relation after addition", number, present, err)
		}
	}
}
