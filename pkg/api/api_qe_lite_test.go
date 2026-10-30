package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQELiteEliminatesUnusedApplication(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "qe-p")
	unused := apiQEBoolVariable(t, f, "qe-unused")
	result, remaining, err := f.c.QELite([]ast.ASTHandle{unused}, p)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&result)) }()
	if len(remaining) != 0 {
		t.Fatal("unused variable retained", remaining)
	}
	value, err := f.m.RefGet(result)
	astTestMust(t, err)
	if value != p {
		t.Fatal("unrelated body changed", value, p)
	}
}
