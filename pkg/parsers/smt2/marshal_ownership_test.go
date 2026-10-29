package smt2

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2UnmarshalResultOwnsTranslatedExpression(t *testing.T) {
	target, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := target.Manager()
	if err != nil {
		t.Fatal(err)
	}
	ref, err := UnmarshalString("(assert (and true false))(check-sat)", manager)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := manager.RefGet(ref)
	if err != nil {
		t.Fatal(err)
	}
	if kind, err := manager.GetKind(handle); err != nil || kind != ast.ASTApp {
		t.Fatalf("owned kind %v, %v", kind, err)
	}
	if err = manager.DelRef(&ref); err != nil {
		t.Fatal(err)
	}
	if err = target.Close(); err != nil {
		t.Fatal(err)
	}
}
