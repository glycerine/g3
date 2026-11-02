package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRelationExplanationProductJoinProjectOwnsIndependentMultiProof(t *testing.T) {
	f := newTableRelationFixture(t)
	a := relationExplanationProof(t, f.ast, "relation-join-a")
	b := relationExplanationProof(t, f.ast, "relation-join-b")
	left := relationExplanationProductWithFact(t, f, Fact{1}, a)
	right := relationExplanationProductWithFact(t, f, Fact{1}, b)
	joined, err := left.Join(right, []int{0}, []int{0})
	must(t, err)
	projected, err := joined.Project([]int{1})
	must(t, err)
	must(t, joined.Close())
	must(t, right.Close())
	must(t, left.Close())
	defer func() { must(t, projected.Close()) }()

	facts, err := projected.table.Facts()
	must(t, err)
	if len(facts) != 1 || len(facts[0]) != 1 || facts[0][0] != 1 {
		t.Fatal("projected finite sieve", facts)
	}
	proofs, err := projected.explanation.Data()
	must(t, err)
	if len(proofs) != 2 || proofs[0] != a || proofs[1] != b {
		t.Fatal("independent joined explanation sieve", proofs)
	}
	if _, _, err = projected.Explanation([]ast.ASTHandle{f.truth}); err == nil {
		t.Fatal("multi-proof intermediate exposed as final explanation")
	}
}

func TestRelationExplanationProductInterpretedFiniteGuardAndProofAssignment(t *testing.T) {
	f := newTableRelationFixture(t)
	manager := f.ast
	ruleSort, err := manager.MkDatalogRuleSort()
	astTestMust(t, err)
	a := relationExplanationProof(t, manager, "relation-filter-a")
	b := relationExplanationProof(t, manager, "relation-filter-b")
	explanation, err := NewExplanationRelation(manager, []ast.ASTHandle{ruleSort, ruleSort, ruleSort})
	must(t, err)
	must(t, explanation.AddFact([]ast.ASTHandle{a, b, {}}))
	signature, err := NewSignature([]uint64{2})
	must(t, err)
	table := NewHashTable(signature)
	_, err = table.Add(Fact{0})
	must(t, err)
	_, err = table.Add(Fact{1})
	must(t, err)
	product := &RelationExplanationProduct{manager: f.relations, finiteSorts: []ast.ASTHandle{f.boolean}, table: table, explanation: explanation}
	defer func() { must(t, product.Close()) }()

	own := func(handle ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, err)
		astTestPin(t, manager, handle)
		return handle
	}
	finiteVariable := own(manager.MkVar(0, f.boolean))
	finiteGuard := own(manager.MkEq(finiteVariable, f.truth))
	firstTail := own(manager.MkVar(3, ruleSort))
	secondTail := own(manager.MkVar(5, ruleSort))
	head := own(manager.MkVar(7, ruleSort))
	step, err := manager.Symbols.Intern("relation-filter-step")
	astTestMust(t, err)
	rule := own(manager.MkDatalogRule(step, []ast.ASTHandle{firstTail, secondTail}))
	assignment := own(manager.MkEq(head, rule))
	columns := map[uint32]int{0: 0, 3: 1, 5: 2, 7: 3}
	sorts := map[uint32]ast.ASTHandle{0: f.boolean, 3: ruleSort, 5: ruleSort, 7: ruleSort}
	must(t, product.FilterInterpreted(tableRelationTheoryRewriter(t, manager), []ast.ASTHandle{finiteGuard, assignment}, columns, sorts))

	facts, err := product.table.Facts()
	must(t, err)
	if len(facts) != 1 || facts[0][0] != 1 {
		t.Fatal("finite guard result", facts)
	}
	proofs, err := product.explanation.Data()
	must(t, err)
	want := own(manager.MkDatalogRule(step, []ast.ASTHandle{a, b}))
	if len(proofs) != 3 || proofs[0] != a || proofs[1] != b || proofs[2] != want {
		t.Fatal("relation-level proof assignment", proofs, want)
	}

	projected, err := product.Project([]int{0})
	must(t, err)
	defer func() { must(t, projected.Close()) }()
	must(t, product.Close())
	proofs, err = projected.explanation.Data()
	must(t, err)
	if len(proofs) != 3 || proofs[2] != want {
		t.Fatal("projection lost whole proof", proofs)
	}
	facts, err = projected.table.Facts()
	must(t, err)
	if len(facts) != 1 || len(facts[0]) != 0 {
		t.Fatal("nullary finite projection", facts)
	}
}

func TestRelationExplanationProductMixedInterpretedConditionIsSieveIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	proof := relationExplanationProof(t, f.ast, "relation-mixed-proof")
	product := relationExplanationProductWithFact(t, f, Fact{0}, proof)
	defer func() { must(t, product.Close()) }()
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	own := func(handle ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, err)
		astTestPin(t, f.ast, handle)
		return handle
	}
	finite := own(f.ast.MkVar(0, f.boolean))
	explanation := own(f.ast.MkVar(1, ruleSort))
	// Equality is ill-sorted across components and therefore cannot be built;
	// use a Boolean if-then-else whose condition and branch span both sieves.
	proofEqual := own(f.ast.MkEq(explanation, proof))
	mixed := own(f.ast.MkIte(proofEqual, finite, f.truth))
	must(t, product.FilterInterpreted(tableRelationTheoryRewriter(t, f.ast), []ast.ASTHandle{mixed}, map[uint32]int{0: 0, 1: 1}, map[uint32]ast.ASTHandle{0: f.boolean, 1: ruleSort}))
	if rows, err := product.table.Len(); err != nil || rows != 1 {
		t.Fatal("mixed sieve condition was not identity", rows, err)
	}
}
