package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazySparseEqualProjectAfterTransferredFilterForce(t *testing.T) {
	for _, projectFirst := range []bool{false, true} {
		engine := NewLazySparseEngine()
		table, err := engine.New(lazySignature(t, 2, 3), 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, fact := range []Fact{{0, 0}, {1, 1}, {1, 2}} {
			_, _ = table.Add(fact)
		}
		if err = table.FilterEqual(1, 0); err != nil {
			t.Fatal(err)
		}
		var projected *LazySparseTable
		if projectFirst {
			projected, err = table.Project([]int{0})
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err = table.Facts(); err != nil || table.root.left != nil {
			t.Fatal("equal force did not consume source", err)
		}
		if !projectFirst {
			projected, err = table.Project([]int{0})
			if err != nil {
				t.Fatal(err)
			}
		}
		facts, err := projected.Facts()
		if err != nil {
			t.Fatal(err)
		}
		instructionWantFacts(t, facts, Fact{2}, Fact{1})
		if err = engine.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLazySparseInterpretedProjectAfterTransferredFilterForce(t *testing.T) {
	for _, projectFirst := range []bool{false, true} {
		f := newTableRelationFixture(t)
		theory := tableRelationTheoryRewriter(t, f.ast)
		engine := NewLazySparseEngine()
		sorts := []ast.ASTHandle{f.boolean, f.bits}
		signature, err := tableRelationSignature(f.relations, sorts)
		if err != nil {
			t.Fatal(err)
		}
		table, err := engine.New(signature, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, fact := range []Fact{{0, 0}, {1, 1}, {1, 2}} {
			_, _ = table.Add(fact)
		}
		variable, _ := f.ast.MkVar(0, f.boolean)
		condition, _ := f.ast.MkEq(variable, f.truth)
		if err = table.FilterInterpreted(f.relations, sorts, theory, condition); err != nil {
			t.Fatal(err)
		}
		var projected *LazySparseTable
		if projectFirst {
			projected, err = table.Project([]int{0})
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err = table.Facts(); err != nil || table.root.left != nil {
			t.Fatal("interpreted force did not consume source", err)
		}
		if !projectFirst {
			projected, err = table.Project([]int{0})
			if err != nil {
				t.Fatal(err)
			}
		}
		facts, err := projected.Facts()
		if err != nil {
			t.Fatal(err)
		}
		instructionWantFacts(t, facts, Fact{2}, Fact{1})
		if err = engine.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
