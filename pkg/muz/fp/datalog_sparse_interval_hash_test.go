package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogSparseIntervalHashAndNullaryOriginals(t *testing.T) {
	for _, tc := range []struct {
		name, favourite string
		explicit        bool
	}{
		{"default-hash", "hashtable", false},
		{"explicit-hash", "sparse", true},
		{"sparse-nullary-fallback", "sparse", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(g.SourceSetName("fixedpoint.datalog.default_table", tc.favourite))
			must(e.SelectEngine("datalog"))
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
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
			mixed := declare("hash-mixed", []ast.ASTHandle{boolean, integer}, []string{"tr_sparse", "interval_relation"})
			var kinds []string
			if tc.explicit {
				kinds = []string{"tr_hashtable"}
			}
			finite := declare("hash-finite", []ast.ASTHandle{boolean}, kinds)
			goal := declare("hash-nullary", nil, kinds)
			blocker := declare("hash-blocker", nil, kinds)
			must(e.AddTableFact(finite, []uint32{1}))
			must(e.AddTableFact(mixed, []uint32{1, 7}))
			b, err := m.MkVar(0, boolean)
			must(err)
			x, err := m.MkVar(1, integer)
			must(err)
			pa, err := m.MkApp(finite, []ast.ASTHandle{b})
			must(err)
			ma, err := m.MkApp(mixed, []ast.ASTHandle{b, x})
			must(err)
			ga, err := m.MkApp(goal, nil)
			must(err)
			body, err := m.MkAnd(pa, ma)
			must(err)
			rule, err := m.MkImplies(body, ga)
			must(err)
			must(e.addRuleEager(rule, "hash-nullary-rule"))
			status, err := e.QueryPredicate(goal)
			must(err)
			if status != lbool.True {
				t.Fatal("exact nullary result", status)
			}
			if tc.favourite == "sparse" && !tc.explicit {
				if _, ok := e.predicates[finite].table.(*rel.SparseTable); !ok {
					t.Fatalf("finite sparse fallback changed: %T", e.predicates[finite].table)
				}
			} else if _, ok := e.predicates[finite].table.(*rel.HashTable); !ok {
				t.Fatalf("public Hash representation changed: %T", e.predicates[finite].table)
			}
			if _, ok := e.predicates[goal].table.(*rel.HashTable); !ok {
				t.Fatalf("nullary fallback changed: %T", e.predicates[goal].table)
			}
			snapshots, err := e.DatalogRelationSnapshots([]ast.ASTHandle{finite, goal})
			must(err)
			if len(snapshots) != 2 || len(snapshots[0].Facts) != 1 || snapshots[0].Facts[0][0] != 1 || len(snapshots[1].Facts) != 1 || len(snapshots[1].Facts[0]) != 0 {
				t.Fatal("Hash/nullary snapshots", snapshots)
			}
			status, err = e.QueryExpression(ga)
			must(err)
			if status != lbool.True {
				t.Fatal("exact nullary expression", status)
			}
			if _, ok := e.predicates[goal].table.(*rel.HashTable); !ok {
				t.Fatal("clone publication changed Hash kind")
			}
			blocked, err := m.MkApp(blocker, nil)
			must(err)
			guarded, err := m.MkAnd(body, blocked)
			must(err)
			strengthened, err := m.MkImplies(guarded, ga)
			must(err)
			must(e.UpdateRule(strengthened, "hash-nullary-rule"))
			status, err = e.QueryPredicate(goal)
			must(err)
			if status != lbool.False {
				t.Fatal("nullary derived result became seed", status)
			}
			must(e.AddTableFact(blocker, nil))
			rows, err := e.predicates[blocker].table.Facts()
			must(err)
			if len(rows) != 1 || len(rows[0]) != 0 {
				t.Fatal("nullary insertion did not publish exact unit", rows)
			}
			status, err = e.QueryPredicate(goal)
			must(err)
			if status != lbool.True {
				t.Fatal("nullary inserted seed missing", status)
			}

		})
	}
}

func TestDatalogSparseIntervalNullaryIntervalFavourite(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "interval_relation"))
	must(e.SelectEngine("datalog"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("nullary-interval-mixed")
	must(err)
	mixed, err := m.MkFuncDecl(name, []ast.ASTHandle{boolean, integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(mixed))
	must(e.SetPredicateRepresentation(mixed, []string{"tr_sparse", "interval_relation"}))
	name, err = m.Symbols.Intern("nullary-interval-favourite")
	must(err)
	goal, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(goal))
	hashName, err := m.Symbols.Intern("nullary-explicit-hash-under-interval")
	must(err)
	hashGoal, err := m.MkFuncDecl(hashName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(hashGoal))
	must(e.SetPredicateRepresentation(hashGoal, []string{"tr_hashtable"}))

	must(e.AddTableFact(mixed, []uint32{1, 7}))
	status, err := e.QueryPredicate(goal)
	must(err)
	if status != lbool.False {
		t.Fatal("empty interval nullary identity", status)
	}
	must(e.AddTableFact(goal, nil))
	status, err = e.QueryPredicate(goal)
	must(err)
	if status != lbool.Undef {
		t.Fatal("interval favourite must precede exact nullary table", status)
	}
	query, err := m.MkApp(goal, nil)
	must(err)
	queryRef, err := m.NewRef(query)
	must(err)
	status, err = e.QueryExpression(query)
	must(err)
	must(m.DelRef(&queryRef))
	if status != lbool.Undef {
		t.Fatal("nullary interval expression precision", status)
	}
	must(e.AddTableFact(hashGoal, nil))
	hashQuery, err := m.MkApp(hashGoal, nil)
	must(err)
	hashRef, err := m.NewRef(hashQuery)
	must(err)
	status, err = e.QueryExpression(hashQuery)
	must(err)
	must(m.DelRef(&hashRef))
	if status != lbool.True {
		t.Fatal("interval favourite replaced exact Hash source result", status)
	}

}
