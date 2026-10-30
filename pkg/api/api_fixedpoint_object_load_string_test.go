package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointObjectLoadSMT2String(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	input := `(declare-rel loaded-p ()) (rule (! loaded-p :named loaded-fact))`
	astTestMust(t, f.fp.LoadSMT2String(input))
	declaration, err := f.fp.command.FindFunctionDeclaration("loaded-p", nil, nil, ast.ASTHandle{})
	astTestMust(t, err)
	status, err := f.fp.Query(declaration)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("loaded query", status)
	}
}
