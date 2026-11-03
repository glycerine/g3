package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationManagerCheckedFactoryPrecedesLazyDefaultForTemporaries(t *testing.T) {
	f := newTableRelationFixture(t)
	lazy := NewLazySparseFiniteFactory()
	t.Cleanup(func() { _ = lazy.Close() })
	if err := f.relations.SetLazySparseFiniteFactory(lazy); err != nil {
		t.Fatal(err)
	}
	if err := f.relations.SetDefaultFiniteTable("lazy_sparse"); err != nil {
		t.Fatal(err)
	}
	checked, err := NewFiniteCheckedFactory("sparse", "hashtable")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.SetCheckedFiniteFactory(checked); err != nil {
		t.Fatal(err)
	}
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	table, err := f.relations.NewDefaultFiniteTable(signature)
	if err != nil {
		t.Fatal(err)
	}
	pair, ok := table.(*FiniteCheckedPair)
	if !ok {
		t.Fatalf("checked temporary type %T", table)
	}
	primary, checker, err := pair.ConcreteKinds()
	if err != nil || primary != "sparse" || checker != "hashtable" {
		t.Fatal("checked temporary kinds", primary, checker, err)
	}
	if err = pair.Close(); err != nil {
		t.Fatal(err)
	}
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	table, err = f.relations.NewDefaultFiniteTable(nullary)
	if err != nil {
		t.Fatal(err)
	}
	if _, checked := table.(*FiniteCheckedPair); checked {
		t.Fatal("source nullary fallback remained checked")
	}
	if _, hash := table.(*HashTable); !hash {
		t.Fatalf("nullary fallback type %T, want hash", table)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	total := NewTotalInstruction(signature, 0)
	t.Cleanup(func() { _ = total.Close() })
	complete, err := total.Perform(context)
	if err != nil || !complete {
		t.Fatal("checked compiler temporary", complete, err)
	}
	if temporary, err := context.FiniteRegister(0); err != nil {
		t.Fatal(err)
	} else if _, ok := temporary.(*FiniteCheckedPair); !ok {
		t.Fatalf("compiler temporary type %T", temporary)
	}
	if err = f.relations.SetCheckedFiniteFactory(nil); err != nil {
		t.Fatal(err)
	}
	table, err = f.relations.NewDefaultFiniteTable(signature)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := table.(*LazySparseTable); !ok || !lazy.Owns(table) {
		t.Fatalf("clearing checked scope did not restore lazy default: %T", table)
	}
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRelationManagerExchangeFiniteRelationRoundTripAndFailures(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("checked-exchange")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := f.relations.GetFiniteRelation(predicate)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := original.Signature()
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewFiniteCheckedFactory("sparse", "hashtable")
	if err != nil {
		t.Fatal(err)
	}
	first, err := factory.New(signature)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	prior, err := f.relations.ExchangeFiniteRelation(predicate, first)
	if err != nil || prior != original {
		t.Fatal("install checked owner", prior, original, err)
	}
	second, err := factory.New(signature)
	if err != nil {
		t.Fatal(err)
	}
	retired, err := f.relations.ExchangeFiniteRelation(predicate, second)
	if err != nil || retired != first {
		t.Fatal("replace checked owner", retired, first, err)
	}
	current, err := f.relations.ExchangeFiniteRelation(predicate, original)
	if err != nil || current != second {
		t.Fatal("restore original owner", current, second, err)
	}
	if err = retired.Close(); err != nil {
		t.Fatal(err)
	}
	if err = current.Close(); err != nil {
		t.Fatal(err)
	}
	stored, found, err := f.relations.TryGetFiniteRelation(predicate)
	if err != nil || !found || stored != original {
		t.Fatal("round trip manager owner", stored, found, err)
	}
	wrongSignature, err := NewSignature([]uint64{3})
	if err != nil {
		t.Fatal(err)
	}
	wrong := NewHashTable(wrongSignature)
	if prior, err = f.relations.ExchangeFiniteRelation(predicate, wrong); err == nil || prior != nil {
		t.Fatal("mismatched exchange accepted", prior, err)
	}
	if _, err = wrong.Len(); err != nil {
		t.Fatal("failed exchange consumed replacement", err)
	}
	if err = wrong.Close(); err != nil {
		t.Fatal(err)
	}
	missing := defaultFinitePredicate(t, f, "checked-exchange-missing", []ast.ASTHandle{f.boolean})
	replacement, err := factory.New(signature)
	if err != nil {
		t.Fatal(err)
	}
	if prior, err = f.relations.ExchangeFiniteRelation(missing, replacement); err == nil || prior != nil {
		t.Fatal("missing exchange accepted", prior, err)
	}
	if _, err = replacement.Len(); err != nil {
		t.Fatal("missing exchange consumed replacement", err)
	}
	if err = replacement.Close(); err != nil {
		t.Fatal(err)
	}
}
