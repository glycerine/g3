package tactic

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
)

func TestSATModelConverterAssertionActionsAreOwnerScoped(t *testing.T) {
	err := satTacticModelConverterAssertion(false, false, 590, "is_uninterp_const(atom)")
	if ast.ASTSourceDebug {
		if !errors.Is(err, sat.ErrSATAssertion) {
			t.Fatalf("debug assertion: %v", err)
		}
	} else if err != nil {
		t.Fatalf("release assertion: %v", err)
	}
	if err = satTacticModelConverterAssertion(true, false, 591, "m().is_bool(atom)"); err != nil {
		t.Fatalf("true assertion: %v", err)
	}
}
