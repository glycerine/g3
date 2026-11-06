package rel

import (
	"errors"
	"math/big"
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

func TestNumericRelationInterpretedFilterMatchesSourceWrapper(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)
	variable, err := f.ast.MkVar(0, integer)
	astTestMust(t, err)
	three, err := f.ast.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	astTestPin(t, f.ast, three)
	condition, err := f.ast.MkArithmeticLT(variable, three)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)

	t.Run("interval applies and checks supported unary comparison", func(t *testing.T) {
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "interval_relation",
			interval: NewIntervalFull(1), checker: &config,
		}
		astTestMust(t, numeric.FilterInterpreted(condition))
		lower, _, upper, upperOpen, err := numeric.interval.Bounds(0)
		astTestMust(t, err)
		if lower != nil || upper == nil || upper.Cmp(big.NewRat(2, 1)) != 0 || upperOpen {
			t.Fatal("interval strict upper bound", lower, upper, upperOpen)
		}
	})

	t.Run("bound unsupported comparison is a source no-op", func(t *testing.T) {
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "bound_relation",
			bound: NewBoundRelation(1, false),
		}
		astTestMust(t, numeric.FilterInterpreted(condition))
		if numeric.bound.Empty() {
			t.Fatal("unsupported bound filter changed the full relation")
		}
	})

	t.Run("checked bound no-op is rejected", func(t *testing.T) {
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "bound_relation",
			bound: NewBoundRelation(1, false), checker: &config,
		}
		if err := numeric.FilterInterpreted(condition); !errors.Is(err, ErrRelationVerification) {
			t.Fatalf("checked unsupported bound filter was not detected: %v", err)
		}
	})
}

func TestFormulaCheckedNumericRelationProjectMatchesSource(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)

	t.Run("bound existentially removes the constrained column", func(t *testing.T) {
		source := NewBoundRelation(2, false)
		astTestMust(t, source.AddLess(0, 1))
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: "bound_relation",
			bound: source, checker: &config,
		}
		projected, err := numeric.Project([]uint32{0})
		astTestMust(t, err)
		defer projected.Close()
		if projected.checker != &config || len(projected.sorts) != 1 {
			t.Fatal("projected numeric backing lost source checker or result signature")
		}
	})

	t.Run("interval existentially removes a point column", func(t *testing.T) {
		source := NewIntervalFull(2)
		astTestMust(t, source.FilterEqual(0, big.NewRat(3, 1)))
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: "interval_relation",
			interval: source, checker: &config,
		}
		projected, err := numeric.Project([]uint32{0})
		astTestMust(t, err)
		defer projected.Close()
		if projected.checker != &config || len(projected.sorts) != 1 {
			t.Fatal("projected numeric backing lost source checker or result signature")
		}
	})
}

func TestFormulaCheckedNumericRelationRenameMatchesSource(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)

	t.Run("bound constraint follows the column permutation", func(t *testing.T) {
		source := NewBoundRelation(2, false)
		astTestMust(t, source.AddLess(0, 1))
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: "bound_relation",
			bound: source, checker: &config,
		}
		renamed, err := numeric.Rename([]uint32{0, 1})
		astTestMust(t, err)
		defer renamed.Close()
		if renamed.checker != &config {
			t.Fatal("renamed bound relation lost its formula checker")
		}
	})

	t.Run("interval point follows the column permutation", func(t *testing.T) {
		source := NewIntervalFull(2)
		astTestMust(t, source.FilterEqual(0, big.NewRat(3, 1)))
		numeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: "interval_relation",
			interval: source, checker: &config,
		}
		renamed, err := numeric.Rename([]uint32{0, 1})
		astTestMust(t, err)
		defer renamed.Close()
		if renamed.checker != &config {
			t.Fatal("renamed interval relation lost its formula checker")
		}
	})
}

func TestFormulaCheckedNumericRelationUnionMatchesSource(t *testing.T) {
	f := newTableRelationFixture(t)
	config := formulaCheckerConfig(t, f.ast)
	arithmetic, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(arithmetic, 1, nil)
	astTestMust(t, err)

	t.Run("bound duplicate constraint", func(t *testing.T) {
		target := NewBoundRelation(2, false)
		source := NewBoundRelation(2, false)
		astTestMust(t, target.AddLess(0, 1))
		astTestMust(t, source.AddLess(0, 1))
		targetNumeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: "bound_relation",
			bound: target, checker: &config,
		}
		sourceNumeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer, integer}, kind: "bound_relation", bound: source,
		}
		astTestMust(t, targetNumeric.UnionFrom(sourceNumeric, nil))
	})

	t.Run("interval duplicate point", func(t *testing.T) {
		target := NewIntervalFull(1)
		source := NewIntervalFull(1)
		astTestMust(t, target.FilterEqual(0, big.NewRat(3, 1)))
		astTestMust(t, source.FilterEqual(0, big.NewRat(3, 1)))
		targetNumeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "interval_relation",
			interval: target, checker: &config,
		}
		sourceNumeric := &relationExplanationNumeric{
			manager: f.relations, sorts: []ast.ASTHandle{integer}, kind: "interval_relation", interval: source,
		}
		astTestMust(t, targetNumeric.UnionFrom(sourceNumeric, nil))
	})
}
