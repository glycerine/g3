package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestManagerFormulaNewToOldRefHasCallerScopedLifetime(t *testing.T) {
	command, manager, owner := pdrManagerFixture(t, 1)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	prefix := pdrFunction(t, manager, "manager_scoped_state", nil, boolean)
	newDeclaration, err := owner.NewPredicate(prefix)
	pdrMust(t, err)
	oldDeclaration, err := owner.OldPredicate(prefix, 0)
	pdrMust(t, err)
	newState, err := manager.MkApp(newDeclaration, nil)
	pdrMust(t, err)
	oldState, err := manager.MkApp(oldDeclaration, nil)
	pdrMust(t, err)

	before := manager.Stats().LiveReferences
	converted, err := owner.FormulaNewToOldRef(newState, 0, true)
	pdrMust(t, err)
	if got := manager.Stats().LiveReferences; got != before+1 {
		t.Fatalf("scoped conversion references = %d, want %d", got, before+1)
	}
	convertedState, err := manager.RefGet(converted)
	pdrMust(t, err)
	if convertedState != oldState {
		t.Fatal("scoped formula conversion", convertedState, oldState)
	}
	pdrMust(t, manager.DelRef(&converted))
	if converted != (ast.ASTRef{}) {
		t.Fatal("scoped formula reference was not consumed")
	}
	if got := manager.Stats().LiveReferences; got != before {
		t.Fatalf("released scoped conversion references = %d, want %d", got, before)
	}

	pdrMust(t, owner.Close())
	pdrMust(t, command.Close())
}
