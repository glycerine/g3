package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationExplanationProductIntervalBackingFiltersAndBuildsFormula(t *testing.T) {
	f := newTableRelationFixture(t)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	astTestMust(t, f.relations.SetRelationExplanationNumericBacking("interval_relation", nil))
	product, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{integer})
	astTestMust(t, err)
	defer func() { astTestMust(t, product.Close()) }()
	if product.numeric == nil || product.table != nil {
		t.Fatal("numeric relation was not selected as the explanation backing")
	}
	astTestMust(t, product.UnionNumericSeed(RelationExplanationNumericSeed{Kind: "interval_relation", Interval: NewIntervalFull(1)}))
	proof := relationExplanationProof(t, f.ast, "numeric-explanation-proof")
	astTestMust(t, product.explanation.AddFact([]ast.ASTHandle{proof}))
	three, err := f.ast.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	astTestPin(t, f.ast, three)
	astTestMust(t, product.FilterEqualAST(three, 0))
	if empty, err := product.Empty(); err != nil || empty {
		t.Fatal("numeric explanation relation became empty", empty, err)
	}
	formula, err := product.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	if _, err := f.ast.RefGet(formula); err != nil {
		t.Fatal("numeric explanation formula is not readable", err)
	}
}
