package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExecutionBytecodeMixedSparseHashAndNullaryProjection(t *testing.T) {
	signature, err := NewSignature([]uint64{4})
	if err != nil {
		t.Fatal(err)
	}
	sparse, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{1}, {2}} {
		if _, err = sparse.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	hash := NewHashTable(signature)
	if _, err = hash.Add(Fact{2}); err != nil {
		t.Fatal(err)
	}
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(0, sparse); err != nil {
		t.Fatal(err)
	}
	if err = context.SetRegister(1, hash); err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	for _, instruction := range []Instruction{
		NewJoinInstruction(0, 1, []int{0}, []int{0}, 2),
		NewProjectionInstruction(2, []int{0, 1}, 3),
		NewCloneInstruction(2, 4),
	} {
		if err = block.Append(instruction); err != nil {
			t.Fatal(err)
		}
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("mixed finite bytecode", complete, err)
	}
	joined, err := context.FiniteRegister(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := joined.(*SparseTable); !ok {
		t.Fatalf("mixed join result type %T, want *SparseTable", joined)
	}
	facts, err := joined.Facts()
	if err != nil || len(facts) != 1 || len(facts[0]) != 2 || facts[0][0] != 2 || facts[0][1] != 2 {
		t.Fatal("mixed join facts", facts, err)
	}
	if _, err = context.Register(2); err == nil {
		t.Fatal("legacy Hash register accessor accepted sparse owner")
	}
	clone, err := context.FiniteRegister(4)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := clone.(*SparseTable); !ok || clone == joined {
		t.Fatalf("sparse clone ownership %T %v", clone, clone == joined)
	}
	nullary, err := context.Register(3)
	if err != nil {
		t.Fatal(err)
	}
	if nullary == nil {
		t.Fatal("project-all lost true nullary identity")
	}
	nullarySignature, err := nullary.Signature()
	if err != nil || nullarySignature.Len() != 0 {
		t.Fatal("project-all signature", nullarySignature.Len(), err)
	}
	contains, err := nullary.Contains(Fact{})
	if err != nil || !contains {
		t.Fatal("project-all nullary fact", contains, err)
	}
}

func TestMixedUnionPreservesLoadedEmptyHashRepresentation(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("empty-hash-mixed-union")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := f.relations.GetRelation(predicate)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := selected.Signature()
	if err != nil {
		t.Fatal(err)
	}
	source, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = source.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(1, source); err != nil {
		t.Fatal(err)
	}
	load, err := NewLoadInstruction(f.relations, predicate, 0)
	if err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	if err = block.Append(load); err != nil {
		t.Fatal(err)
	}
	if err = block.Append(NewUnionInstruction(1, 0, VoidRegister)); err != nil {
		t.Fatal(err)
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("load and mixed union", complete, err)
	}
	target, err := context.Register(0)
	if err != nil || target == nil {
		t.Fatal("mixed union replaced selected Hash owner", target, err)
	}
	found, err := target.Contains(Fact{1})
	if err != nil || !found {
		t.Fatal("mixed union lost sparse source row", found, err)
	}
}
