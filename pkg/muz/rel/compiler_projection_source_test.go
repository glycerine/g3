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

	compiler := finiteCompiler{manager: f.ast}
	columns := map[uint32][]int{0: {0, 3, 6}, 1: {1, 4}, 2: {2, 5}, 3: {7}}
	removed, err := compiler.joinProjectionColumns(finiteCompiledRule{
		head: head, negative: []ast.ASTHandle{negative}, interpreted: []ast.ASTHandle{guard},
	}, 8, columns)
	must(err)
	if want := []int{0, 1, 2, 7}; !reflect.DeepEqual(removed, want) {
		t.Fatalf("removed columns = %v, want %v", removed, want)
	}
	if want := map[uint32][]int{0: {0, 3}, 1: {1}, 2: {2}}; !reflect.DeepEqual(columns, want) {
		t.Fatalf("remaining variable columns = %v, want %v", columns, want)
	}
}
