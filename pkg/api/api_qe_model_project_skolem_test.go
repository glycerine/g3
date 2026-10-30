package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIQEModelProjectSkolemSubstitutesBooleanWithoutPublishingMap(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "skolem-bool-p")
	owner := apiQECheckedModel(t, f, p)
	mapping, err := f.c.NewASTMap()
	astTestMust(t, err)
	defer func() { astTestMust(t, mapping.Close()) }()
	result, err := f.c.QEModelProjectSkolem(owner.core, []ast.ASTHandle{p}, p, mapping)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&result)) }()
	formula, err := f.m.RefGet(result)
	astTestMust(t, err)
	if !f.m.IsTrue(formula) {
		t.Fatal("Boolean model substitution", formula)
	}
	size, err := mapping.object.size()
	astTestMust(t, err)
	if size != 0 {
		t.Fatal("source does not publish Boolean map entries", size)
	}
}
