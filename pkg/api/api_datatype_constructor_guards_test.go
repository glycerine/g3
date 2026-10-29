package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIDatatypeConstructorOwnershipAndRecursiveGuards(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	foreign, err := right.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-foreign-constructor", Recognizer: "is-api-foreign-constructor"})
	astTestMust(t, err)
	if _, err = left.c.NewDatatypeConstructorList(foreign); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign constructor", err)
	}
	bad, err := left.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-bad-recursive", Recognizer: "is-api-bad-recursive", Accessors: []APIDatatypeAccessor{{Name: "api-bad-field", Recursive: true, RecursiveIndex: 1}}})
	astTestMust(t, err)
	if _, err = left.c.DatatypeSortFromConstructors("api-bad-recursive-sort", bad); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("recursive index", err)
	}
	boolean := left.pin(left.c.BoolSort())
	if _, err = left.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-malformed", Recognizer: "is-api-malformed", Accessors: []APIDatatypeAccessor{{Name: "api-malformed-field", Recursive: true, Sort: boolean}}}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("recursive accessor with concrete sort", err)
	}
	astTestMust(t, bad.Close())
	astTestMust(t, foreign.Close())
}
