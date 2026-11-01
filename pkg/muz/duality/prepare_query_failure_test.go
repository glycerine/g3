package duality_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
)

func TestPrepareQueryFailureReleasesClauseOwners(t *testing.T) {
	ctx, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := ctx.Close(); e != nil {
			t.Error(e)
		}
	}()
	m, err := ctx.Manager()
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	decl := func(name string) ast.ASTHandle {
		t.Helper()
		symbol, e := m.Symbols.Intern(name)
		if e != nil {
			t.Fatal(e)
		}
		h, e := m.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	p, missing := decl("duality-frontier-p"), decl("duality-frontier-missing")
	pApp, err := m.MkApp(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	truth, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	rule, err := m.MkImplies(truth, pApp)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []ast.ASTHandle{p, missing, pApp, rule}
	owners := make([]ast.ASTRef, len(inputs))
	for i, input := range inputs {
		owners[i], err = m.NewRef(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for i := len(owners) - 1; i >= 0; i-- {
			if e := m.DelRef(&owners[i]); e != nil {
				t.Error(e)
			}
		}
	}()
	baseline := make([]uint32, len(inputs))
	for i, input := range inputs {
		baseline[i], err = m.GetRefCount(input)
		if err != nil {
			t.Fatal(err)
		}
	}

	prepared, prepareErr := duality.PrepareQuery(m, []ast.ASTHandle{rule}, []uint32{0}, pApp, []ast.ASTHandle{p, ast.ASTHandle{}}, 4)
	if prepareErr == nil || prepared != nil {
		t.Fatalf("invalid pinned predicate returned prepared=%v err=%v", prepared, prepareErr)
	}
	for i, input := range inputs {
		got, e := m.GetRefCount(input)
		if e != nil || got != baseline[i] {
			t.Fatalf("failure cleanup input %d refcount=%d err=%v, want %d", i, got, e, baseline[i])
		}
	}
}
