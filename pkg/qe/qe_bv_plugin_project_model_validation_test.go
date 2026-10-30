package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestBitVectorPluginProjectModelRejectsNilAndForeignModels(t *testing.T) {
	f := newBVModelProjectFixture(t, 8, 4)
	if projected, err := f.plugin.ProjectModel(f.contains, nil, f.formula, f.base.r.config); projected || !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("nil model accepted", projected, err)
	}
	other := newFMFixture(t)
	foreign, err := model.NewModel(other.r.manager)
	fmMust(t, err)
	defer func() { fmMust(t, foreign.Close()) }()
	if projected, err := f.plugin.ProjectModel(f.contains, foreign, f.formula, f.base.r.config); projected || err == nil {
		t.Fatal("foreign model accepted", projected, err)
	}
}
