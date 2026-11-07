package smtcore

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSMTEnodeTemporaryKeyInvariantDisplayAndDeletion(t *testing.T) {
	m := astTestManager(t)
	sort, err := m.MkSort(symbol.Numerical(2300), ast.ASTSortInfo{})
	astTestMust(t, err)
	a, an := smtEnodeConstant(t, m, 2301, sort, 0)
	b, bn := smtEnodeConstant(t, m, 2302, sort, 2)
	info, err := m.NewFuncDeclInfo(ast.NullFamilyID, -1, nil, ast.ASTFuncFlags{Commutative: true}, false)
	astTestMust(t, err)
	declaration, err := m.MkFuncDeclWithInfo(symbol.Numerical(2303), []ast.ASTHandle{sort, sort}, sort, info)
	astTestMust(t, err)
	astTestMust(t, m.DelFuncDeclInfo(&info))
	owner, err := m.MkApp(declaration, []ast.ASTHandle{a, b})
	astTestMust(t, err)
	concrete, err := NewSMTEnode(m, owner, smtEnodeMapLookup(map[ast.ASTHandle]*SMTEnode{a: an, b: bn}), SMTEnodeOptions{CongruenceEnabled: true, UpdateChildrenParent: true})
	astTestMust(t, err)
	temporary, err := NewSMTTempEnode(m)
	astTestMust(t, err)
	key, err := temporary.Set(declaration, []*SMTEnode{bn, an})
	astTestMust(t, err)
	yes, commuted, err := SMTEnodesCongruent(concrete, key)
	astTestMust(t, err)
	if !yes || !commuted {
		t.Fatal("temporary commutative key", yes, commuted)
	}
	key.SetFunctionDeclID(17)
	astTestMust(t, temporary.Reset())
	if key.FunctionDeclID() != math.MaxUint32 {
		t.Fatal("temporary reset")
	}
	concrete.Labels().Insert(3)
	concrete.PropagatedLabels().Insert(5)
	var display bytes.Buffer
	astTestMust(t, concrete.DisplayLabels(&display))
	if !strings.Contains(display.String(), "lbls: {3}") || !strings.Contains(display.String(), "plbls: {5}") {
		t.Fatal("label display", display.String())
	}
	astTestMust(t, concrete.CheckInvariant())
	astTestMust(t, concrete.Delete(true))
	if an.NumParents() != 0 || bn.NumParents() != 0 {
		t.Fatal("deletion did not pop child parent lists")
	}
	if err := concrete.CheckInvariant(); err == nil {
		t.Fatal("deleted enode remained valid")
	}
}

func TestSMTEnodeEqualityClassInvariantAndTransitivityCycleGuard(t *testing.T) {
	m := astTestManager(t)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	_, root := smtEnodeConstant(t, m, 2310, boolean, 4)
	_, member := smtEnodeConstant(t, m, 2311, boolean, 1)
	root.root, root.next, root.classSize = root, member, 2
	member.root, member.next = root, root
	member.trans.Target = root
	astTestMust(t, root.CheckInvariant())
	astTestMust(t, member.CheckInvariant())
	member.trans.Target = member
	if member.TransReaches(root) || member.CheckInvariant() == nil {
		t.Fatal("cyclic transitivity chain accepted")
	}
}
