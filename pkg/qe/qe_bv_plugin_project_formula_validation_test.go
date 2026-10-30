package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBitVectorPluginProjectModelRejectsInvalidFormulaRef(t *testing.T) {
	f := newBVModelProjectFixture(t, 8, 4)
	if projected, err := f.plugin.ProjectModel(f.contains, f.model, ast.ASTRef{}, f.base.r.config); projected || err == nil {
		t.Fatal("invalid formula reference accepted", projected, err)
	}
	foreign := newBVPluginFixture(t, 8)
	ref, err := foreign.manager.NewRef(foreign.variable)
	fmMust(t, err)
	defer func() { fmMust(t, foreign.manager.DelRef(&ref)) }()
	if projected, err := f.plugin.ProjectModel(f.contains, f.model, ref, f.base.r.config); projected || !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("foreign formula reference accepted", projected, err)
	}
}
