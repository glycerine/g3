package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticCompleteManyOwnershipGuards(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	r, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&r)
	if _, e = f.plugin.ProjectVariablesNNFComplete(nil, nil, r, f.r.config); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
	result, e := f.plugin.ProjectVariablesNNFComplete(nil, f.model, r, f.r.config)
	fmMust(t, e)
	copy := *result
	if e = copy.Close(); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
	fmMust(t, result.Close())
	if _, e = result.Len(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}
