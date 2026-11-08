package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogQueriesRestoreFilterAndJoinScopes(t *testing.T) {
	for _, kind := range []string{"filter", "join"} {
		t.Run(kind, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			goal := f.declarations["u"]
			if kind == "filter" {
				b, err := m.MkBoolSort()
				must(err)
				name, err := m.Symbols.Intern("scoped-filter-P")
				must(err)
				p, err := m.MkFuncDecl(name, []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(p))
				yes, err := m.MkTrue()
				must(err)
				must(e.AddFact(p, []ast.ASTHandle{yes}))
				app, err := m.MkApp(p, []ast.ASTHandle{yes})
				must(err)
				rule, err := m.MkImplies(app, f.applications["u"])
				must(err)
				must(e.addRuleEager(rule, "filter-consumer"))
			} else {
				must(e.AddFact(f.declarations["p"], nil))
				must(e.AddFact(f.declarations["q"], nil))
				b, err := m.MkBoolSort()
				must(err)
				name, err := m.Symbols.Intern("scoped-join-R")
				must(err)
				p, err := m.MkFuncDecl(name, nil, b, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(p))
				must(e.AddFact(p, nil))
				app, err := m.MkApp(p, nil)
				must(err)
				body, err := m.MkAnd(f.applications["p"], f.applications["q"])
				must(err)
				body, err = m.MkAnd(body, app)
				must(err)
				rule, err := m.MkImplies(body, f.applications["u"])
				must(err)
				must(e.addRuleEager(rule, "join-consumer"))
			}
			original := e.set
			count, err := e.set.Len()
			must(err)
			predicates := len(e.predicates)
			order := len(e.predicateOrder)
			for i := 0; i < 3; i++ {
				status, err := e.QueryPredicate(goal)
				must(err)
				if status != lbool.True {
					t.Fatal("query", i, status)
				}
				got, err := e.set.Len()
				must(err)
				if e.set != original || got != count {
					t.Fatal("query rewrote persistent source rules", i, count, got)
				}
				if len(e.predicates) != predicates || len(e.predicateOrder) != order || len(e.auxiliary) != 0 {
					t.Fatal("query leaked generated predicates", i, predicates, len(e.predicates), len(e.auxiliary))
				}
			}
			zero, err := m.ArithmeticIntegerNumeralText("0")
			must(err)
			integer, err := m.GetSort(zero)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("scoped-infinite-output")
			must(err)
			infinite, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(infinite))
			if _, err = e.QueryPredicates([]ast.ASTHandle{goal, infinite}); err == nil {
				t.Fatal("unsupported infinite output accepted")
			}
			got, err := e.set.Len()
			must(err)
			if e.set != original || got != count || len(e.predicates) != predicates+1 || len(e.auxiliary) != 0 {
				t.Fatal("failed query did not restore source program")
			}
			outputs, err := e.set.OutputPredicates()
			must(err)
			if len(outputs) != 0 {
				t.Fatal("query output marks escaped scope", outputs)
			}

		})
	}
}
