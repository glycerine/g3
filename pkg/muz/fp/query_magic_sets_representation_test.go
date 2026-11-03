package fp

// Source: context::mk_fresh_head_predicate and
// rel_context::inherit_predicate_kind preserve requested relation families for
// both adorned and magic predicates, with relation-manager fallback when a
// requested Sparse plugin cannot represent a generated nullary signature.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestQueryMagicSetsInheritsRequestedFiniteRepresentations(t *testing.T) {
	for _, kind := range []string{"tr_hashtable", "tr_sparse", "tr_lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			manager, globals := newPDRLifecycleServices(t)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(globals.SourceSetName("fixedpoint.datalog.magic_sets_for_queries", "true"))
			defaultTable := "hashtable"
			if kind == "tr_hashtable" {
				defaultTable = "sparse"
			}
			must(globals.SourceSetName("fixedpoint.datalog.default_table", defaultTable))
			engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
			must(err)
			defer func() { must(engine.Close()) }()
			must(engine.SelectEngine("datalog"))
			boolean, err := manager.MkBoolSort()
			must(err)
			declare := func(name string) ast.ASTHandle {
				t.Helper()
				symbol, declarationErr := manager.Symbols.Intern("magic-kind-" + kind + "-" + name)
				must(declarationErr)
				declaration, declarationErr := manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
				must(declarationErr)
				must(engine.RegisterPredicate(declaration))
				return declaration
			}
			extensional, predicate := declare("E"), declare("P")
			must(engine.SetPredicateRepresentation(predicate, []string{kind}))
			truth, err := manager.MkTrue()
			must(err)
			must(engine.AddFact(extensional, []ast.ASTHandle{truth}))
			variable, err := manager.MkVar(0, boolean)
			must(err)
			extensionalApp, err := manager.MkApp(extensional, []ast.ASTHandle{variable})
			must(err)
			predicateApp, err := manager.MkApp(predicate, []ast.ASTHandle{variable})
			must(err)
			rule, err := manager.MkImplies(extensionalApp, predicateApp)
			must(err)
			must(engine.AddRule(rule, "magic-kind-rule"))
			query, err := manager.MkApp(predicate, []ast.ASTHandle{truth})
			must(err)
			queryRef, err := manager.NewRef(query)
			must(err)
			defer func() { must(manager.DelRef(&queryRef)) }()
			originals := make(map[ast.ASTHandle]bool, len(engine.predicates))
			for declaration := range engine.predicates {
				originals[declaration] = true
			}
			originalCount := len(engine.predicateOrder)
			status, err := engine.QueryExpression(query)
			must(err)
			if status != lbool.True {
				t.Fatal("magic representation query", status)
			}
			if len(engine.predicateOrder) != originalCount {
				t.Fatal("query-local MagicSets changed persistent predicates")
			}
			clone := engine.queryClone
			if clone == nil {
				t.Fatal("successful expression query did not preserve its clone")
			}
			fresh, concrete, fallback := 0, 0, 0
			for _, declaration := range clone.predicateOrder {
				if originals[declaration] {
					continue
				}
				fresh++
				state := clone.predicates[declaration]
				if state == nil || len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != kind {
					t.Fatal("fresh predicate lost requested representation", declaration, state)
				}
				arity, arityErr := manager.GetArity(declaration)
				must(arityErr)
				if arity == 0 && kind != "tr_hashtable" {
					fallback++
					if state.explicitFiniteRepresentation {
						t.Fatal("unsupported nullary Sparse request became concrete")
					}
					if _, ok := state.table.(*rel.HashTable); !ok {
						t.Fatalf("nullary fallback = %T, want Hash", state.table)
					}
					continue
				}
				concrete++
				if !state.explicitFiniteRepresentation {
					t.Fatal("supported fresh predicate lost explicit representation")
				}
				switch kind {
				case "tr_hashtable":
					if _, ok := state.table.(*rel.HashTable); !ok {
						t.Fatalf("inherited Hash predicate = %T", state.table)
					}
				case "tr_sparse":
					if _, ok := state.table.(*rel.SparseTable); !ok {
						t.Fatalf("inherited Sparse predicate = %T", state.table)
					}
				case "tr_lazy_sparse":
					if _, ok := state.table.(*rel.LazySparseTable); !ok || clone.lazyFinite == nil || !clone.lazyFinite.Owns(state.table) {
						t.Fatalf("inherited LazySparse predicate = %T", state.table)
					}
				}
			}
			if fresh == 0 || concrete == 0 {
				t.Fatal("missing query-local inherited predicates", fresh, concrete)
			}
			if kind != "tr_hashtable" && fallback == 0 {
				t.Fatal("missing nullary compatibility fallback")
			}
			must(engine.clearQueryResources())
			if engine.queryClone != nil || len(engine.predicateOrder) != originalCount {
				t.Fatal("closing the query clone did not roll back MagicSets predicates")
			}
			persistent := engine.predicates[predicate]
			if persistent == nil || !persistent.explicitFiniteRepresentation || len(persistent.requestedRepresentations) != 1 || persistent.requestedRepresentations[0] != kind {
				t.Fatal("query rollback changed the original representation")
			}
			switch kind {
			case "tr_hashtable":
				if _, ok := persistent.table.(*rel.HashTable); !ok {
					t.Fatalf("persistent Hash predicate = %T", persistent.table)
				}
			case "tr_sparse":
				if _, ok := persistent.table.(*rel.SparseTable); !ok {
					t.Fatalf("persistent Sparse predicate = %T", persistent.table)
				}
			case "tr_lazy_sparse":
				if _, ok := persistent.table.(*rel.LazySparseTable); !ok || engine.lazyFinite == nil || !engine.lazyFinite.Owns(persistent.table) {
					t.Fatalf("persistent LazySparse predicate = %T", persistent.table)
				}
			}
		})
	}
}
