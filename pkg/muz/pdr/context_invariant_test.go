package pdr

// Source: pdr_context.cpp context::check_invariant.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestContextCheckInvariantAcceptsValidAndRejectsInvalidFrame(t *testing.T) {
	manager, owner, transformer, _, closeBase := closureFixture(t)
	truth, err := manager.MkTrue()
	closureMust(t, err)
	closureMust(t, transformer.InitializeSystem(truth, truth))
	transformer.rulesReady = true
	head, err := transformer.Head()
	closureMust(t, err)
	context := &Context{
		manager: manager, owner: owner,
		transformers: map[ast.ASTHandle]*PredicateTransformer{head: transformer}, order: []ast.ASTHandle{head},
	}
	valid, err := context.CheckPredicateInvariant(0, head)
	closureMust(t, err)
	if !valid {
		t.Fatal("empty next frame is not invariant")
	}
	valid, err = context.CheckInvariants(0)
	closureMust(t, err)
	if !valid {
		t.Fatal("valid context invariant rejected")
	}
	falsity, err := manager.MkFalse()
	closureMust(t, err)
	changed, err := transformer.AddProperty(falsity, 1)
	closureMust(t, err)
	if !changed {
		t.Fatal("invalid next-frame property was not installed")
	}
	valid, err = context.CheckPredicateInvariant(0, head)
	closureMust(t, err)
	if valid {
		t.Fatal("transition premises imply false next frame")
	}
	valid, err = context.CheckInvariants(0)
	closureMust(t, err)
	if valid {
		t.Fatal("invalid context invariant accepted")
	}
	closureMust(t, transformer.Close())
	closureMust(t, owner.Close())
	closeBase()
}
