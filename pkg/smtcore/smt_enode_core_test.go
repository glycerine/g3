package smtcore

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtEnodeConstant(t *testing.T, m *ast.ASTManager, name uint32, sort ast.ASTHandle, generation uint32) (ast.ASTHandle, *SMTEnode) {
	t.Helper()
	owner, err := m.MkConst(symbol.Numerical(name), sort)
	astTestMust(t, err)
	node, err := NewSMTEnode(m, owner, nil, SMTEnodeOptions{Generation: generation, CongruenceEnabled: true})
	astTestMust(t, err)
	return owner, node
}

func TestSMTEnodeConstructionParentsFlagsAndTheoryVariables(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2000), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2001, sort, 7)
	b, bn := smtEnodeConstant(t, m, 2002, sort, 3)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{Commutative: true}, false)
	astTestMust(t, err)
	declaration, err := m.MkFuncDeclWithInfo(symbol.Numerical(2003), []ast.ASTHandle{sort, sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	owner, err := m.MkApp(declaration, []ast.ASTHandle{a, b})
	astTestMust(t, err)
	node, err := NewSMTEnode(m, owner, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{a: an, b: bn}), SMTEnodeOptions{
		Generation: 11, InternalScopeLevel: 4, CongruenceEnabled: true, UpdateChildrenParent: true,
	})
	astTestMust(t, err)
	if node.Root() != node || node.Next() != node || node.ClassSize() != 1 || node.Generation() != 11 || node.InternalScopeLevel() != 4 || node.FunctionDeclID() != math.MaxUint32 {
		t.Fatal("source scalar initialization")
	}
	if !node.IsCommutative() || node.IsBool() || node.IsEquality() || node.NumArgs() != 2 || an.NumParents() != 1 || bn.NumParents() != 1 || an.Parents()[0] != node {
		t.Fatal("arguments, flags or parent use lists")
	}
	astTestMust(t, node.AddTheoryVar(3, 17))
	astTestMust(t, node.AddTheoryVar(5, 29))
	if node.NumTheoryVars() != 2 || node.TheoryVar(3) != 17 || node.TheoryVar(5) != 29 {
		t.Fatal("theory variable insertion")
	}
	astTestMust(t, node.ReplaceTheoryVar(3, 19))
	astTestMust(t, node.DeleteTheoryVar(3))
	if node.NumTheoryVars() != 1 || node.TheoryVar(3) != SMTNullTheoryVar || node.TheoryVar(5) != 29 {
		t.Fatal("theory variable replacement/deletion")
	}
	astTestMust(t, node.DeleteTheoryVar(5))
	if node.NumTheoryVars() != 0 {
		t.Fatal("inline theory cell did not return to null")
	}
	if err := node.MarkAsInterpreted(); err != ast.ErrASTDomain {
		t.Fatal("nonconstant interpreted guard", err)
	}
	astTestMust(t, an.MarkAsInterpreted())
}

func TestSMTEnodeCongruenceCommutationEqualityClassAndMarks(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2010), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2011, sort, 8)
	b, bn := smtEnodeConstant(t, m, 2012, sort, 2)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{Commutative: true}, false)
	astTestMust(t, err)
	declaration, err := m.MkFuncDeclWithInfo(symbol.Numerical(2013), []ast.ASTHandle{sort, sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	leftOwner, err := m.MkApp(declaration, []ast.ASTHandle{a, b})
	astTestMust(t, err)
	rightOwner, err := m.MkApp(declaration, []ast.ASTHandle{b, a})
	astTestMust(t, err)
	lookup := map[ast.ASTHandle]*SMTEnode{a: an, b: bn}
	left, err := NewSMTEnode(m, leftOwner, smtEnodeMapLookup(lookup), SMTEnodeOptions{Generation: 9, CongruenceEnabled: true})
	astTestMust(t, err)
	right, err := NewSMTEnode(m, rightOwner, smtEnodeMapLookup(lookup), SMTEnodeOptions{Generation: 4, CongruenceEnabled: true})
	astTestMust(t, err)
	yes, commuted, err := SMTEnodesCongruent(left, right)
	astTestMust(t, err)
	if !yes || !commuted {
		t.Fatal("commutative congruence", yes, commuted)
	}
	left.root, left.next, left.classSize = left, right, 2
	right.root, right.next = left, left
	if right.EqualityClassMinGeneration() != right || left.EqualityClassMinGeneration() != right || SMTEnodesMaxGeneration([]*SMTEnode{an, bn, left, right}) != 9 {
		t.Fatal("equality-class generation selection")
	}
	astTestMust(t, left.SetMark(true))
	astTestMust(t, right.SetMark(true))
	astTestMust(t, left.SetMark2(true))
	astTestMust(t, right.SetMark2(true))
	astTestMust(t, SMTUnmarkEnodes([]*SMTEnode{left, right}))
	astTestMust(t, SMTUnmarkEnodes2([]*SMTEnode{left, right}))
	if left.IsMarked() || right.IsMarked() || left.IsMarked2() || right.IsMarked2() {
		t.Fatal("bulk unmark")
	}
	equalityOwner, err := m.MkEq(a, a)
	astTestMust(t, err)
	equality, err := NewSMTEnode(m, equalityOwner, smtEnodeMapLookup(lookup), SMTEnodeOptions{MergeTruthValue: true, CongruenceEnabled: true})
	astTestMust(t, err)
	if !equality.IsBool() || !equality.IsEquality() || !equality.IsTrueEquality() {
		t.Fatal("Boolean equality flags")
	}
}
