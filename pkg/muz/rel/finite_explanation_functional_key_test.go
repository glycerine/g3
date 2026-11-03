package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationProjectAllUsesZeroColumnFunctionalKey(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	proof := func(text string) ast.ASTHandle {
		name, cause := f.ast.Symbols.Intern(text)
		astTestMust(t, cause)
		value, cause := f.ast.MkConst(name, ruleSort)
		astTestMust(t, cause)
		astTestPin(t, f.ast, value)
		return value
	}
	first, second := proof("zero-key-first-proof"), proof("zero-key-second-proof")
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood}, first))
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, second))

	projected, err := source.Project([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	if count, cause := projected.Len(); cause != nil || count != 1 {
		t.Fatal("project-all duplicated the empty functional key", count, cause)
	}
	got, found, err := projected.Explanation(nil)
	astTestMust(t, err)
	if !found || got != first {
		t.Fatal("project-all did not preserve the first explanation", got, found)
	}
}
