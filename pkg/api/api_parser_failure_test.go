package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIParserFailureIsolatedAndForeignBindingsRejected(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	if _, err := left.c.ParseSMT2String("(assert", SMT2Bindings{}); err == nil {
		t.Fatal("malformed parser input accepted")
	}
	assertions, err := left.c.Assertions()
	astTestMust(t, err)
	if len(assertions) != 0 {
		t.Fatal("failed parser leaked assertion", assertions)
	}
	foreignSort := right.pin(right.c.BoolSort())
	if _, err = left.c.ParseSMT2String("", SMT2Bindings{Sorts: []SMT2SortBinding{{Name: "B", Sort: foreignSort}}}); err == nil {
		t.Fatal("foreign sort binding accepted")
	}
	localSort := left.pin(left.c.BoolSort())
	if _, err = left.c.ParseSMT2String("", SMT2Bindings{Declarations: []SMT2DeclarationBinding{{Name: "bad", Declaration: localSort}}}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-declaration binding", err)
	}
}
