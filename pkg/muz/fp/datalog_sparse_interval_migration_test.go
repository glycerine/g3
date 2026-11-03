package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSparseIntervalFiniteSeedMigrationCloneAndPublication(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	boolean, err := m.MkBoolSort()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	declare := func(name string, domain ast.ASTHandle) ast.ASTHandle {
		symbol, cause := m.Symbols.Intern(name)
		must(cause)
		declaration, cause := m.MkFuncDecl(symbol, []ast.ASTHandle{domain}, boolean, ast.ASTFuncDeclInfo{})
		must(cause)
		must(e.RegisterPredicate(declaration))
		return declaration
	}
	finite := declare("sparse-interval-migrated-finite", boolean)
	arithmetic := declare("sparse-interval-migrated-arithmetic", integer)
	nullaryName, err := m.Symbols.Intern("sparse-interval-migrated-nullary")
	must(err)
	nullary, err := m.MkFuncDecl(nullaryName, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(nullary))
	truth, err := m.MkTrue()
	must(err)
	falsehood, err := m.MkFalse()
	must(err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	must(err)

	state := e.predicates[finite]
	prior := state.table
	seed, err := e.relations.EncodeFact(state.sorts, []ast.ASTHandle{truth})
	must(err)
	_, err = state.table.Add(seed)
	must(err)
	_, err = e.predicates[nullary].table.Add(rel.Fact{})
	must(err)
	originals := []sparseIntervalOriginal{
		{declaration: finite, class: sparseIntervalFiniteOriginal},
		{declaration: nullary, class: sparseIntervalHashOriginal},
		{declaration: arithmetic, class: sparseIntervalArithmeticOriginal},
	}
	activation, selected, err := newClassifiedSparseIntervalDatalogActivation(m, e.set, e.guard, originals)
	must(err)
	if !selected {
		t.Fatal("heterogeneous activation not selected")
	}
	e.sparseIntervalDatalog = activation
	declarations := []ast.ASTHandle{finite, nullary, arithmetic}
	must(e.migrateSparseIntervalFiniteSeeds(activation, declarations))
	intervalSource, intervalSelected, err := newIntervalDatalogActivation(m, e.set, []ast.ASTHandle{arithmetic})
	must(err)
	if !intervalSelected {
		t.Fatal("interval seed owner not selected")
	}
	defer func() { must(intervalSource.close()) }()
	must(intervalSource.addFact(arithmetic, []ast.ASTHandle{seven}))
	must(e.migrateSparseIntervalArithmeticSeeds(activation, intervalSource, []ast.ASTHandle{finite, arithmetic}))
	foundArithmetic, err := activation.store.entries[arithmetic].relation.ContainsFact([]ast.ASTHandle{seven})
	must(err)
	if !foundArithmetic {
		t.Fatal("interval seed was not transferred without enumeration")
	}
	foundNullary, err := activation.store.entries[nullary].relation.ContainsFact(nil)
	must(err)
	if !foundNullary {
		t.Fatal("nullary Hash seed was not migrated")
	}
	if e.predicates[finite].table != prior {
		t.Fatal("migration replaced public finite owner")
	}
	rows, err := prior.Facts()
	must(err)
	if len(rows) != 1 {
		t.Fatal("migration mutated public finite rows", rows)
	}
	found, err := activation.store.entries[finite].relation.ContainsFact([]ast.ASTHandle{truth})
	must(err)
	if !found {
		t.Fatal("pre-activation finite fact was not migrated")
	}
	late, err := e.relations.EncodeFact(state.sorts, []ast.ASTHandle{falsehood})
	must(err)
	_, err = prior.Add(late)
	must(err)
	must(e.migrateSparseIntervalFiniteSeeds(activation, declarations))
	found, err = activation.store.entries[finite].relation.ContainsFact([]ast.ASTHandle{falsehood})
	must(err)
	if found {
		t.Fatal("completed migration replayed later public rows as seeds")
	}
	must(e.addSparseIntervalFact(finite, []ast.ASTHandle{falsehood}))
	found, err = activation.store.entries[finite].relation.ContainsFact([]ast.ASTHandle{falsehood})
	must(err)
	if !found {
		t.Fatal("atomic direct fact did not update product seed")
	}
	afterDirect := e.predicates[finite].table
	if afterDirect == prior {
		t.Fatal("atomic direct fact did not replace public snapshot")
	}

	clone, err := e.newQueryClone()
	must(err)
	defer func() { must(clone.Close()) }()
	if clone.sparseIntervalDatalog == nil || clone.sparseIntervalDatalog == activation {
		t.Fatal("query clone did not own an independent product seed")
	}
	must(clone.sparseIntervalDatalog.addFact(finite, []ast.ASTHandle{falsehood}))
	if status, cause := clone.sparseIntervalDatalog.query([]ast.ASTHandle{finite}, false); cause != nil || status != lbool.True {
		t.Fatal("finite product query status", status, cause)
	}
	must(e.publishSparseIntervalFiniteRelations(clone))
	if e.predicates[finite].table == afterDirect {
		t.Fatal("successful publication kept stale public owner")
	}
	for _, value := range []ast.ASTHandle{truth, falsehood} {
		fact, cause := e.relations.EncodeFact(state.sorts, []ast.ASTHandle{value})
		must(cause)
		contains, cause := e.predicates[finite].table.Contains(fact)
		must(cause)
		if !contains {
			t.Fatal("published finite result lost source row", value)
		}
	}
	if e.predicates[arithmetic].table != nil {
		t.Fatal("arithmetic result became an enumerable finite table")
	}
	if _, exact, cause := clone.sparseIntervalDatalog.finiteSnapshot(arithmetic); cause != nil || exact {
		t.Fatal("arithmetic snapshot claimed exact finite ownership", exact, cause)
	}
	if _, ok := e.predicates[finite].table.(*rel.SparseTable); !ok {
		t.Fatal("finite publication did not install sparse snapshot")
	}
	if _, ok := e.predicates[nullary].table.(*rel.HashTable); !ok {
		t.Fatal("nullary publication did not preserve Hash snapshot kind")
	}
	if contains, cause := e.predicates[nullary].table.Contains(rel.Fact{}); cause != nil || !contains {
		t.Fatal("nullary Hash publication lost unit row", contains, cause)
	}
}
