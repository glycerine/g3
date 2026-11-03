package fp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newDirectFactFixture(t *testing.T) *pdrLifecycleFixture {
	t.Helper()
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, e.Close()) })
	b, err := m.MkBoolSort()
	fragmentMust(t, err)
	f := &pdrLifecycleFixture{manager: m, engine: e, declarations: make(map[string]ast.ASTHandle), applications: make(map[string]ast.ASTHandle)}
	for _, name := range []string{"p", "q", "u"} {
		symbol, err := m.Symbols.Intern("direct-fact-" + name)
		fragmentMust(t, err)
		d, err := m.MkFuncDecl(symbol, nil, b, ast.ASTFuncDeclInfo{})
		fragmentMust(t, err)
		fragmentMust(t, e.RegisterPredicate(d))
		app, err := m.MkApp(d, nil)
		fragmentMust(t, err)
		f.declarations[name] = d
		f.applications[name] = app
	}
	return f
}

func TestDatalogDirectFactsPersistAcrossExpressionAndDirectQueries(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	// This fixture checks publication of materialized intermediate tables.
	// Keep their defining rules instead of substituting them into the query.
	for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear"} {
		must(e.globals.SourceSetName(key, "false"))
	}
	must(e.Push())
	must(e.Pop())
	must(e.Push())
	must(e.AddFact(f.declarations["p"], nil))
	must(e.AddTableFact(f.declarations["p"], nil))
	if err := e.Pop(); err == nil {
		t.Fatal("Datalog fact insertion did not instantiate engine before pop")
	}
	if len(e.scopes) != 1 {
		t.Fatal("refused pop changed scope")
	}
	forms, err := e.Rules()
	must(err)
	if len(forms) != 0 || len(e.rawRules) != 0 {
		t.Fatal("direct tuples became rules", len(forms), len(e.rawRules))
	}
	var text bytes.Buffer
	must(e.WriteContextSMT2(&text, nil, nil))
	if strings.Contains(text.String(), "(rule") {
		t.Fatal("tuple leaked into source rule display", text.String())
	}
	rows, err := e.predicates[f.declarations["p"]].table.Len()
	must(err)
	if rows != 1 {
		t.Fatal("duplicate direct tuple", rows)
	}
	step, err := m.MkImplies(f.applications["p"], f.applications["q"])
	must(err)
	must(e.AddRule(step, "edge"))
	query, err := m.MkAnd(f.applications["p"], f.applications["q"])
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	defer func() { must(m.DelRef(&queryRef)) }()
	prior := e.predicates[f.declarations["p"]].table
	status, err := e.QueryExpression(query)
	must(err)
	if status != lbool.True {
		t.Fatal("expression clone lost table inputs", status)
	}
	if _, err := prior.Len(); !errors.Is(err, rel.ErrTable) {
		t.Fatal("replaced persistent table remained live", err)
	}
	if e.queryClone.predicates[f.declarations["p"]].table == e.predicates[f.declarations["p"]].table {
		t.Fatal("clone and parent share table ownership")
	}
	rows, err = e.predicates[f.declarations["q"]].table.Len()
	must(err)
	if rows != 1 {
		t.Fatal("successful expression did not publish derived tuples", rows)
	}
	status, err = e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal("direct query lost persistent tables after clone cleanup", status)
	}
	must(e.AddFact(f.declarations["u"], nil))
	if _, err := e.LastResult(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("direct insertion left stale result valid", err)
	}
	status, err = e.QueryPredicate(f.declarations["u"])
	must(err)
	if status != lbool.True {
		t.Fatal("post-query fact unavailable", status)
	}
}

func TestDatalogFactSelectionUsesParametersAndCachesSourceChoice(t *testing.T) {
	for _, tc := range []struct{ name, module, local, want string }{
		{"default-auto", "", "", "datalog"},
		{"auto-pdr-rule", "", "", "pdr"},
		{"module-pdr", "pdr", "", "pdr"},
		{"local-datalog", "pdr", "datalog", "datalog"},
		{"local-pdr", "datalog", "pdr", "pdr"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			if tc.module != "" {
				must(e.globals.SourceSetName("fixedpoint.engine", tc.module))
			}
			if tc.local != "" {
				value, err := e.params.NewValue()
				must(err)
				key := "engine"
				symbol, err := e.manager.Symbols.Intern(tc.local)
				must(err)
				must(e.params.SetSymbolName(&value, &key, symbol))
				must(e.SetQueryParameters(&value))
				must(e.params.Del(&value))
			}
			rawCount := 1
			if tc.name == "auto-pdr-rule" {
				zero, err := e.manager.ArithmeticIntegerNumeralText("0")
				must(err)
				integer, err := e.manager.GetSort(zero)
				must(err)
				boolean, err := e.manager.MkBoolSort()
				must(err)
				name, err := e.manager.Symbols.Intern("direct-auto-integer")
				must(err)
				d, err := e.manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				fact, err := e.manager.MkApp(d, []ast.ASTHandle{zero})
				must(err)
				must(e.AddRule(fact, "auto-integer-rule"))
				rawCount++
			}
			must(e.Push())
			must(e.AddFact(f.declarations["p"], nil))
			if e.backend != tc.want {
				t.Fatal("wrong fact engine", e.backend, tc.want)
			}
			if tc.want == "datalog" {
				if len(e.rawRules) != 0 || !e.engineInstantiated {
					t.Fatal("Datalog fact was not direct")
				}
			} else {
				if len(e.rawRules) != rawCount || e.engineInstantiated {
					t.Fatal("non-Datalog raw fact changed")
				}
				must(e.Pop())
			}
			next := "pdr"
			if tc.want == "pdr" {
				next = "datalog"
			}
			must(e.globals.SourceSetName("fixedpoint.engine", next))
			must(e.SetQueryParameters(nil))
			must(e.SelectConfiguredEngine())
			if e.backend != tc.want {
				t.Fatal("configured source engine choice was recomputed", e.backend, tc.want)
			}
			must(e.SelectEngine(next))
			if e.backend != next {
				t.Fatal("explicit Go override lost", e.backend, next)
			}
		})
	}
}

func TestDatalogDirectFiniteFactValidationAndCloneOwnership(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("direct-finite")
	must(err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	yes, err := m.MkTrue()
	must(err)
	must(e.AddFact(d, []ast.ASTHandle{yes}))
	must(e.AddTableFact(d, []uint32{1}))
	if err := e.AddTableFact(d, []uint32{2}); err == nil {
		t.Fatal("invalid finite value accepted")
	}
	if err := e.AddFact(d, nil); err == nil {
		t.Fatal("invalid tuple arity accepted")
	}
	rows, err := e.predicates[d].table.Len()
	must(err)
	if rows != 1 {
		t.Fatal("invalid/duplicate tuple mutated table", rows)
	}
	clone, err := e.newQueryClone()
	must(err)
	if clone.predicates[d].table == e.predicates[d].table {
		t.Fatal("clone borrowed source table")
	}
	must(clone.predicates[d].table.Reset())
	rows, err = e.predicates[d].table.Len()
	must(err)
	if rows != 1 {
		t.Fatal("clone mutation escaped")
	}
	must(clone.Close())
	rows, err = e.predicates[d].table.Len()
	must(err)
	if rows != 1 {
		t.Fatal("clone cleanup retired parent table")
	}
}
