package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQEModelProjectSkolemResultOutlivesModelReference(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "skolem-owner-p")
	owner := apiQECheckedModel(t, f, p)
	mapping, err := f.c.NewASTMap()
	astTestMust(t, err)
	result, err := f.c.QEModelProjectSkolem(owner.core, []ast.ASTHandle{p}, p, mapping)
	astTestMust(t, err)
	astTestMust(t, owner.core.DecRef())
	owner.core = nil
	formula, err := f.m.RefGet(result)
	astTestMust(t, err)
	if !f.m.IsTrue(formula) {
		t.Fatal("owned projected result", formula)
	}
	astTestMust(t, mapping.Close())
	astTestMust(t, f.m.DelRef(&result))
}
