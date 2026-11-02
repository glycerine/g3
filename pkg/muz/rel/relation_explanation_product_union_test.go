package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func relationExplanationProof(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := manager.MkDatalogRuleSort()
	astTestMust(t, err)
	symbol, err := manager.Symbols.Intern(name)
	astTestMust(t, err)
	proof, err := manager.MkConst(symbol, sort)
	astTestMust(t, err)
	return proof
}

func relationExplanationProductWithFact(t *testing.T, f *tableRelationFixture, fact Fact, proof ast.ASTHandle) *RelationExplanationProduct {
	t.Helper()
	signature, err := NewSignature([]uint64{2})
	must(t, err)
	table := NewHashTable(signature)
	_, err = table.Add(fact)
	must(t, err)
	product, err := newRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean}, table, proof)
	must(t, err)
	must(t, table.Close())
	return product
}

func requireRelationUnionProof(t *testing.T, manager *ast.ASTManager, proof, left, right ast.ASTHandle) {
	t.Helper()
	declaration, err := manager.GetDecl(proof)
	astTestMust(t, err)
	name, err := manager.GetASTName(declaration)
	astTestMust(t, err)
	text, err := manager.Symbols.Display(name)
	astTestMust(t, err)
	first, err := manager.GetArg(proof, 0)
	astTestMust(t, err)
	second, err := manager.GetArg(proof, 1)
	astTestMust(t, err)
	if text != "e_union" || first != left || second != right {
		t.Fatal("whole-relation proof aggregation", text, first, second, left, right)
	}
}

func TestRelationExplanationProductCloneAndDistinctRowUnionDelta(t *testing.T) {
	f := newTableRelationFixture(t)
	a := relationExplanationProof(t, f.ast, "relation-proof-a")
	b := relationExplanationProof(t, f.ast, "relation-proof-b")
	d := relationExplanationProof(t, f.ast, "relation-proof-d")
	target := relationExplanationProductWithFact(t, f, Fact{0}, a)
	source := relationExplanationProductWithFact(t, f, Fact{1}, b)
	delta := relationExplanationProductWithFact(t, f, Fact{0}, d)
	defer func() { must(t, delta.Close()); must(t, source.Close()); must(t, target.Close()) }()

	clone, err := target.Clone()
	must(t, err)
	must(t, target.Close())
	target = clone
	proof, found, err := target.Explanation([]ast.ASTHandle{f.falsehood})
	if err != nil || !found || proof != a {
		t.Fatal("clone whole-relation proof", proof, found, err)
	}

	must(t, target.Union(source, delta))
	if rows, err := target.table.Len(); err != nil || rows != 2 {
		t.Fatal("union finite rows", rows, err)
	}
	if rows, err := delta.table.Len(); err != nil || rows != 2 {
		t.Fatal("delta finite rows", rows, err)
	}
	for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
		proof, found, err = target.Explanation([]ast.ASTHandle{value})
		if err != nil || !found {
			t.Fatal("target aggregated explanation", value, found, err)
		}
		requireRelationUnionProof(t, f.ast, proof, a, b)
		proof, found, err = delta.Explanation([]ast.ASTHandle{value})
		if err != nil || !found {
			t.Fatal("delta aggregated explanation", value, found, err)
		}
		requireRelationUnionProof(t, f.ast, proof, d, b)
	}
}

func TestRelationExplanationProductEmptyCloneAndUnion(t *testing.T) {
	f := newTableRelationFixture(t)
	empty, err := NewRelationExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	must(t, err)
	clone, err := empty.Clone()
	must(t, err)
	defer func() { must(t, clone.Close()); must(t, empty.Close()) }()
	must(t, empty.Union(clone, nil))
	if rows, err := empty.table.Len(); err != nil || rows != 0 {
		t.Fatal("empty relation product union", rows, err)
	}
}

func TestRelationExplanationProductEmptyFiniteSourceStillAggregatesProof(t *testing.T) {
	f := newTableRelationFixture(t)
	a := relationExplanationProof(t, f.ast, "empty-finite-proof-a")
	b := relationExplanationProof(t, f.ast, "empty-finite-proof-b")
	d := relationExplanationProof(t, f.ast, "empty-finite-proof-d")
	target := relationExplanationProductWithFact(t, f, Fact{0}, a)
	source := relationExplanationProductWithFact(t, f, Fact{1}, b)
	delta := relationExplanationProductWithFact(t, f, Fact{0}, d)
	defer func() { must(t, delta.Close()); must(t, source.Close()); must(t, target.Close()) }()
	// Finite filtering can empty one sieve while the independent whole-proof
	// sieve remains populated. Source aligned union still combines that proof.
	must(t, source.table.Reset())
	must(t, target.Union(source, delta))
	if rows, err := target.table.Len(); err != nil || rows != 1 {
		t.Fatal("empty finite source changed target rows", rows, err)
	}
	if rows, err := delta.table.Len(); err != nil || rows != 1 {
		t.Fatal("empty finite source changed delta rows", rows, err)
	}
	proof, found, err := target.Explanation([]ast.ASTHandle{f.falsehood})
	if err != nil || !found {
		t.Fatal("target proof after empty finite source", found, err)
	}
	requireRelationUnionProof(t, f.ast, proof, a, b)
	proof, found, err = delta.Explanation([]ast.ASTHandle{f.falsehood})
	if err != nil || !found {
		t.Fatal("delta proof after empty finite source", found, err)
	}
	requireRelationUnionProof(t, f.ast, proof, d, b)
}
