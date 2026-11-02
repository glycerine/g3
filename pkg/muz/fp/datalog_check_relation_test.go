package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

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
