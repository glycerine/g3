package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQEModelProjectSkolemValidatesMapAndBounds(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "skolem-validation-p")
	owner := apiQECheckedModel(t, f, p)
	if _, err := f.c.QEModelProjectSkolem(owner.core, nil, p, nil); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil map error", err)
	}
	mapping, err := f.c.NewASTMap()
	astTestMust(t, err)
	defer func() { astTestMust(t, mapping.Close()) }()
	sort := f.pin(f.m.MkBoolSort())
	if _, err = f.c.QEModelProjectSkolem(owner.core, []ast.ASTHandle{sort}, p, mapping); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-application bound error", err)
	}
}
