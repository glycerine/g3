package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestArithmeticCompleteProjectGuards(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "0")
	truth := f.own(f.r.manager.MkTrue())
	r, e := f.r.manager.NewRef(truth)
	fmMust(t, e)
	defer f.r.manager.DelRef(&r)
	if e = f.plugin.ValidateCompleteProjection(nil, f.model, r); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
	if e = f.plugin.ValidateCompleteProjection(f.contains, nil, r); !errors.Is(e, ast.ErrASTHandle) {
		t.Fatal(e)
	}
}
