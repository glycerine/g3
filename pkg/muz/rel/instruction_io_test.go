package rel_test

// Source: src/muz/rel/dl_instruction.cpp instr_io load/store operations.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestRelationInstructionsLoadCloneAndStoreOwnership(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Close() })
	manager, err := command.Manager()
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
	name, err := manager.Symbols.Intern("instruction-relation")
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := relations.GetRelation(predicate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stored.Add(rel.Fact{1}); err != nil {
		t.Fatal(err)
	}
	context, err := rel.NewExecutionContextWithRelations(relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	load, err := rel.NewLoadInstruction(relations, predicate, 0)
	if err != nil {
		t.Fatal(err)
	}
	store, err := rel.NewStoreInstruction(relations, predicate, 0)
	if err != nil {
		t.Fatal(err)
	}
	block := rel.NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	if err = block.Append(load); err != nil {
		t.Fatal(err)
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("load", complete, err)
	}
	loaded, err := context.Register(0)
	if err != nil || loaded == nil || loaded == stored {
		t.Fatal("load did not clone", loaded, stored, err)
	}
	if _, err = loaded.Add(rel.Fact{0}); err != nil {
		t.Fatal(err)
	}
	if count, x := stored.Len(); x != nil || count != 1 {
		t.Fatal("load mutated stored relation", count, x)
	}
	if err = block.Append(store); err != nil {
		t.Fatal(err)
	}
	complete, err = store.Perform(context)
	if err != nil || !complete {
		t.Fatal("store", complete, err)
	}
	if value, x := context.Register(0); x != nil || value != nil {
		t.Fatal("store did not release register", value, x)
	}
	stored, found, err := relations.TryGetRelation(predicate)
	if err != nil || !found || stored != loaded {
		t.Fatal("store did not transfer table", stored, loaded, found, err)
	}
	complete, err = store.Perform(context)
	if err != nil || !complete {
		t.Fatal("empty store", complete, err)
	}
	stored, found, err = relations.TryGetRelation(predicate)
	if err != nil || !found || stored == nil {
		t.Fatal("empty relation was not stored", stored, found, err)
	}
	if count, x := stored.Len(); x != nil || count != 0 {
		t.Fatal("empty store retained facts", count, x)
	}
	if err = block.MakeAnnotations(context); err != nil {
		t.Fatal(err)
	}
	annotation, found, err := context.RegisterAnnotation(0)
	if err != nil || !found || annotation != "instruction-relation" {
		t.Fatal("relation annotation", annotation, found, err)
	}
	var display bytes.Buffer
	if err = load.DisplayHead(&display); err != nil || display.String() != "load instruction-relation into 0" {
		t.Fatal("load display", display.String(), err)
	}

	wrongSignature, err := rel.NewSignature([]uint64{8})
	if err != nil {
		t.Fatal("wrong signature", err)
	}
	wrongTable := rel.NewHashTable(wrongSignature)
	if _, err = wrongTable.Add(rel.Fact{1}); err != nil {
		t.Fatal("wrong table fact", err)
	}
	if err = context.SetRegister(0, wrongTable); err != nil {
		t.Fatal("set invalid store table", err)
	}
	if complete, err = store.Perform(context); err == nil || complete {
		t.Fatal("store accepted a table with the wrong signature", complete, err)
	}
	retained, err := context.Register(0)
	if err != nil || retained != wrongTable {
		t.Fatal("failed store did not restore register ownership", retained, err)
	}
	if count, err := retained.Len(); err != nil || count != 1 {
		t.Fatal("failed store retired its input", count, err)
	}
}
