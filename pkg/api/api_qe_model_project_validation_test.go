package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQEModelProjectValidatesBoundsAndOwners(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "qe-owner-p")
	owner := apiQECheckedModel(t, f, p)
	sort := f.pin(f.m.MkBoolSort())
	if _, err := f.c.QEModelProject(owner.core, []ast.ASTHandle{sort}, p); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-application bound error", err)
	}
	foreign := newSMTAPIFixture(t, "QF_UF", nil)
	q := apiQEBoolVariable(t, foreign, "qe-foreign-q")
	foreignOwner := apiQECheckedModel(t, foreign, q)
	if _, err := f.c.QEModelProject(foreignOwner.core, nil, p); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign model error", err)
	}
}
