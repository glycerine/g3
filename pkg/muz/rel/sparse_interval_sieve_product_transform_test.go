package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalSieveProductProjectionSupportsPureAndNullaryResults(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	p, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer}, []string{"tr_sparse", "interval_relation"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	two := numeral("2")
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth, two}))
	finite, err := p.Project([]int{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, finite.Close()) }()
	if found, cause := finite.ContainsFact([]ast.ASTHandle{f.truth}); cause != nil || !found {
		t.Fatal("pure finite projection", found, cause)
	}
	arithmetic, err := p.Project([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, arithmetic.Close()) }()
	if found, cause := arithmetic.ContainsFact([]ast.ASTHandle{two}); cause != nil || !found {
		t.Fatal("pure arithmetic projection", found, cause)
	}
	nullary, err := p.Project([]int{0, 1})
	astTestMust(t, err)
	defer func() { astTestMust(t, nullary.Close()) }()
	if found, cause := nullary.ContainsFact(nil); cause != nil || !found {
		t.Fatal("nullary projection", found, cause)
	}
}

func TestSparseIntervalSieveProductRenameCrossesComponentPositions(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	defer func() { astTestMust(t, relations.Close()) }()
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	five, err := f.ast.ArithmeticRealNumeralText("5")
	astTestMust(t, err)
	astTestPin(t, f.ast, five)
	p, err := NewSparseIntervalSieveProduct(relations, []ast.ASTHandle{f.boolean, integer, f.boolean, real}, []string{"interval_relation", "tr_sparse"}, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	two := numeral("2")
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth, two, f.falsehood, five}))
	renamed, err := p.Rename([]int{0, 1, 2, 3})
	astTestMust(t, err)
	defer func() { astTestMust(t, renamed.Close()) }()
	if got := renamed.FiniteColumns(); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatal("renamed finite columns", got)
	}
	found, err := renamed.ContainsFact([]ast.ASTHandle{two, f.falsehood, five, f.truth})
	if err != nil || !found {
		t.Fatal("outer rename did not preserve component tuple", found, err)
	}
	original, err := p.ContainsFact([]ast.ASTHandle{f.truth, two, f.falsehood, five})
	if err != nil || !original {
		t.Fatal("rename modified source", original, err)
	}
}
