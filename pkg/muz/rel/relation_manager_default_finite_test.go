package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func defaultFinitePredicate(t *testing.T, f *tableRelationFixture, name string, domains []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	symbol, err := f.ast.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(symbol, domains, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	return predicate
}

func TestRelationManagerDefaultFiniteSelectionAndExplicitPrecedence(t *testing.T) {
	f := newTableRelationFixture(t)
	explicit := defaultFinitePredicate(t, f, "default-explicit-hash", []ast.ASTHandle{f.boolean})
	explicitTable, err := f.relations.GetRelation(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.relations.SetDefaultFiniteTable("sparse"); err != nil {
		t.Fatal(err)
	}
	borrowed, err := f.relations.GetFiniteRelation(explicit)
	if err != nil || borrowed != explicitTable {
		t.Fatal("default replaced explicit existing owner", borrowed, explicitTable, err)
	}
	sparsePredicate := defaultFinitePredicate(t, f, "default-sparse", []ast.ASTHandle{f.boolean})
	sparse, err := f.relations.GetFiniteRelation(sparsePredicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sparse.(*SparseTable); !ok {
		t.Fatalf("non-nullary default type %T, want *SparseTable", sparse)
	}
	nullaryPredicate := defaultFinitePredicate(t, f, "default-nullary", nil)
	nullary, err := f.relations.GetFiniteRelation(nullaryPredicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := nullary.(*HashTable); !ok {
		t.Fatalf("nullary fallback type %T, want *HashTable", nullary)
	}
	if err = f.relations.SetDefaultFiniteTable("interval"); err == nil {
		t.Fatal("unavailable default finite table accepted")
	}
}

func TestConstructInstructionsUseContextDefaultFiniteTable(t *testing.T) {
	f := newTableRelationFixture(t)
	if err := f.relations.SetDefaultFiniteTable("sparse"); err != nil {
		t.Fatal(err)
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	nullary, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	singleton, err := NewUnarySingletonInstruction(2, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	total := NewTotalInstruction(signature, 1)
	nullaryTotal := NewTotalInstruction(nullary, 2)
	for _, instruction := range []Instruction{singleton, total, nullaryTotal} {
		t.Cleanup(func() { _ = instruction.Close() })
		complete, err := instruction.Perform(context)
		if err != nil || !complete {
			t.Fatal("construct default finite", complete, err)
		}
	}
	for _, register := range []uint32{0, 1} {
		table, err := context.FiniteRegister(register)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := table.(*SparseTable); !ok {
			t.Fatalf("register %d type %T, want *SparseTable", register, table)
		}
	}
	table, err := context.FiniteRegister(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := table.(*HashTable); !ok {
		t.Fatalf("nullary total type %T, want *HashTable", table)
	}
}
