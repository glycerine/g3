package rel

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: dl_compiler.cpp get_local_indexes_for_projection counts head
// occurrences, reserves one occurrence for a later tail, then removes the
// earliest surplus columns in positive-tail order.
func TestCompilerProjectionSourceColumnSelection(t *testing.T) {
	f := newTableRelationFixture(t)
	must := func(err error) { t.Helper(); astTestMust(t, err) }
	vars := make([]ast.ASTHandle, 4)
	for i := range vars {
		var err error
		vars[i], err = f.ast.MkVar(uint32(i), f.boolean)
		must(err)
	}
	headDecl := finiteInterpretedProjectDeclaration(t, f.ast, "projection-source-head", []ast.ASTHandle{f.boolean, f.boolean}, f.boolean)
	head, err := f.ast.MkApp(headDecl, []ast.ASTHandle{vars[0], vars[0]})
	must(err)
	negativeDecl := finiteInterpretedProjectDeclaration(t, f.ast, "projection-source-negative", []ast.ASTHandle{f.boolean}, f.boolean)
	negative, err := f.ast.MkApp(negativeDecl, []ast.ASTHandle{vars[1]})
	must(err)
	guard, err := f.ast.MkEq(vars[2], f.truth)
	must(err)

	tail1Decl := finiteInterpretedProjectDeclaration(t, f.ast, "projection-source-tail1", []ast.ASTHandle{f.boolean, f.boolean, f.boolean}, f.boolean)
	tail1, err := f.ast.MkApp(tail1Decl, []ast.ASTHandle{vars[0], vars[1], vars[2]})
	must(err)
	tail2Decl := finiteInterpretedProjectDeclaration(t, f.ast, "projection-source-tail2", []ast.ASTHandle{f.boolean, f.boolean, f.boolean, f.boolean, f.boolean}, f.boolean)
	tail2, err := f.ast.MkApp(tail2Decl, []ast.ASTHandle{vars[0], vars[1], vars[2], vars[0], vars[3]})
	must(err)

	compiler := finiteCompiler{manager: f.ast}
	removed, err := compiler.localIndexesForProjection(finiteCompiledRule{
		head: head, tail: tail1, tail2: tail2, negative: []ast.ASTHandle{negative}, interpreted: []ast.ASTHandle{guard},
	})
	must(err)
	if want := []int{0, 1, 2, 7}; !reflect.DeepEqual(removed, want) {
		t.Fatalf("removed columns = %v, want %v", removed, want)
	}
}
