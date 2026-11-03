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

func TestSparseIntervalHashOriginalSnapshotsPreserveHashAndNullary(t *testing.T) {
	f, _, _ := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })

	for _, sorts := range [][]ast.ASTHandle{{f.boolean}, nil} {
		product, cause := NewSparseIntervalHashOriginal(relations, sorts, false)
		astTestMust(t, cause)
		values := []ast.ASTHandle{f.truth}
		if len(sorts) == 0 {
			values = nil
		}
		astTestMust(t, product.AddFact(values))
		snapshot, cause := product.HashSnapshot()
		astTestMust(t, cause)
		facts, cause := snapshot.Facts()
		astTestMust(t, cause)
		if len(facts) != 1 || len(facts[0]) != len(sorts) {
			t.Fatal("hash snapshot lost exact original row", len(sorts), facts)
		}
		astTestMust(t, product.Reset())
		length, cause := snapshot.Len()
		astTestMust(t, cause)
		if length != 1 {
			t.Fatal("hash snapshot borrowed product mutation", len(sorts), length)
		}
		if len(sorts) == 0 {
			if interval, intervalErr := product.ArithmeticSnapshot(); intervalErr == nil || interval != nil {
				t.Fatal("hash nullary exposed its transparent interval identity", interval, intervalErr)
			}
		}
		astTestMust(t, snapshot.Close())
		astTestMust(t, product.Close())
	}

	full, err := NewSparseIntervalHashOriginal(relations, nil, true)
	astTestMust(t, err)
	if empty, cause := full.Empty(); cause != nil || empty {
		t.Fatal("full nullary hash original lost the zero tuple", empty, cause)
	}
	astTestMust(t, full.Reset())
	if empty, cause := full.Empty(); cause != nil || !empty {
		t.Fatal("reset did not empty the exact nullary component", empty, cause)
	}
	if full.IntervalComponent().Empty() {
		t.Fatal("reset emptied the nullary missing-component identity")
	}
	astTestMust(t, full.Close())
}

func TestSparseIntervalArithmeticNullaryUsesIntervalTruthValue(t *testing.T) {
	f, _, _ := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })

	empty, err := NewSparseIntervalArithmeticOriginal(relations, nil, false)
	astTestMust(t, err)
	if isEmpty, cause := empty.Empty(); cause != nil || !isEmpty {
		t.Fatal("empty interval nullary was not false", isEmpty, cause)
	}
	astTestMust(t, empty.AddFact(nil))
	if isEmpty, cause := empty.Empty(); cause != nil || isEmpty {
		t.Fatal("interval nullary fact remained false", isEmpty, cause)
	}
	clone, err := empty.Clone()
	astTestMust(t, err)
	snapshot, err := empty.ArithmeticSnapshot()
	astTestMust(t, err)
	if snapshot.Empty() {
		t.Fatal("interval nullary snapshot lost truth")
	}
	if hash, cause := empty.HashSnapshot(); cause == nil || hash != nil {
		t.Fatal("interval nullary exposed its transparent finite identity", hash, cause)
	}
	astTestMust(t, empty.Reset())
	if isEmpty, cause := empty.Empty(); cause != nil || !isEmpty {
		t.Fatal("interval nullary reset reversed component roles", isEmpty, cause)
	}
	resetSnapshot, cause := empty.ArithmeticSnapshot()
	astTestMust(t, cause)
	if !resetSnapshot.Empty() {
		t.Fatal("reset interval nullary snapshot remained true")
	}
	if isEmpty, cause := clone.Empty(); cause != nil || isEmpty {
		t.Fatal("interval nullary clone borrowed reset", isEmpty, cause)
	}
	astTestMust(t, clone.Reset())
	if isEmpty, cause := clone.Empty(); cause != nil || !isEmpty {
		t.Fatal("clone lost interval-nullary role on reset", isEmpty, cause)
	}
	astTestMust(t, clone.Close())
	astTestMust(t, empty.Close())

	full, err := NewSparseIntervalArithmeticOriginal(relations, nil, true)
	astTestMust(t, err)
	if isEmpty, cause := full.Empty(); cause != nil || isEmpty {
		t.Fatal("full interval nullary was not true", isEmpty, cause)
	}
	astTestMust(t, full.Close())
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

func TestSparseIntervalArithmeticOriginalImportsOwnedSeedAndValidatesSorts(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	relations, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, relations.Close()) })
	seed := NewIntervalEmpty(1)
	two, four, ten := numeral("2"), numeral("4"), numeral("10")
	astTestMust(t, seed.AddFactExpressions(f.ast, []ast.ASTHandle{integer}, []ast.ASTHandle{two}))
	astTestMust(t, seed.AddFactExpressions(f.ast, []ast.ASTHandle{integer}, []ast.ASTHandle{four}))
	product, err := NewSparseIntervalArithmeticOriginalFrom(relations, []ast.ASTHandle{integer}, seed)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, product.Close()) })
	if err = product.MatchesSorts([]ast.ASTHandle{integer}); err != nil {
		t.Fatal("imported product rejected its exact signature", err)
	}
	if err = product.MatchesSorts([]ast.ASTHandle{f.boolean}); err == nil {
		t.Fatal("imported product accepted a different signature")
	}
	if !product.OwnedBy(relations) {
		t.Fatal("imported product rejected its exact relation-manager owner")
	}
	foreign, err := NewRelationManager(f.ast)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, foreign.Close()) })
	if product.OwnedBy(foreign) {
		t.Fatal("product accepted a different relation manager over the same AST manager")
	}
	three := numeral("3")
	if found, cause := product.ContainsFact([]ast.ASTHandle{three}); cause != nil || !found {
		t.Fatal("imported interval seed lost its hull", found, cause)
	}
	astTestMust(t, seed.FilterEqualExpression(f.ast, 0, ten))
	if found, cause := product.ContainsFact([]ast.ASTHandle{three}); cause != nil || !found {
		t.Fatal("imported interval seed borrowed later source mutation", found, cause)
	}
	if invalid, cause := NewSparseIntervalArithmeticOriginalFrom(relations, []ast.ASTHandle{integer, integer}, seed); cause == nil || invalid != nil {
		t.Fatal("seed import accepted mismatched arity", invalid, cause)
	}
}
