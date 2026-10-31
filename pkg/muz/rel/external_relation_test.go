package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExternalRelationCallbacksAndClone(t *testing.T) {
	f := newTableRelationFixture(t)
	rows := make(map[ast.ASTHandle]map[ast.ASTHandle]bool)
	callbacks := ExternalRelationCallbacks{}
	callbacks.ReduceAssign = func(decl ast.ASTHandle, args, outputs []ast.ASTHandle) error {
		op, err := f.ast.GetDeclKind(decl)
		if err != nil {
			return err
		}
		switch op {
		case ast.DatalogOpEmpty:
			rows[outputs[0]] = make(map[ast.ASTHandle]bool)
		case ast.DatalogOpStore:
			rows[args[0]][args[1]] = true
		case ast.DatalogOpClone:
			rows[outputs[0]] = make(map[ast.ASTHandle]bool)
			for value := range rows[args[0]] {
				rows[outputs[0]][value] = true
			}
		default:
			t.Fatalf("unexpected assignment operation %d", op)
		}
		return nil
	}
	callbacks.Reduce = func(decl ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
		op, err := f.ast.GetDeclKind(decl)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		switch op {
		case ast.DatalogOpIsEmpty:
			return f.ast.MkBoolVal(len(rows[args[0]]) == 0)
		case ast.DatalogOpSelect:
			return f.ast.MkBoolVal(rows[args[0]][args[1]])
		default:
			return ast.ASTHandle{}, nil
		}
	}
	r, err := NewExternalRelation(f.relations, []ast.ASTHandle{f.boolean}, callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	if empty, err := r.Empty(); err != nil || !empty {
		t.Fatal("new relation", empty, err)
	}
	astTestMust(t, r.AddFact([]ast.ASTHandle{f.truth}))
	if empty, err := r.Empty(); err != nil || empty {
		t.Fatal("stored relation", empty, err)
	}
	if _, err := r.ContainsFact([]ast.ASTHandle{f.truth}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source select guard", err)
	}
	clone, err := r.Clone()
	astTestMust(t, err)
	defer func() { astTestMust(t, clone.Close()) }()
	astTestMust(t, clone.AddFact([]ast.ASTHandle{f.falsehood}))
	originalHandle, err := r.Relation()
	astTestMust(t, err)
	cloneHandle, err := clone.Relation()
	astTestMust(t, err)
	if rows[originalHandle][f.falsehood] || !rows[cloneHandle][f.falsehood] {
		t.Fatal("clone did not own independent callback state")
	}
	if err = r.AddFact(nil); err == nil {
		t.Fatal("wrong arity accepted")
	}
}

func TestExternalRelationFallbackAndFormula(t *testing.T) {
	f := newTableRelationFixture(t)
	r, err := NewExternalRelation(f.relations, []ast.ASTHandle{f.boolean}, ExternalRelationCallbacks{})
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	if empty, err := r.Empty(); err != nil || empty {
		t.Fatal("unreduced empty is not true", empty, err)
	}
	if _, err := r.ContainsFact([]ast.ASTHandle{f.truth}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("source select guard", err)
	}
	complement, err := r.Complement()
	astTestMust(t, err)
	defer func() { astTestMust(t, complement.Close()) }()
	formula, err := complement.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	expression, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	op, err := f.ast.GetDeclKind(expression)
	if err != nil || op != ast.DatalogOpComplement {
		t.Fatal("fallback complement formula", op, err)
	}
}

func TestExternalRelationProjectAndRename(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	var operations []int32
	callbacks := ExternalRelationCallbacks{Reduce: func(decl ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
		op, err := f.ast.GetDeclKind(decl)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		operations = append(operations, op)
		return ast.ASTHandle{}, nil
	}}
	r, err := NewExternalRelation(f.relations, []ast.ASTHandle{f.boolean, integer, real}, callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	projected, err := r.Project([]uint32{1})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	projectSorts, err := projected.Sorts()
	astTestMust(t, err)
	if len(projectSorts) != 2 || projectSorts[0] != f.boolean || projectSorts[1] != real {
		t.Fatal("projected signature", projectSorts)
	}
	renamed, err := r.Rename([]uint32{0, 1, 2})
	astTestMust(t, err)
	defer func() { astTestMust(t, renamed.Close()) }()
	renameSorts, err := renamed.Sorts()
	astTestMust(t, err)
	if len(renameSorts) != 3 || renameSorts[0] != integer || renameSorts[1] != real || renameSorts[2] != f.boolean {
		t.Fatal("renamed signature", renameSorts)
	}
	if len(operations) != 2 || operations[0] != ast.DatalogOpProject || operations[1] != ast.DatalogOpRename {
		t.Fatal("reduction operations", operations)
	}
	for _, invalid := range [][]uint32{nil, {1, 1}, {3}} {
		if _, err := r.Project(invalid); err == nil {
			t.Fatalf("accepted projection %v", invalid)
		}
	}
	if _, err := r.Rename([]uint32{0, 0}); err == nil {
		t.Fatal("accepted duplicate cycle")
	}
}
