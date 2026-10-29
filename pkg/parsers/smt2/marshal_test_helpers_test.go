package smt2

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

func marshalTestFormula(t *testing.T, script string) (*cmd_context.Context, *ast.ASTManager, ast.ASTHandle) {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = ParseCommands(context, strings.NewReader(script), io.Discard); err != nil {
		t.Fatal(err)
	}
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	assertions, err := context.Assertions()
	if err != nil || len(assertions) != 1 {
		t.Fatalf("assertions %v, %v", assertions, err)
	}
	return context, manager, assertions[0]
}

func marshalTestRoundTrip(t *testing.T, manager *ast.ASTManager, formula ast.ASTHandle) (string, ast.ASTRef, ast.ASTHandle) {
	t.Helper()
	text, err := MarshalString(formula, manager)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := UnmarshalString(text, manager)
	if err != nil {
		t.Fatal(err, text)
	}
	handle, err := manager.RefGet(ref)
	if err != nil {
		t.Fatal(err)
	}
	return text, ref, handle
}
