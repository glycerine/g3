package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogCheckedPublicationPreservesFiniteRepresentation(t *testing.T) {
	signature, err := rel.NewSignature([]uint64{3})
	if err != nil {
		t.Fatal(err)
	}
	sparse, err := rel.NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if sparse != nil {
			if err := sparse.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	if _, err = sparse.Add(rel.Fact{0}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := finiteHashSnapshot(sparse)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := snapshot.Close(); err != nil {
			t.Error(err)
		}
	}()
	if found, err := snapshot.Contains(rel.Fact{0}); err != nil || !found {
		t.Fatal("hash snapshot", found, err)
	}
	published, err := publishCheckedFactsLike(sparse, []rel.Fact{{2}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := published.(*rel.SparseTable); !ok {
		t.Fatalf("publication changed representation: %T", published)
	}
	if found, err := published.Contains(rel.Fact{0}); err != nil || found {
		t.Fatal("publication retained stale row", found, err)
	}
	if found, err := published.Contains(rel.Fact{2}); err != nil || !found {
		t.Fatal("publication missing checked row", found, err)
	}
	if err = published.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDatalogCheckRelationParameterRunsCheckedSaturation(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
	must(e.globals.SourceSetName("fixedpoint.datalog.unbound_compressor", "false"))
	must(e.AddFact(f.declarations["p"], nil))
	step, err := m.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(e.AddRule(step, "checked-step"))
	step, err = m.MkImplies(f.applications["q"], f.applications["u"])
	must(err)
	must(e.AddRule(step, "checked-goal"))

	before := e.predicates[f.declarations["u"]].table
	status, err := e.QueryPredicate(f.declarations["u"])
	must(err)
	if status != lbool.True || e.compiled {
		t.Fatal("checked saturation result/compiler", status, e.compiled)
	}
	if e.predicates[f.declarations["u"]].checked != nil {
		t.Fatal("checked query owner escaped its scope")
	}
	if e.predicates[f.declarations["u"]].table == before {
		t.Fatal("checked result was not published into persistent storage")
	}
	rows, err := e.predicates[f.declarations["u"]].table.Len()
	must(err)
	if rows != 1 {
		t.Fatal("checked recursive result rows", rows)
	}
	status, err = e.QueryPredicate(f.declarations["u"])
	must(err)
	if status != lbool.True {
		t.Fatal("repeated checked query", status)
	}
}

func TestDatalogCheckRelationLocalPrecedenceAndDocCompression(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "hashtable"))
	local, err := e.params.NewValue()
	must(err)
	key := "datalog.check_relation"
	null, err := m.Symbols.Intern("null")
	must(err)
	must(e.params.SetSymbolName(&local, &key, null))
	must(e.SetQueryParameters(&local))
	must(e.params.Del(&local))
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		t.Fatal("local null did not disable module checker", enabled, err)
	}

	local, err = e.params.NewValue()
	must(err)
	doc, err := m.Symbols.Intern("doc")
	must(err)
	must(e.params.SetSymbolName(&local, &key, doc))
	must(e.SetQueryParameters(&local))
	must(e.params.Del(&local))
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("checked-doc-universal")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	x, err := m.MkVar(0, b)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	must(e.AddRule(px, "checked-doc-all"))
	implication, err := m.MkImplies(px, f.applications["q"])
	must(err)
	must(e.AddRule(implication, "checked-doc-goal"))
	scope, err := e.prepareDatalogUnbound([]ast.ASTHandle{f.declarations["q"]})
	must(err)
	if scope != nil {
		t.Fatal("doc check relation did not disable unbound compression")
	}
	status, err := e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal("doc checked query", status)
	}
}

func TestDatalogCheckRelationSparseBackend(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.check_relation", "sparse"))
	must(e.globals.SourceSetName("fixedpoint.datalog.unbound_compressor", "false"))
	b, err := m.MkBoolSort()
	must(err)
	makePredicate := func(text string) ast.ASTHandle {
		name, err := m.Symbols.Intern(text)
		must(err)
		declaration, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	p, q := makePredicate("checked-sparse-p"), makePredicate("checked-sparse-q")
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(p, []ast.ASTHandle{yes}))
	x, err := m.MkVar(0, b)
	must(err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	must(err)
	qx, err := m.MkApp(q, []ast.ASTHandle{x})
	must(err)
	rule, err := m.MkImplies(px, qx)
	must(err)
	must(e.AddRule(rule, "checked-sparse-step"))

	status, err := e.QueryPredicate(q)
	must(err)
	if status != lbool.True || e.compiled {
		t.Fatal("sparse checked result/compiler", status, e.compiled)
	}
	state := e.predicates[q]
	if state.checked != nil || state.sparse != nil {
		t.Fatal("sparse checked owner escaped query scope")
	}
	facts, err := state.table.Facts()
	must(err)
	if len(facts) != 1 || len(facts[0]) != 1 || facts[0][0] != 1 {
		t.Fatal("sparse checked facts", facts)
	}
	status, err = e.QueryPredicate(q)
	must(err)
	if status != lbool.True {
		t.Fatal("repeated sparse checked query", status)
	}
}

func TestDatalogDefaultTableCheckedParametersRunConcretePair(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "sparse"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	must(e.globals.SourceSetName("fixedpoint.datalog.unbound_compressor", "false"))
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("checked-default-table")
	must(err)
	predicate, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(predicate))
	must(e.SetPredicateRepresentation(predicate, []string{"tr_sparse"}))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(predicate, []ast.ASTHandle{yes}))
	scope, err := e.beginDatalogCheckRelation()
	must(err)
	foundPair := false
	for i := range scope.entries {
		entry := &scope.entries[i]
		if entry.declaration != predicate {
			continue
		}
		primary, checker, err := entry.pair.ConcreteKinds()
		must(err)
		if primary != "sparse" || checker != "hashtable" {
			t.Fatal("live configured pair", primary, checker)
		}
		foundPair = true
	}
	if !foundPair {
		t.Fatal("configured table checker did not wrap predicate")
	}
	must(scope.complete(false))

	status, err := e.QueryPredicate(predicate)
	must(err)
	if status != lbool.True || !e.compiled {
		t.Fatal("checked default table result/compiler", status, e.compiled)
	}
	if _, ok := e.predicates[predicate].table.(*rel.SparseTable); !ok {
		t.Fatalf("checked publication changed default table representation: %T", e.predicates[predicate].table)
	}
	found, err := e.predicates[predicate].table.Contains(rel.Fact{1})
	if err != nil || !found {
		t.Fatal("checked default table fact", found, err)
	}
}

func TestDatalogDefaultTableCheckedParameterPrecedenceAndNull(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "hashtable"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	mode, enabled, err := e.datalogDefaultTableCheck()
	if err != nil || !enabled || mode != "hashtable" {
		t.Fatal("module same-plugin checker", mode, enabled, err)
	}

	local, err := e.params.NewValue()
	must(err)
	checkedKey := "datalog.default_table_checked"
	must(e.params.SetBoolName(&local, &checkedKey, false))
	must(e.SetQueryParameters(&local))
	must(e.params.Del(&local))
	if mode, enabled, err = e.datalogDefaultTableCheck(); err != nil || enabled || mode != "" {
		t.Fatal("local false did not override module true", mode, enabled, err)
	}

	local, err = e.params.NewValue()
	must(err)
	checkerKey := "datalog.default_table_checker"
	null, err := m.Symbols.Intern("null")
	must(err)
	must(e.params.SetSymbolName(&local, &checkerKey, null))
	must(e.SetQueryParameters(&local))
	must(e.params.Del(&local))
	if mode, enabled, err = e.datalogDefaultTableCheck(); err != nil || enabled || mode != "" {
		t.Fatal("local null did not disable module checker", mode, enabled, err)
	}

	must(e.SetQueryParameters(nil))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table", "sparse"))
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "sparse"))
	if mode, enabled, err = e.datalogDefaultTableCheck(); err != nil || !enabled || mode != "sparse" {
		t.Fatal("module sparse same-plugin checker", mode, enabled, err)
	}
	must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "null"))
	if mode, enabled, err = e.datalogDefaultTableCheck(); err != nil || enabled || mode != "" {
		t.Fatal("module null checker", mode, enabled, err)
	}
}

func TestDatalogDefaultTableCheckedUsesCatalogPrimaryFallback(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", "hashtable"))
	checker, enabled, err := e.datalogDefaultTableCheck()
	if err != nil || !enabled || checker != "hashtable" {
		t.Fatal("catalog default sparse primary was not recognized", checker, enabled, err)
	}
}
