package qe

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestExprQETreeGuardedLeavesComposeDefinitions(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	truth := f.own(m.MkTrue())
	falsity := f.own(m.MkFalse())
	x := f.constant("definition-x", f.boolean)
	y := f.constant("definition-y", f.boolean)
	branch := f.constant("definition-branch", f.boolean)
	guard := f.constant("definition-guard", f.boolean)
	root, err := newExprQETree(m, f.r.config, nil, truth)
	fmMust(t, err)
	defer func() { fmMust(t, root.Close()) }()
	fmMust(t, root.initialize(truth))
	fmMust(t, root.addDefinition(x, y))
	fmMust(t, root.consumeVariables([]ast.ASTHandle{branch}))
	fmMust(t, root.selectVariable(branch, big.NewRat(2, 1)))
	a, err := root.addBranch(f.r.config, big.NewRat(0, 1), truth)
	fmMust(t, err)
	b, err := root.addBranch(f.r.config, big.NewRat(1, 1), truth)
	fmMust(t, err)
	fmMust(t, a.initialize(guard))
	fmMust(t, a.addDefinition(y, truth))
	fmMust(t, b.initialize(falsity))
	result, err := root.guardedLeaves(f.r.config)
	fmMust(t, err)
	defer func() { fmMust(t, result.Close()) }()
	n, err := result.Size()
	fmMust(t, err)
	if n != 1 {
		t.Fatalf("got %d non-false leaves", n)
	}
	h, err := result.Guard(0)
	fmMust(t, err)
	if h != guard {
		t.Fatal("wrong guard")
	}
	n, err = result.DefinitionSize(0)
	fmMust(t, err)
	if n != 2 {
		t.Fatalf("got %d definitions", n)
	}
	for i, v := range []ast.ASTHandle{x, y} {
		decl, e := m.GetDecl(v)
		fmMust(t, e)
		got, e := result.Variable(0, uint32(i))
		fmMust(t, e)
		if got != decl {
			t.Fatal("definition declaration order", i)
		}
		value, e := result.Definition(0, uint32(i))
		fmMust(t, e)
		if value != truth {
			t.Fatal("definition was not normalized", i)
		}
	}
}
