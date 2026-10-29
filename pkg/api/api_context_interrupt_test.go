package api

import (
	"errors"
	"testing"
)

func TestSMTContextInterruptSetsActualResourceLimit(t *testing.T) {
	context, err := NewSMTContext()
	astTestMust(t, err)
	manager, err := context.Manager()
	astTestMust(t, err)
	if manager.ResourceLimit().CancelFlag() {
		t.Fatal("new context starts canceled")
	}
	astTestMust(t, context.Interrupt())
	if !manager.ResourceLimit().CancelFlag() {
		t.Fatal("interrupt did not reach AST resource limit")
	}
	manager.ResourceLimit().ResetCancel()
	astTestMust(t, context.Close())
	if err = context.Interrupt(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed interrupt", err)
	}
}
