package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQELiteRejectsNonApplicationVariable(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "qe-validation-p")
	sort := f.pin(f.m.MkBoolSort())
	if _, _, err := f.c.QELite([]ast.ASTHandle{sort}, p); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-app variable error", err)
	}
	if _, _, err := f.c.QELite(nil, ast.ASTHandle{}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("null body error", err)
	}
}
