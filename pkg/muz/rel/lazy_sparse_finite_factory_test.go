package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazySparseFiniteFactoryMigratesAndOwnsConcreteTables(t *testing.T) {
	signature, err := NewSignature([]uint64{4, 8})
	if err != nil {
		t.Fatal(err)
	}
	source := NewHashTable(signature)
	t.Cleanup(func() { _ = source.Close() })
	for _, fact := range []Fact{{1, 2}, {3, 4}} {
		if _, err = source.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = factory.Close() })
	lazy, err := factory.Migrate(source)
	if err != nil {
		t.Fatal(err)
	}
	if !factory.Owns(lazy) || factory.Owns(source) {
		t.Fatal("factory ownership classification")
	}
	facts, err := lazy.Facts()
	if err != nil || len(facts) != 2 || facts[0][0] != 1 || facts[1][0] != 3 {
		t.Fatal("migrated facts", facts, err)
	}
	if err = lazy.Reset(); err != nil {
		t.Fatal(err)
	}
	count, err := source.Len()
	if err != nil || count != 2 {
		t.Fatal("migration aliased source", count, err)
	}
	if err = lazy.Close(); err != nil {
		t.Fatal(err)
	}
	if factory.Owns(lazy) {
		t.Fatal("factory claimed closed handle")
	}
}

func TestLazySparseFiniteFactoryCloseRetiresOutstandingHandles(t *testing.T) {
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	factory := NewLazySparseFiniteFactory()
	table, err := factory.New(signature)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = table.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	if err = factory.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Len(); err == nil {
		t.Fatal("factory close left table live")
	}
	if _, err = factory.New(signature); err == nil {
		t.Fatal("closed factory created a table")
	}
	if err = factory.Close(); err != nil {
		t.Fatal("idempotent close", err)
	}
}

func TestLazySparseFiniteFactoryRejectsNullaryMigration(t *testing.T) {
	signature, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	source := NewHashTable(signature)
	t.Cleanup(func() { _ = source.Close() })
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = factory.Close() })
	if table, err := factory.Migrate(source); err == nil || table != nil {
		t.Fatal("nullary lazy sparse migration accepted", table, err)
	}
}

func TestLazySparseFiniteFactoryPublishesThroughRelationManager(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("lazy-sparse-manager-selection")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := f.relations.GetFiniteRelation(predicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	factory := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = factory.Close() })
	lazy, err := factory.Migrate(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.StoreFiniteRelation(predicate, lazy); err != nil {
		t.Fatal(err)
	}
	stored, found, err := f.relations.TryGetFiniteRelation(predicate)
	if err != nil || !found || stored != lazy || !factory.Owns(stored) {
		t.Fatal("manager did not publish concrete lazy owner", stored, found, err)
	}
	contains, err := stored.Contains(Fact{1})
	if err != nil || !contains {
		t.Fatal("manager publication lost migrated row", contains, err)
	}
	if err = f.relations.ResetRelations(); err != nil {
		t.Fatal(err)
	}
	if factory.Owns(lazy) {
		t.Fatal("manager reset left lazy handle live")
	}
}
