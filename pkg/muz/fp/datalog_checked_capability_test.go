package fp

// Source: src/muz/rel/dl_check_table.cpp check_table_plugin capability and
// src/muz/rel/dl_relation_manager.cpp favourite-plugin fallback selection.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogCheckedFavouriteHonorsBothNullaryCapabilities(t *testing.T) {
	for _, tc := range []struct{ primary, checker string }{
		{primary: "sparse", checker: "hashtable"},
		{primary: "lazy_sparse", checker: "hashtable"},
		{primary: "hashtable", checker: "sparse"},
		{primary: "hashtable", checker: "lazy_sparse"},
	} {
		name := tc.primary + "-" + tc.checker
		t.Run(name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			must(e.globals.SourceSetName("fixedpoint.datalog.default_table", tc.primary))
			must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checked", "true"))
			must(e.globals.SourceSetName("fixedpoint.datalog.default_table_checker", tc.checker))
			must(e.globals.SourceSetName("fixedpoint.datalog.unbound_compressor", "false"))

			boolean, err := m.MkBoolSort()
			must(err)
			declare := func(suffix string, domain []ast.ASTHandle) ast.ASTHandle {
				t.Helper()
				symbol, declarationErr := m.Symbols.Intern("checked-capability-" + name + "-" + suffix)
				must(declarationErr)
				declaration, declarationErr := m.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
				must(declarationErr)
				must(e.RegisterPredicate(declaration))
				return declaration
			}
			unary := declare("unary", []ast.ASTHandle{boolean})
			nullary := declare("nullary", nil)
			truth, err := m.MkTrue()
			must(err)
			must(e.AddFact(unary, []ast.ASTHandle{truth}))

			// check_table_plugin is the favourite table plugin: the unary
			// predicate's table is a check table from its creation on.
			unaryPair, ok := e.predicates[unary].table.(*rel.FiniteCheckedPair)
			if !ok {
				t.Fatalf("unary predicate table = %T", e.predicates[unary].table)
			}
			primary, checker, err := unaryPair.ConcreteKinds()
			must(err)
			if primary != tc.primary || checker != tc.checker {
				t.Fatal("unary checked concrete kinds", primary, checker)
			}
			unarySignature, err := e.predicates[unary].table.Signature()
			must(err)
			unaryTemporary, err := e.relations.NewDefaultFiniteTable(unarySignature)
			must(err)
			pair, ok := unaryTemporary.(*rel.FiniteCheckedPair)
			if !ok {
				t.Fatalf("supported compiler temporary type = %T", unaryTemporary)
			}
			primary, checker, err = pair.ConcreteKinds()
			must(err)
			if primary != tc.primary || checker != tc.checker {
				t.Fatal("compiler temporary concrete kinds", primary, checker)
			}
			must(unaryTemporary.Close())

			nullarySignature, err := e.predicates[nullary].table.Signature()
			must(err)
			nullaryTemporary, err := e.relations.NewDefaultFiniteTable(nullarySignature)
			must(err)
			if _, checked := nullaryTemporary.(*rel.FiniteCheckedPair); checked {
				t.Fatal("compiler constructed a checked nullary pair outside factory capability")
			}
			if _, hash := nullaryTemporary.(*rel.HashTable); !hash {
				t.Fatalf("nullary compiler fallback type = %T", nullaryTemporary)
			}
			must(nullaryTemporary.Close())

			variable, err := m.MkVar(0, boolean)
			must(err)
			unaryApplication, err := m.MkApp(unary, []ast.ASTHandle{variable})
			must(err)
			nullaryApplication, err := m.MkApp(nullary, nil)
			must(err)
			rule, err := m.MkImplies(unaryApplication, nullaryApplication)
			must(err)
			must(e.addRuleEager(rule, "checked-capability-step"))
			for iteration := 0; iteration < 2; iteration++ {
				status, queryErr := e.QueryPredicate(nullary)
				must(queryErr)
				if status != lbool.True || !e.compiled {
					t.Fatal("checked unary-to-nullary query", iteration, status, e.compiled)
				}
				if _, checked := e.predicates[unary].table.(*rel.FiniteCheckedPair); !checked {
					t.Fatalf("unary check table type = %T", e.predicates[unary].table)
				}
				if _, hash := e.predicates[nullary].table.(*rel.HashTable); !hash {
					t.Fatalf("published nullary fallback type = %T", e.predicates[nullary].table)
				}
				postSignature, signatureErr := e.predicates[unary].table.Signature()
				must(signatureErr)
				postTemporary, temporaryErr := e.relations.NewDefaultFiniteTable(postSignature)
				must(temporaryErr)
				if _, checked := postTemporary.(*rel.FiniteCheckedPair); !checked {
					t.Fatalf("compiler temporary after the query = %T", postTemporary)
				}
				must(postTemporary.Close())
			}
		})
	}
}
