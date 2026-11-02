package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func relationExplanationProductWithProofRow(t *testing.T, f *tableRelationFixture, finiteSorts []ast.ASTHandle, facts []Fact, proofs []ast.ASTHandle) *RelationExplanationProduct {
	t.Helper()
	domains := make([]uint64, len(finiteSorts))
	for index, sort := range finiteSorts {
		var err error
		domains[index], err = f.relations.sortDomain(sort)
		must(t, err)
	}
	signature, err := NewSignature(domains)
	must(t, err)
	table := NewHashTable(signature)
	for _, fact := range facts {
		_, err = table.Add(fact)
		must(t, err)
	}
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	proofSorts := make([]ast.ASTHandle, len(proofs))
	for index := range proofSorts {
		proofSorts[index] = ruleSort
	}
	explanation, err := NewExplanationRelation(f.ast, proofSorts)
	must(t, err)
	must(t, explanation.AddFact(proofs))
	return &RelationExplanationProduct{manager: f.relations, finiteSorts: append([]ast.ASTHandle(nil), finiteSorts...), table: table, explanation: explanation}
}

func TestRelationExplanationProductProjectColumnsKeepsHeadProof(t *testing.T) {
	f := newTableRelationFixture(t)
	tailA := relationExplanationProof(t, f.ast, "relation-project-tail-a")
	tailB := relationExplanationProof(t, f.ast, "relation-project-tail-b")
	head := relationExplanationProof(t, f.ast, "relation-project-head")
	source := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean, f.boolean}, []Fact{{0, 1}}, []ast.ASTHandle{tailA, tailB, head})
	// Normalized columns are finite0, finite1, tailA, tailB, head.
	projected, err := source.ProjectColumns([]int{0, 2, 3})
	must(t, err)
	must(t, source.Close())
	defer func() { must(t, projected.Close()) }()

	proof, found, err := projected.Explanation([]ast.ASTHandle{f.truth})
	if err != nil || !found || proof != head {
		t.Fatal("assigned head proof projection", proof, found, err)
	}
	facts, err := projected.table.Facts()
	must(t, err)
	if len(facts) != 1 || len(facts[0]) != 1 || facts[0][0] != 1 {
		t.Fatal("finite projection", facts)
	}
}

func TestRelationExplanationProductProjectColumnsZeroProofJoinIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	leftProof := relationExplanationProof(t, f.ast, "relation-project-zero-left")
	rightProof := relationExplanationProof(t, f.ast, "relation-project-zero-right")
	left := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, []Fact{{1}}, []ast.ASTHandle{leftProof})
	identity, err := left.ProjectColumns([]int{1})
	must(t, err)
	must(t, left.Close())
	empty, err := identity.explanation.Empty()
	must(t, err)
	row, err := identity.explanation.Data()
	must(t, err)
	if empty || len(row) != 0 {
		t.Fatal("projected proof identity", empty, row)
	}

	right := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, []Fact{{1}}, []ast.ASTHandle{rightProof})
	joined, err := identity.Join(right, []int{0}, []int{0})
	must(t, err)
	must(t, right.Close())
	must(t, identity.Close())
	defer func() { must(t, joined.Close()) }()
	row, err = joined.explanation.Data()
	must(t, err)
	if len(row) != 1 || row[0] != rightProof {
		t.Fatal("zero-proof identity duplicated proof suffix", row)
	}
}

func TestRelationExplanationProductProjectColumnsFailureLeavesSource(t *testing.T) {
	f := newTableRelationFixture(t)
	first := relationExplanationProof(t, f.ast, "relation-project-failure-first")
	second := relationExplanationProof(t, f.ast, "relation-project-failure-second")
	source := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, []Fact{{1}}, []ast.ASTHandle{first, second})
	defer func() { must(t, source.Close()) }()
	for _, removed := range [][]int{{2, 1}, {1, 1}, {3}} {
		if projected, err := source.ProjectColumns(removed); err == nil || projected != nil {
			t.Fatal("invalid logical projection accepted", removed, projected, err)
		}
	}
	facts, err := source.table.Facts()
	must(t, err)
	proofs, err := source.explanation.Data()
	must(t, err)
	if len(facts) != 1 || facts[0][0] != 1 || len(proofs) != 2 || proofs[0] != first || proofs[1] != second {
		t.Fatal("failed projection mutated source", facts, proofs)
	}
}
