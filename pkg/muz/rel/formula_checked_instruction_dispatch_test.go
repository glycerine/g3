package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedFiniteInstructionDispatchPreservesChecker(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	makeChecked := func(sorts []ast.ASTHandle, facts ...Fact) *FormulaCheckedTable {
		relation, err := NewTableRelation(f.relations, sorts)
		astTestMust(t, err)
		checked, err := NewFormulaCheckedTable(relation, config)
		astTestMust(t, err)
		for _, fact := range facts {
			_, err = checked.Add(fact)
			astTestMust(t, err)
		}
		return checked
	}
	left := makeChecked([]ast.ASTHandle{f.boolean, f.boolean}, Fact{1, 0}, Fact{1, 1})
	right := makeChecked([]ast.ASTHandle{f.boolean}, Fact{0}, Fact{1})
	defer func() { astTestMust(t, errors.Join(left.Close(), right.Close())) }()

	joined, err := finiteTableJoin(left, right, []int{1}, []int{0})
	astTestMust(t, err)
	if _, ok := joined.(*FormulaCheckedTable); !ok {
		t.Fatalf("join lost formula checker: %T", joined)
	}
	defer func() { astTestMust(t, joined.Close()) }()
	projected, err := finiteTableProject(joined, []int{1})
	astTestMust(t, err)
	if _, ok := projected.(*FormulaCheckedTable); !ok {
		t.Fatalf("project lost formula checker: %T", projected)
	}
	defer func() { astTestMust(t, projected.Close()) }()
	renamed, err := finiteTableRename(projected, []int{0, 1})
	astTestMust(t, err)
	if _, ok := renamed.(*FormulaCheckedTable); !ok {
		t.Fatalf("rename lost formula checker: %T", renamed)
	}
	defer func() { astTestMust(t, renamed.Close()) }()

	target, err := emptyFiniteLike(renamed)
	astTestMust(t, err)
	delta, err := emptyFiniteLike(renamed)
	astTestMust(t, err)
	defer func() { astTestMust(t, errors.Join(target.Close(), delta.Close())) }()
	astTestMust(t, finiteTableUnion(target, renamed, delta))
	if rows, err := delta.Len(); err != nil || rows != 2 {
		t.Fatal("formula delta novelty", rows, err)
	}
	astTestMust(t, finiteTableFilterEqual(target, 1, 0))
	if rows, err := target.Len(); err != nil || rows != 1 {
		t.Fatal("formula equal filter", rows, err)
	}
}

func TestFormulaCheckedBoundInterpretedDispatchMapsCompilerVariables(t *testing.T) {
	f := newTableRelationFixture(t)
	relation, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	checked, err := NewFormulaCheckedTable(relation, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	defer func() { astTestMust(t, checked.Close()) }()
	for _, fact := range []Fact{{0, 0}, {1, 0}, {0, 1}} {
		_, err = checked.Add(fact)
		astTestMust(t, err)
	}
	variable, err := f.ast.MkVar(0, f.boolean)
	astTestMust(t, err)
	condition, err := f.ast.MkEq(variable, f.truth)
	astTestMust(t, err)
	theory := tableRelationTheoryRewriter(t, f.ast)
	astTestMust(t, formulaCheckedFilterInterpretedBound(checked, theory, []ast.ASTHandle{condition}, map[uint32]int{0: 1}, map[uint32]ast.ASTHandle{0: f.boolean}))
	facts, err := checked.Facts()
	astTestMust(t, err)
	if len(facts) != 1 || len(facts[0]) != 2 || facts[0][0] != 0 || facts[0][1] != 1 {
		t.Fatal("mapped interpreted filter", facts)
	}
}
