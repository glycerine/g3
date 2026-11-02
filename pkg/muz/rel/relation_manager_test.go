package rel_test

// Source: src/muz/rel/dl_relation_manager.cpp finite signature and fact
// conversion.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestRelationManagerFiniteSignatureAndFactRoundTrip(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := relations.Close(); err != nil {
			t.Fatal(err)
		}
	})
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	bitVector, err := manager.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	finiteName, err := manager.Symbols.Intern("rel-finite-three")
	if err != nil {
		t.Fatal(err)
	}
	finite, err := manager.MkDatalogFiniteSort(finiteName, 3)
	if err != nil {
		t.Fatal(err)
	}
	predicateName, err := manager.Symbols.Intern("rel-round-trip")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(predicateName, []ast.ASTHandle{boolean, bitVector, finite}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	sorts, signature, err := relations.PredicateSignature(predicate)
	if err != nil {
		t.Fatal(err)
	}
	if got := signature.Domains(); len(got) != 3 || got[0] != 2 || got[1] != 4 || got[2] != 3 {
		t.Fatal("signature", got)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	bv, err := manager.BitVectorNumeralUint64(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	fd, err := manager.MkDatalogNumeral(1, finite)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := relations.EncodeFact(sorts, []ast.ASTHandle{truth, bv, fd})
	if err != nil {
		t.Fatal(err)
	}
	if len(fact) != 3 || fact[0] != 1 || fact[1] != 2 || fact[2] != 1 {
		t.Fatal("encoded fact", fact)
	}
	references, err := relations.DecodeFact(sorts, fact)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for i := len(references) - 1; i >= 0; i-- {
			if err := manager.DelRef(&references[i]); err != nil {
				t.Fatal(err)
			}
		}
	}()
	decoded := make([]ast.ASTHandle, len(references))
	for i := range references {
		decoded[i], err = manager.RefGet(references[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	roundTrip, err := relations.EncodeFact(sorts, decoded)
	if err != nil {
		t.Fatal(err)
	}
	for i := range fact {
		if roundTrip[i] != fact[i] {
			t.Fatal("round trip", fact, roundTrip)
		}
	}
}

func TestRelationManagerRejectsInfiniteAndWrongSortValues(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	integer, err := context.LookupSort("Int")
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = relations.EncodeValue(integer, truth); err == nil {
		t.Fatal("infinite relation sort accepted")
	}
	bitVector, err := manager.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = relations.EncodeValue(bitVector, truth); err == nil {
		t.Fatal("wrong-sort relation value accepted")
	}
	if _, err = relations.DecodeValue(boolean, 2); err == nil {
		t.Fatal("out-of-domain table value accepted")
	}
}

func TestRelationManagerOwnsStoredPredicateRelations(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	relations, err := rel.NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relations.Close() })
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("stored-relation")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	table, err := relations.GetRelation(predicate)
	if err != nil {
		t.Fatal(err)
	}
	if count, x := table.Len(); x != nil || count != 0 {
		t.Fatal("new relation", count, x)
	}
	if _, err = table.Add(rel.Fact{1}); err != nil {
		t.Fatal(err)
	}
	stored, found, err := relations.TryGetRelation(predicate)
	if err != nil || !found || stored != table {
		t.Fatal("stored relation identity", stored, found, err)
	}
	replacement, err := table.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err = relations.StoreRelation(predicate, replacement); err != nil {
		t.Fatal(err)
	}
	if _, err := table.Len(); err == nil {
		t.Fatal("replaced manager table remained live")
	}
	stored, found, err = relations.TryGetRelation(predicate)
	if err != nil || !found || stored != replacement {
		t.Fatal("replacement relation identity", stored, found, err)
	}
	wrongSignature, err := rel.NewSignature([]uint64{3})
	if err != nil {
		t.Fatal(err)
	}
	if err = relations.StoreRelation(predicate, rel.NewHashTable(wrongSignature)); err == nil {
		t.Fatal("mismatched stored relation accepted")
	}
	if err = relations.ResetRelations(); err != nil {
		t.Fatal(err)
	}
	if stored, found, err = relations.TryGetRelation(predicate); err != nil || found || stored != nil {
		t.Fatal("reset relation remained stored", stored, found, err)
	}
	if _, err := replacement.Len(); err == nil {
		t.Fatal("reset manager table remained live")
	}
}
