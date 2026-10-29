package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type tableRelationFixture struct {
	t         *testing.T
	ast       *ast.ASTManager
	relations *RelationManager
	boolean   ast.ASTHandle
	bits      ast.ASTHandle
	falsehood ast.ASTHandle
	truth     ast.ASTHandle
}

func newTableRelationFixture(t *testing.T) *tableRelationFixture {
	t.Helper()
	manager := docASTManager(t)
	relations, err := NewRelationManager(manager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := relations.Close(); err != nil {
			t.Error(err)
		}
	})
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	bits, err := manager.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	falsehood, err := manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	return &tableRelationFixture{t: t, ast: manager, relations: relations, boolean: boolean, bits: bits, falsehood: falsehood, truth: truth}
}

func (f *tableRelationFixture) bv(value uint64) ast.ASTHandle {
	f.t.Helper()
	result, err := f.ast.BitVectorNumeralUint64(value, 2)
	if err != nil {
		f.t.Fatal(err)
	}
	return result
}

func closeTableRelations(t *testing.T, relations ...*TableRelation) {
	t.Helper()
	t.Cleanup(func() {
		for _, relation := range relations {
			if relation != nil {
				if err := relation.Close(); err != nil {
					t.Error(err)
				}
			}
		}
	})
}

func TestTableRelationSourceFactConversionCloneAndOwnership(t *testing.T) {
	f := newTableRelationFixture(t)
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.bits})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, relation)
	inserted, err := relation.AddFact([]ast.ASTHandle{f.truth, f.bv(2)})
	if err != nil || !inserted {
		t.Fatal("add", inserted, err)
	}
	inserted, err = relation.AddFact([]ast.ASTHandle{f.truth, f.bv(2)})
	if err != nil || inserted {
		t.Fatal("duplicate", inserted, err)
	}
	found, err := relation.ContainsFact([]ast.ASTHandle{f.truth, f.bv(2)})
	if err != nil || !found {
		t.Fatal("contains", found, err)
	}
	found, err = relation.ContainsFact([]ast.ASTHandle{f.falsehood, f.bv(2)})
	if err != nil || found {
		t.Fatal("missing", found, err)
	}
	clone, err := relation.Clone()
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, clone)
	if err = relation.Reset(); err != nil || !relation.Empty() || clone.NumRows() != 1 {
		t.Fatal("clone/reset independence", relation.Empty(), clone.NumRows(), err)
	}
	if clone.Table() == relation.Table() {
		t.Fatal("clone shared owned table")
	}
}

func TestTableRelationComplementSourceWidthContract(t *testing.T) {
	f := newTableRelationFixture(t)
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, relation)
	_, _ = relation.AddFact([]ast.ASTHandle{f.falsehood})
	complement, err := relation.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, complement)
	foundFalse, _ := complement.ContainsFact([]ast.ASTHandle{f.falsehood})
	foundTrue, _ := complement.ContainsFact([]ast.ASTHandle{f.truth})
	if foundFalse || !foundTrue {
		t.Fatal("unary complement", foundFalse, foundTrue)
	}
	wide, _ := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	closeTableRelations(t, wide)
	if _, err = wide.Complement(); !errors.Is(err, ErrTableRelation) {
		t.Fatal("wide complement", err)
	}
	emptyArity, err := NewTableRelation(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeTableRelations(t, emptyArity)
	fullArity, err := emptyArity.Complement()
	if err != nil || fullArity.NumRows() != 1 {
		t.Fatal("zero-column complement", fullArity, err)
	}
	closeTableRelations(t, fullArity)
}

func TestTableRelationPredicateConstructionValidationAndClose(t *testing.T) {
	f := newTableRelationFixture(t)
	name, err := f.ast.Symbols.Intern("table-relation-predicate")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := f.ast.MkFuncDecl(name, []ast.ASTHandle{f.boolean, f.bits}, f.boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	relation, err := NewPredicateTableRelation(f.relations, predicate)
	if err != nil {
		t.Fatal(err)
	}
	if signature, _ := relation.Signature(); signature.Len() != 2 || signature.domains[0] != 2 || signature.domains[1] != 4 {
		t.Fatal("predicate signature", signature.Domains())
	}
	wrongSignature, signatureErr := NewSignature([]uint64{3, 4})
	if signatureErr != nil {
		t.Fatal(signatureErr)
	}
	wrong := NewHashTable(wrongSignature)
	if _, err = AdoptTableRelation(f.relations, relation.sorts, wrong); !errors.Is(err, ErrTableRelation) {
		t.Fatal("adopted mismatch", err)
	}
	if err = relation.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = relation.AddTableFact(Fact{0, 0}); !errors.Is(err, ErrTableRelation) {
		t.Fatal("closed relation", err)
	}
}
