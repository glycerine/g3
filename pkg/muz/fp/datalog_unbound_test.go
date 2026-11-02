package fp

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogUnboundCompressorLiveParametersAndScopedRules(t *testing.T) {
	for _, tc := range []struct {
		name, module, local, relation string
		compressed                    bool
	}{
		{name: "default", compressed: true},
		{name: "module-disabled", module: "false"},
		{name: "local-disabled", module: "true", local: "false"},
		{name: "local-enabled", module: "false", local: "true", compressed: true},
		{name: "doc-disables", local: "true", relation: "doc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			if tc.module != "" {
				must(e.globals.SourceSetName("fixedpoint.datalog.unbound_compressor", tc.module))
			}
			value, err := e.params.NewValue()
			must(err)
			if tc.local != "" {
				key := "datalog.unbound_compressor"
				must(e.params.SetBoolName(&value, &key, tc.local == "true"))
			}
			if tc.relation != "" {
				key := "datalog.default_relation"
				sym, err := m.Symbols.Intern(tc.relation)
				must(err)
				must(e.params.SetSymbolName(&value, &key, sym))
			}
			must(e.SetQueryParameters(&value))
			must(e.params.Del(&value))
			b, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("unbound-universal")
			must(err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			x, err := m.MkVar(0, b)
			must(err)
			px, err := m.MkApp(p, []ast.ASTHandle{x})
			must(err)
			must(e.AddRule(px, "all"))
			implication, err := m.MkImplies(px, f.applications["q"])
			must(err)
			must(e.AddRule(implication, "goal"))
			before, err := e.set.Len()
			must(err)
			valid, err := e.set.Finalize()
			must(err)
			if !valid {
				t.Fatal("fixture not stratified")
			}
			scope, err := e.prepareDatalogUnbound([]ast.ASTHandle{f.declarations["q"]})
			must(err)
			if (scope != nil) != tc.compressed {
				t.Fatalf("compressed=%v, want %v", scope != nil, tc.compressed)
			}
			if scope != nil {
				var temporary rel.FiniteTable
				for decl := range e.auxiliary {
					temporary = e.predicates[decl].table
				}
				if temporary == nil {
					t.Fatal("compression did not instantiate finite auxiliary table")
				}
				must(scope.Close())
				if _, err := temporary.Len(); err == nil {
					t.Fatal("scoped table remained live")
				}
			}
			status, err := e.QueryPredicate(f.declarations["q"])
			must(err)
			if status != lbool.True {
				t.Fatal("unbound-rule query", status)
			}
			if len(e.auxiliary) != 0 {
				t.Fatal("scoped compression leaked auxiliary predicates")
			}
			after, err := e.set.Len()
			must(err)
			if after != before {
				t.Fatalf("scoped compression changed original rule count: %d -> %d", before, after)
			}
			status, err = e.QueryPredicate(p)
			must(err)
			if status != lbool.True {
				t.Fatal("later original-predicate query lost universal rule", status)
			}
			count, err := e.predicates[p].table.Len()
			must(err)
			if count != 2 {
				t.Fatal("original Boolean relation lost tuples", count)
			}
			if tc.name == "default" {
				for _, enabled := range []bool{false, true} {
					updated, err := e.params.NewValue()
					must(err)
					key := "datalog.unbound_compressor"
					must(e.params.SetBoolName(&updated, &key, enabled))
					must(e.SetQueryParameters(&updated))
					must(e.params.Del(&updated))
					scope, err := e.prepareDatalogUnbound([]ast.ASTHandle{f.declarations["q"]})
					must(err)
					if (scope != nil) != enabled {
						t.Fatal("parameter update did not change compression", enabled)
					}
					if scope != nil {
						must(scope.Close())
					}
				}
			}
		})
	}
}

func TestDatalogUnboundCompressorDifferentColumnsAndNegation(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	b, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("unbound-two-columns")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	x, err := m.MkVar(0, b)
	must(err)
	yes, err := m.MkTrue()
	must(err)
	no, err := m.MkFalse()
	must(err)
	app := func(a, b ast.ASTHandle) ast.ASTHandle {
		v, err := m.MkApp(p, []ast.ASTHandle{a, b})
		must(err)
		return v
	}
	must(e.AddRule(app(x, no), "first-column"))
	must(e.AddRule(app(yes, x), "second-column"))
	absent := app(no, yes)
	rule, err := m.MkImplies(absent, f.applications["q"])
	must(err)
	must(e.AddRule(rule, "absent"))
	negative, err := m.MkNot(absent)
	must(err)
	rule, err = m.MkImplies(negative, f.applications["u"])
	must(err)
	must(e.AddRule(rule, "negative"))
	status, err := e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.False {
		t.Fatal("compression invented missing tuple", status)
	}
	for decl := range e.auxiliary {
		name, err := m.GetASTName(decl)
		must(err)
		text, _, err := m.Symbols.BareString(name)
		must(err)
		if strings.Contains(text, "compr_arg_") {
			t.Fatal("compressed predicate escaped scope", text)
		}
	}

	status, err = e.QueryPredicate(f.declarations["u"])
	must(err)
	if status != lbool.True {
		t.Fatal("negative compressed query", status)
	}
	status, err = e.QueryPredicate(p)
	must(err)
	if status != lbool.True {
		t.Fatal("original relation query", status)
	}
	count, err := e.predicates[p].table.Len()
	must(err)
	if count != 3 {
		t.Fatal("original relation union", count)
	}
}
