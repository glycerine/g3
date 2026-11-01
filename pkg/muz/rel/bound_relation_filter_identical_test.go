package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoundRelationFilterIdenticalBuildsSourceEqualityClass(t *testing.T) {
	manager := astTestManager(t)
	family, err := manager.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	relation := NewBoundRelation(3, false)
	if err = relation.FilterIdentical([]uint32{0, 2, 1}); err != nil {
		t.Fatal(err)
	}
	formula, err := relation.ToFormula(manager, []ast.ASTHandle{integer, integer, integer})
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.DelRef(&formula)) }()
	root, err := manager.RefGet(formula)
	astTestMust(t, err)
	if kind, err := manager.GetDeclKind(root); err != nil || kind != ast.ASTOpAnd {
		t.Fatal("identical-column formula root", kind, err)
	}
	count, err := manager.GetNumArgs(root)
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("identical-column equalities", count)
	}
	for i := uint32(0); i < count; i++ {
		equality, err := manager.GetArg(root, i)
		astTestMust(t, err)
		if kind, err := manager.GetDeclKind(equality); err != nil || kind != ast.ASTOpEq {
			t.Fatal("column equality", i, kind, err)
		}
	}
	if err = relation.FilterIdentical([]uint32{0, 3}); err == nil {
		t.Fatal("out-of-range identity column accepted")
	}
	if err = relation.FilterIdentical([]uint32{0, 1}); err != nil {
		t.Fatal("failed validation mutated relation", err)
	}
}

func TestBoundRelationFilterIdenticalPreservesEmptyAndTrivialCases(t *testing.T) {
	for _, columns := range [][]uint32{nil, {1}, {0, 2}} {
		relation := NewBoundRelation(3, true)
		if err := relation.FilterIdentical(columns); err != nil {
			t.Fatal(columns, err)
		}
		if !relation.Empty() {
			t.Fatal("identity filter changed empty relation", columns)
		}
	}
}

func TestBoundRelationFilterEqualPreservesCoarseBounds(t *testing.T) {
	relation := NewBoundRelation(3, false)
	if err := relation.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	beforeStrict, beforeWeak, err := relation.Bounds(0)
	if err != nil {
		t.Fatal(err)
	}
	relation.FilterEqual(Element(17), 99)
	afterStrict, afterWeak, err := relation.Bounds(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeStrict) != len(afterStrict) || len(beforeWeak) != len(afterWeak) {
		t.Fatal("source filter_equal changed bound cardinalities", beforeStrict, beforeWeak, afterStrict, afterWeak)
	}
	for i := range beforeStrict {
		if beforeStrict[i] != afterStrict[i] {
			t.Fatal("source filter_equal changed strict bounds", beforeStrict, afterStrict)
		}
	}
	for i := range beforeWeak {
		if beforeWeak[i] != afterWeak[i] {
			t.Fatal("source filter_equal changed weak bounds", beforeWeak, afterWeak)
		}
	}
}
