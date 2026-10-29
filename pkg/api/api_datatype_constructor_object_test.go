package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIDatatypeConstructorObjectOwnsSortsAndLifecycle(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	object, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{
		Name: "api-box", Recognizer: "is-api-box",
		Accessors: []APIDatatypeAccessor{{Name: "api-box-value", Sort: boolean}},
	})
	astTestMust(t, err)
	if len(object.sortRefs) != 1 || object.sortRefs[0] == (ast.ASTRef{}) {
		t.Fatal("constructor did not retain accessor sort")
	}
	if _, err = object.Query(); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unbuilt constructor query", err)
	}
	astTestMust(t, object.Close())
	astTestMust(t, object.Close())
	if _, err = object.Query(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed constructor", err)
	}
}
