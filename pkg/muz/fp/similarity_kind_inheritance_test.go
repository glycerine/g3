package fp

// Source: dl_mk_similarity_compressor.cpp merge_class passes its source head
// to context::mk_fresh_head_predicate, which inherits the requested kind.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogSimilarityInheritsRequestedFiniteKind(t *testing.T) {
	for _, kind := range []string{"tr_hashtable", "tr_sparse", "tr_lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			engine, manager := newFragmentTestEngine(t, "datalog")
			favourite := "hashtable"
			if kind == "tr_hashtable" {
				favourite = "sparse"
			}
			engine.datalogDefaultTable = favourite
			fragmentMust(t, engine.configureDatalogFiniteDefault(favourite))
			name, err := manager.Symbols.Intern("similarity-inherited-domain")
			fragmentMust(t, err)
			sort, err := manager.MkDatalogFiniteSort(name, 16)
			fragmentMust(t, err)
			boolean, err := manager.MkBoolSort()
			fragmentMust(t, err)
			name, err = manager.Symbols.Intern("similarity-inherited-P")
			fragmentMust(t, err)
			predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
			fragmentMust(t, err)
			fragmentMust(t, engine.RegisterPredicate(predicate))
			fragmentMust(t, engine.SetPredicateRepresentation(predicate, []string{kind}))
			original := engine.predicates[predicate].table
			addRule := func(value uint64) {
				t.Helper()
				constant, err := manager.MkDatalogNumeral(value, sort)
				fragmentMust(t, err)
				atom, err := manager.MkApp(predicate, []ast.ASTHandle{constant})
				fragmentMust(t, err)
				fragmentMust(t, engine.addRuleEager(atom, ""))
			}
			for value := uint64(0); value < 12; value++ {
				addRule(value)
			}
			goal := similarityTestGoal(t, engine, manager, predicate, sort)
			checkKind := func(table rel.FiniteTable) {
				t.Helper()
				switch kind {
				case "tr_hashtable":
					if _, ok := table.(*rel.HashTable); !ok {
						t.Fatalf("inherited auxiliary %T, want Hash", table)
					}
				case "tr_sparse":
					if _, ok := table.(*rel.SparseTable); !ok {
						t.Fatalf("inherited auxiliary %T, want Sparse", table)
					}
				case "tr_lazy_sparse":
					if _, ok := table.(*rel.LazySparseTable); !ok || engine.lazyFinite == nil || !engine.lazyFinite.Owns(table) {
						t.Fatalf("inherited auxiliary %T lost the original LazySparse engine", table)
					}
				}
			}
			checkKind(original)
			for iteration := 0; iteration < 3; iteration++ {
				count := 12
				if iteration == 2 {
					addRule(12)
					count++
				}
				status, err := engine.QueryPredicate(goal)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("inherited similarity query", status)
				}
				if len(engine.auxiliary) != 0 {
					t.Fatal("query leaked generated similarity declarations")
				}
				beforeTransform := engine.predicates[predicate].table
				checkKind(beforeTransform)
				finish := similarityTestInspectScope(t, engine, goal)
				if len(engine.auxiliary) != 1 {
					t.Fatal("expected one compressed class", len(engine.auxiliary))
				}
				for auxiliary := range engine.auxiliary {
					state := engine.predicates[auxiliary]
					if state == nil || !state.explicitFiniteRepresentation || len(state.requestedRepresentations) != 1 || state.requestedRepresentations[0] != kind {
						t.Fatal("auxiliary lost requested source kind", state)
					}
					checkKind(state.table)
					for value := rel.Element(0); value < 16; value++ {
						found, err := state.table.Contains(rel.Fact{value})
						fragmentMust(t, err)
						if found != (int(value) < count) {
							t.Fatal("auxiliary constant tuple", value, found, count)
						}
					}
					marked, err := engine.relations.IsSaturated(auxiliary)
					fragmentMust(t, err)
					if !marked {
						t.Fatal("inherited auxiliary was not saturated")
					}
				}
				finish()
				if engine.predicates[predicate].table != beforeTransform {
					t.Fatal("similarity replaced the explicitly selected source owner")
				}
				output, found, err := engine.relations.TryGetFiniteRelation(goal)
				fragmentMust(t, err)
				if !found {
					t.Fatal("missing compiled goal output")
				}
				for value := rel.Element(0); value < 16; value++ {
					found, err := output.Contains(rel.Fact{value})
					fragmentMust(t, err)
					if found != (int(value) < count) {
						t.Fatal("compiled inherited similarity output", value, found, count)
					}
				}
			}
		})
	}
}
