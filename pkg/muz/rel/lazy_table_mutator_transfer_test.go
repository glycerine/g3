package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazyTableMutatingForceReleasesInterpretedSourceOwners(t *testing.T) {
	for _, operation := range []string{"equal", "identical", "negation"} {
		t.Run(operation, func(t *testing.T) {
			f := newTableRelationFixture(t)
			rewriter := tableRelationTheoryRewriter(t, f.ast)
			engine := NewLazyTableEngine()
			t.Cleanup(func() {
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			})
			sorts := []ast.ASTHandle{f.boolean}
			signature := lazySignature(t, 2)
			facts := []Fact{{0}, {1}}
			if operation == "identical" {
				sorts = []ast.ASTHandle{f.boolean, f.boolean}
				signature = lazySignature(t, 2, 2)
				facts = []Fact{{0, 0}, {0, 1}, {1, 1}}
			}
			table := newLazyTableWithFacts(t, engine, signature, facts...)
			condition := lazyBooleanCondition(t, f.ast, f.boolean)
			baseline := f.ast.Stats().LiveReferences
			if err := table.FilterInterpreted(
				f.relations,
				sorts,
				rewriter,
				[]ast.ASTHandle{condition},
				map[uint32]int{0: 0},
				map[uint32]ast.ASTHandle{0: f.boolean},
			); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "equal":
				if err := table.FilterEqual(0, 0); err != nil {
					t.Fatal(err)
				}
			case "identical":
				if err := table.FilterIdentical([]int{0, 1}); err != nil {
					t.Fatal(err)
				}
			case "negation":
				negative := newLazyTableWithFacts(t, engine, signature, Fact{1})
				closeLazyTables(t, negative)
				if err := table.FilterByNegation(negative, []int{0}, []int{0}); err != nil {
					t.Fatal(err)
				}
			}
			if got := f.ast.Stats().LiveReferences; got != baseline+1 {
				t.Fatalf("interpreted source owner missing before force: got %d want %d", got, baseline+1)
			}
			rows, err := table.Facts()
			if err != nil {
				t.Fatal(err)
			}
			want := []Fact{{0}}
			if operation == "identical" {
				want = []Fact{{0, 0}}
			}
			instructionWantFacts(t, rows, want...)
			if table.root.left != nil {
				t.Fatal("mutating force kept transferred source edge")
			}
			if got := f.ast.Stats().LiveReferences; got != baseline {
				t.Fatalf("mutating force kept source condition owner: got %d want %d", got, baseline)
			}
			if err = table.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
