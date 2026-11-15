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

// AUDIT.md qe_03-F6: spacer_legacy_mbp.cpp qe_project(..., map) applies the
// Boolean model values with mk_expr_simp_replacer and then th_rewriter, so
// no (or true ...) is left behind.
func TestAPIQEModelProjectSkolemSimplifiesBooleanSubstitution(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "skolem-simp-p")
	q := apiQEBoolVariable(t, f, "skolem-simp-q")
	notQ := f.pin(f.m.MkNot(q))
	owner := apiQECheckedModel(t, f, f.pin(f.m.MkAnd(p, notQ)))
	mapping, err := f.c.NewASTMap()
	astTestMust(t, err)
	defer func() { astTestMust(t, mapping.Close()) }()
	// qe-lite cannot remove p from (or p q); p := true leaves (or true q).
	disjunction := f.pin(f.m.MkOr(p, q))
	result, err := f.c.QEModelProjectSkolem(owner.core, []ast.ASTHandle{p}, disjunction, mapping)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&result)) }()
	formula, err := f.m.RefGet(result)
	astTestMust(t, err)
	if !f.m.IsTrue(formula) {
		t.Fatal("Boolean substitution must be simplified to true", formula)
	}
}
