package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextUpdateQuantifierPatternsReplacesAnnotations(t *testing.T) {
	f := newQuantifierUpdateFixture(t)
	secondTerm, err := f.manager.MkNot(f.false)
	secondTerm = f.pin(t, secondTerm, err)
	second, err := f.manager.MkPattern([]ast.ASTHandle{secondTerm})
	second = f.pin(t, second, err)
	ref, err := f.context.UpdateQuantifierPatterns(f.value, []ast.ASTHandle{second}, f.false)
	updated := f.own(t, ref, err)
	if count, err := f.manager.GetNumPatterns(updated); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if pattern, err := f.manager.GetPattern(updated, 0); err != nil || pattern != second {
		t.Fatal(pattern, err)
	}
	if count, err := f.manager.GetNumNoPatterns(updated); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
