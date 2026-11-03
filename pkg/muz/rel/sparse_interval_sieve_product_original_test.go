package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSparseIntervalOriginalMissingComponentsStayFull(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })

	finite, err := NewSparseIntervalFiniteOriginal(relations, []ast.ASTHandle{f.boolean}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, finite.Close()) })
	if finite.IntervalComponent().Empty() {
		t.Fatal("finite original's transparent interval component is empty")
	}
	empty, err := finite.Empty()
	astTestMust(t, err)
	if !empty {
		t.Fatal("empty finite component did not make product empty")
	}
	astTestMust(t, finite.AddFact([]ast.ASTHandle{f.truth}))
	if found, cause := finite.ContainsFact([]ast.ASTHandle{f.truth}); cause != nil || !found {
		t.Fatal("finite original lost added fact", found, cause)
	}
	astTestMust(t, finite.Reset())
	if finite.IntervalComponent().Empty() {
		t.Fatal("finite reset emptied its transparent interval component")
	}
	if empty, cause := finite.Empty(); cause != nil || !empty {
		t.Fatal("finite reset did not empty the exact component", empty, cause)
	}

	arithmetic, err := NewSparseIntervalArithmeticOriginal(relations, []ast.ASTHandle{integer}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, arithmetic.Close()) })
	if empty, cause := arithmetic.Empty(); cause != nil || !empty {
		t.Fatal("empty arithmetic component did not make product empty", empty, cause)
	}
	two := numeral("2")
	astTestMust(t, arithmetic.AddFact([]ast.ASTHandle{two}))
	if found, cause := arithmetic.ContainsFact([]ast.ASTHandle{two}); cause != nil || !found {
		t.Fatal("full zero-column finite component blocked arithmetic fact", found, cause)
	}
	astTestMust(t, arithmetic.Reset())
	astTestMust(t, arithmetic.AddFact([]ast.ASTHandle{two}))
	if found, cause := arithmetic.ContainsFact([]ast.ASTHandle{two}); cause != nil || !found {
		t.Fatal("arithmetic reset lost transparent finite identity", found, cause)
	}
}

func TestSparseIntervalOriginalSnapshotsAreOwnedAndClassified(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })

	finite, err := NewSparseIntervalFiniteOriginal(relations, []ast.ASTHandle{f.boolean}, false)
	astTestMust(t, err)
	astTestMust(t, finite.AddFact([]ast.ASTHandle{f.falsehood}))
	sparse, err := finite.FiniteSnapshot()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, sparse.Close()) })
	astTestMust(t, finite.FilterEqual(0, f.truth))
	if length, cause := sparse.Len(); cause != nil || length != 1 {
		t.Fatal("finite snapshot borrowed later source mutation", length, cause)
	}
	if _, cause := finite.ArithmeticSnapshot(); cause == nil {
		t.Fatal("finite original exposed an arithmetic snapshot")
	}

	arithmetic, err := NewSparseIntervalArithmeticOriginal(relations, []ast.ASTHandle{integer}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, arithmetic.Close()) })
	two, three := numeral("2"), numeral("3")
	astTestMust(t, arithmetic.AddFact([]ast.ASTHandle{two}))
	interval, err := arithmetic.ArithmeticSnapshot()
	astTestMust(t, err)
	astTestMust(t, arithmetic.FilterEqual(0, three))
	if found, cause := interval.ContainsFactExpressions(f.ast, []ast.ASTHandle{two}); cause != nil || !found {
		t.Fatal("arithmetic snapshot borrowed later source mutation", found, cause)
	}
	if _, cause := arithmetic.FiniteSnapshot(); cause == nil {
		t.Fatal("arithmetic original exposed a finite snapshot")
	}

	if invalid, cause := NewSparseIntervalFiniteOriginal(relations, []ast.ASTHandle{integer}, false); cause == nil || invalid != nil {
		t.Fatal("finite constructor accepted arithmetic signature", invalid, cause)
	}
	if invalid, cause := NewSparseIntervalArithmeticOriginal(relations, []ast.ASTHandle{f.boolean}, false); cause == nil || invalid != nil {
		t.Fatal("arithmetic constructor accepted finite signature", invalid, cause)
	}
}

func TestSparseIntervalOriginalJoinCompletesMissingComponents(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	finite, err := NewSparseIntervalFiniteOriginal(relations, []ast.ASTHandle{f.boolean}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, finite.Close()) })
	arithmetic, err := NewSparseIntervalArithmeticOriginal(relations, []ast.ASTHandle{integer}, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, arithmetic.Close()) })
	astTestMust(t, finite.AddFact([]ast.ASTHandle{f.truth}))
	two := numeral("2")
	astTestMust(t, arithmetic.AddFact([]ast.ASTHandle{two}))
	joined, err := JoinSparseIntervalSieveProducts(finite, arithmetic, nil, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, joined.Close()) })
	if found, cause := joined.ContainsFact([]ast.ASTHandle{f.truth, two}); cause != nil || !found {
		t.Fatal("implicit full component completion lost cross-product tuple", found, cause)
	}
	if found, cause := finite.ContainsFact([]ast.ASTHandle{f.truth}); cause != nil || !found {
		t.Fatal("heterogeneous join mutated finite source", found, cause)
	}
	if found, cause := arithmetic.ContainsFact([]ast.ASTHandle{two}); cause != nil || !found {
		t.Fatal("heterogeneous join mutated arithmetic source", found, cause)
	}
}
