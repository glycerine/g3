package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextUpdateQuantifierAnnotationsAndGuards(t *testing.T) {
	f := newQuantifierUpdateFixture(t)
	ref, err := f.context.UpdateQuantifierAnnotations(f.value, nil, []ast.ASTHandle{f.truth}, f.false)
	updated := f.own(t, ref, err)
	if positive, err := f.manager.GetNumPatterns(updated); err != nil || positive != 0 {
		t.Fatal(positive, err)
	}
	if negative, err := f.manager.GetNumNoPatterns(updated); err != nil || negative != 1 {
		t.Fatal(negative, err)
	}
	if value, err := f.manager.GetNoPattern(updated, 0); err != nil || value != f.truth {
		t.Fatal(value, err)
	}
	// AUDIT.md ast_03-F5: ast_manager::mk_quantifier throws
	// ast_exception("simultaneous patterns and no-patterns not supported").
	if _, err := f.context.UpdateQuantifierAnnotations(f.value, []ast.ASTHandle{f.pattern}, []ast.ASTHandle{f.truth}, f.false); err == nil || err.Error() != "simultaneous patterns and no-patterns not supported" {
		t.Fatal("mixed annotations accepted", err)
	}
	if _, err := f.context.UpdateQuantifierWeight(f.truth, 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-quantifier accepted", err)
	}
}
