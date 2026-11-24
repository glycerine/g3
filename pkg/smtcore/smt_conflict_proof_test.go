package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTConflictProofReconstructsEnodeEqualityBranch(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	sort, err := m.MkSort(symbol.Numerical(2840), ast.ASTSortInfo{})
	astTestMust(t, err)
	_, left := smtEnodeConstant(t, m, 2841, sort, 0)
	_, root := smtEnodeConstant(t, m, 2842, sort, 0)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	fact, err := m.MkEq(left.owner, root.owner)
	astTestMust(t, err)
	leaf, err := m.MkProofHypothesis(fact)
	astTestMust(t, err)
	justification, err := NewSMTJustificationProofWrapper(m, leaf, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, justification.Release(m)) })
	left.root = root
	left.trans = newSMTEqualityEdge(nil, root, NewSMTEqualityJustification(justification))
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	proof, err := builder.equalityProof(left, root)
	astTestMust(t, err)
	got, err := m.GetProofFact(proof)
	astTestMust(t, err)
	if got != fact {
		t.Fatal("equality proof fact", got, fact)
	}
	reverse, err := builder.equalityProof(root, left)
	astTestMust(t, err)
	reverseFact, err := m.GetProofFact(reverse)
	astTestMust(t, err)
	wantReverse, err := m.MkEq(root.owner, left.owner)
	astTestMust(t, err)
	if reverseFact != wantReverse {
		t.Fatal("reverse equality proof fact", reverseFact, wantReverse)
	}
}

func TestSMTConflictProofBuildsEqualityRootAndConflictJustifications(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	variable := smtRelevancyBool(t, m, "proof-equality-root")
	variableNode, err := NewSMTEnode(m, variable, nil, SMTEnodeOptions{})
	astTestMust(t, err)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	truthNode, err := NewSMTEnode(m, truth, nil, SMTEnodeOptions{})
	astTestMust(t, err)
	astTestMust(t, truthNode.MarkAsInterpreted())
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	iff, err := m.MkEq(variable, truth)
	astTestMust(t, err)
	iffProof, err := m.MkProofHypothesis(iff)
	astTestMust(t, err)
	iffWrapper, err := NewSMTJustificationProofWrapper(m, iffProof, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, iffWrapper.Release(m)) })
	variableNode.root = truthNode
	variableNode.trans = newSMTEqualityEdge(nil, truthNode, NewSMTEqualityJustification(iffWrapper))
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	rootJustification, err := NewSMTEqualityRootJustification(variableNode)
	astTestMust(t, err)
	rootProof, err := builder.objectProof(rootJustification)
	astTestMust(t, err)
	rootFact, err := m.GetProofFact(rootProof)
	astTestMust(t, err)
	if rootFact != variable {
		t.Fatal("equality-root proof fact", rootFact, variable)
	}
	negativeVariable := smtRelevancyBool(t, m, "proof-equality-false-root")
	negativeNode, err := NewSMTEnode(m, negativeVariable, nil, SMTEnodeOptions{})
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	falseNode, err := NewSMTEnode(m, falsity, nil, SMTEnodeOptions{})
	astTestMust(t, err)
	astTestMust(t, falseNode.MarkAsInterpreted())
	negativeFact, err := m.MkNot(negativeVariable)
	astTestMust(t, err)
	negativeProof, err := m.MkProofHypothesis(negativeFact)
	astTestMust(t, err)
	negativeWrapper, err := NewSMTJustificationProofWrapper(m, negativeProof, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, negativeWrapper.Release(m)) })
	negativeNode.root = falseNode
	negativeNode.trans = newSMTEqualityEdge(nil, falseNode, NewSMTEqualityJustification(negativeWrapper))
	negativeRoot, err := NewSMTEqualityRootJustification(negativeNode)
	astTestMust(t, err)
	negativeRootProof, err := builder.objectProof(negativeRoot)
	astTestMust(t, err)
	negativeRootFact, err := m.GetProofFact(negativeRootProof)
	astTestMust(t, err)
	if negativeRootFact != negativeFact {
		t.Fatal("false equality-root proof fact", negativeRootFact, negativeFact)
	}

	sort, err := m.MkSort(symbol.Numerical(2850), ast.ASTSortInfo{})
	astTestMust(t, err)
	_, first := smtEnodeConstant(t, m, 2851, sort, 0)
	_, second := smtEnodeConstant(t, m, 2852, sort, 0)
	astTestMust(t, first.MarkAsInterpreted())
	astTestMust(t, second.MarkAsInterpreted())
	equality, err := m.MkEq(first.owner, second.owner)
	astTestMust(t, err)
	equalityProof, err := m.MkProofHypothesis(equality)
	astTestMust(t, err)
	equalityWrapper, err := NewSMTJustificationProofWrapper(m, equalityProof, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, equalityWrapper.Release(m)) })
	conflict, err := NewSMTEqualityConflictJustification(first, second, NewSMTEqualityJustification(equalityWrapper))
	astTestMust(t, err)
	conflictProof, err := builder.objectProof(conflict)
	astTestMust(t, err)
	conflictFact, err := m.GetProofFact(conflictProof)
	astTestMust(t, err)
	if conflictFact != falsity {
		t.Fatal("equality-conflict proof fact", conflictFact, falsity)
	}
}

func TestSMTConflictProofBuildsCongruenceEquality(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	sort, err := m.MkSort(symbol.Numerical(2860), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2861, sort, 0)
	a2, an2 := smtEnodeConstant(t, m, 2862, sort, 0)
	b, bn := smtEnodeConstant(t, m, 2863, sort, 0)
	decl := smtCGTestDeclaration(t, m, 2864, sort, ast.ASTFuncFlags{})
	first := smtCGTestApplication(t, m, decl, []ast.ASTHandle{a, b}, []*SMTEnode{an, bn})
	second := smtCGTestApplication(t, m, decl, []ast.ASTHandle{a2, b}, []*SMTEnode{an2, bn})
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	childFact, err := m.MkEq(a, a2)
	astTestMust(t, err)
	childProof, err := m.MkProofHypothesis(childFact)
	astTestMust(t, err)
	childWrapper, err := NewSMTJustificationProofWrapper(m, childProof, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, childWrapper.Release(m)) })
	an.root = an2
	an.trans = newSMTEqualityEdge(nil, an2, NewSMTEqualityJustification(childWrapper))
	first.root = second
	first.trans = newSMTEqualityEdge(nil, second, NewSMTEqualityCongruence(false))
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	proof, err := builder.equalityProof(first, second)
	astTestMust(t, err)
	fact, err := m.GetProofFact(proof)
	astTestMust(t, err)
	want, err := m.MkEq(first.owner, second.owner)
	astTestMust(t, err)
	if fact != want {
		t.Fatal("congruence proof fact", fact, want)
	}
}

// smtcore_02-F8: ext_theory_eq_propagation_justification::mk_proof states
// ctx.mk_eq_atom(lhs, rhs), which puts the smaller id first.
func TestSMTConflictProofExtTheoryEqualityUsesEqAtomOrder(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	sort, err := m.MkSort(symbol.Numerical(2860), ast.ASTSortInfo{})
	astTestMust(t, err)
	_, small := smtEnodeConstant(t, m, 2861, sort, 0)
	_, large := smtEnodeConstant(t, m, 2862, sort, 0)
	smallID, err := m.GetASTID(small.owner)
	astTestMust(t, err)
	largeID, err := m.GetASTID(large.owner)
	astTestMust(t, err)
	if smallID >= largeID {
		t.Fatal("fixture ids", smallID, largeID)
	}
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	justification, err := NewSMTExtTheoryEqualityPropagationJustification(m, SMTTheoryID(family), nil, nil, large, small, nil)
	astTestMust(t, err)
	builder := &smtConflictProofBuilder{
		state: state, marked: make(map[int32]struct{}), literal: make(map[SMTLiteral]ast.ASTHandle),
		objects: make(map[SMTJustification]ast.ASTHandle), equality: make(map[smtEnodePairKey]ast.ASTHandle),
	}
	proof, err := builder.objectProof(justification)
	astTestMust(t, err)
	got, err := m.GetProofFact(proof)
	astTestMust(t, err)
	want, err := m.MkEq(small.owner, large.owner)
	astTestMust(t, err)
	if got != want {
		t.Fatal("eq-propagate proof fact is not mk_eq_atom order", got, want)
	}
}
