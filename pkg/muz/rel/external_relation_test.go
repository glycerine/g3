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

func TestExternalRelationCachesAccessorDeclarations(t *testing.T) {
	f := newTableRelationFixture(t)
	seen := make(map[int32]ast.ASTHandle)
	check := func(decl ast.ASTHandle) error {
		kind, err := f.ast.GetDeclKind(decl)
		if err != nil {
			return err
		}
		if prior, ok := seen[kind]; ok && prior != decl {
			t.Fatalf("accessor declaration %d changed from %v to %v", kind, prior, decl)
		}
		seen[kind] = decl
		return nil
	}
	callbacks := ExternalRelationCallbacks{
		ReduceAssign: func(decl ast.ASTHandle, _, _ []ast.ASTHandle) error { return check(decl) },
		Reduce: func(decl ast.ASTHandle, _ []ast.ASTHandle) (ast.ASTHandle, error) {
			if err := check(decl); err != nil {
				return ast.ASTHandle{}, err
			}
			return ast.ASTHandle{}, nil
		},
	}
	r, err := NewExternalRelation(f.relations, []ast.ASTHandle{f.boolean}, callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	for i := 0; i < 2; i++ {
		astTestMust(t, r.AddFact([]ast.ASTHandle{f.truth}))
		if empty, err := r.Empty(); err != nil || empty {
			t.Fatal("empty", empty, err)
		}
	}
	for _, kind := range []int32{ast.DatalogOpEmpty, ast.DatalogOpStore, ast.DatalogOpIsEmpty} {
		if seen[kind] == (ast.ASTHandle{}) {
			t.Fatalf("accessor declaration %d was not exercised", kind)
		}
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

func TestExternalRelationCallbackTrailLifetime(t *testing.T) {
	f := newTableRelationFixture(t)
	callbacks := ExternalRelationCallbacks{ReduceAssign: func(ast.ASTHandle, []ast.ASTHandle, []ast.ASTHandle) error { return nil }}
	plugin, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)
	defer func() { astTestMust(t, plugin.Close()) }()
	r, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, r.Close()) }()
	value, err := f.ast.MkFreshConst("callback-value", f.boolean)
	astTestMust(t, err)
	owner, err := f.ast.NewRef(value)
	astTestMust(t, err)
	astTestMust(t, r.AddFact([]ast.ASTHandle{value}))
	astTestMust(t, f.ast.DelRef(&owner))
	if _, err = f.ast.GetKind(value); err != nil {
		t.Fatal("callback argument was not kept alive", err)
	}
	if err = plugin.Close(); err == nil {
		t.Fatal("closed plugin with live relation")
	}
	astTestMust(t, r.Close())
	if _, err = f.ast.GetKind(value); err != nil {
		t.Fatal("trail was released before plugin", err)
	}
	astTestMust(t, plugin.Close())
	if f.ast.IsLiveHandle(value) {
		t.Fatal("plugin close did not release callback argument")
	}
}

func TestExternalRelationReduceCallbackRetainsReturnedAST(t *testing.T) {
	f := newTableRelationFixture(t)
	var result ast.ASTHandle
	var resultRef ast.ASTRef
	callbacks := ExternalRelationCallbacks{Reduce: func(ast.ASTHandle, []ast.ASTHandle) (ast.ASTHandle, error) {
		var err error
		result, err = f.ast.MkFreshConst("callback-result", f.boolean)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		resultRef, err = f.ast.NewRef(result)
		return result, err
	}}
	plugin, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)
	relation, err := plugin.NewEmpty([]ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	if empty, err := relation.Empty(); err != nil || empty {
		t.Fatal("callback result was not used", empty, err)
	}
	astTestMust(t, f.ast.DelRef(&resultRef))
	if _, err := f.ast.GetKind(result); err != nil {
		t.Fatal("callback result was not retained by plugin trail", err)
	}
	astTestMust(t, relation.Close())
	if _, err := f.ast.GetKind(result); err != nil {
		t.Fatal("callback result released before plugin cleanup", err)
	}
	astTestMust(t, plugin.Close())
	if f.ast.IsLiveHandle(result) {
		t.Fatal("plugin cleanup did not release callback result")
	}
}
