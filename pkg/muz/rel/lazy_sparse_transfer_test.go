package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestLazySparseMutatingForceTransfersPackedSource(t *testing.T) {
	for _, operation := range []string{"equal", "identical", "negation"} {
		t.Run(operation, func(t *testing.T) {
			engine := NewLazySparseEngine()
			defer engine.Close()
			signature := lazySignature(t, 2)
			facts := []Fact{{0}, {1}}
			if operation == "identical" {
				signature = lazySignature(t, 2, 2)
				facts = []Fact{{0, 0}, {0, 1}, {1, 1}}
			}
			table, err := engine.New(signature, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, fact := range facts {
				if _, err = table.Add(fact); err != nil {
					t.Fatal(err)
				}
			}
			switch operation {
			case "equal":
				err = table.FilterEqual(0, 0)
			case "identical":
				err = table.FilterIdentical([]int{0, 1})
			case "negation":
				negative, newErr := engine.New(signature, 0)
				if newErr != nil {
					t.Fatal(newErr)
				}
				if _, newErr = negative.Add(facts[len(facts)-1]); newErr != nil {
					t.Fatal(newErr)
				}
				err = table.FilterByNegation(negative, []int{0}, []int{0})
			}
			if err != nil {
				t.Fatal(err)
			}
			rows, err := table.Facts()
			if err != nil {
				t.Fatal(err)
			}
			if table.root.left != nil || table.root.retryForce {
				t.Fatal("mutating force did not finish transferred source ownership")
			}
			wantCount := 1
			if operation == "identical" {
				wantCount = 2
			}
			if len(rows) != wantCount {
				t.Fatal("unexpected transferred filter rows", rows)
			}
		})
	}
}

func TestLazySparseInterpretedForceRetriesTransferredPackedSource(t *testing.T) {
	f := newTableRelationFixture(t)
	rewriter := tableRelationTheoryRewriter(t, f.ast)
	engine := NewLazySparseEngine()
	defer engine.Close()
	table, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = table.Add(Fact{0})
	_, _ = table.Add(Fact{1})
	condition := lazyBooleanCondition(t, f.ast, f.boolean)
	if err = table.FilterInterpreted(f.relations, []ast.ASTHandle{f.boolean}, rewriter, condition); err != nil {
		t.Fatal(err)
	}
	if err = rewriter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Facts(); !errors.Is(err, ErrLazySparse) {
		t.Fatal("closed rewriter did not refuse deferred force", err)
	}
	if table.root.evaluated == nil || !table.root.retryForce || table.root.left != nil || table.Forced() {
		t.Fatal("failed force lost transferred owner or published incomplete result")
	}
	table.root.interpreted.rewriter = tableRelationTheoryRewriter(t, f.ast)
	rows, err := table.Facts()
	if err != nil {
		t.Fatal("retry", err)
	}
	instructionWantFacts(t, rows, Fact{0})
	if table.root.retryForce || !table.Forced() {
		t.Fatal("successful retry did not publish transferred result")
	}
}
