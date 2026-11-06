package rel

import (
	"errors"
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

func TestFormulaCheckedNumericRelationFilterEqualUsesSelectedBase(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	three, err := f.ast.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	astTestPin(t, f.ast, three)

	t.Run("interval backing checks and accepts the numeric filter", func(t *testing.T) {
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "interval_relation",
			interval: NewIntervalFull(1), checker: &config,
		}
		astTestMust(t, numeric.FilterEqual(0, three))
		if numeric.interval.Empty() {
			t.Fatal("verified interval filter made the full relation empty")
		}
	})

	t.Run("bound no-op is rejected by the selected formula checker", func(t *testing.T) {
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "bound_relation",
			bound: NewBoundRelation(1, false), checker: &config,
		}
		if err := numeric.FilterEqual(0, three); !errors.Is(err, ErrRelationVerification) {
			t.Fatalf("bound filter_equal source no-op was not detected: %v", err)
		}
	})
}

func TestFormulaCheckedNumericRelationFilterIdenticalMatchesSelectedBases(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)

	for _, kind := range []string{"bound_relation", "interval_relation"} {
		t.Run(kind, func(t *testing.T) {
			numeric := &relationExplanationNumeric{
				manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: kind, checker: &config,
			}
			switch kind {
			case "bound_relation":
				numeric.bound = NewBoundRelation(2, false)
			case "interval_relation":
				numeric.interval = NewIntervalFull(2)
			}
			if err := numeric.FilterIdentical([]uint32{0, 1}); err != nil {
				t.Fatalf("checked identical filter: %v", err)
			}
		})
	}
}
