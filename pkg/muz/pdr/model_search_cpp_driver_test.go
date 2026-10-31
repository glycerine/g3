package pdr

// Source: src/test/pdr.cpp, seeded model-search tree and backtracking driver.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestModelSearchSourceRandomTrees(t *testing.T) {
	manager, owner, transformer, _, closeBase := closureFixture(t)
	search := NewModelSearch(true)
	var stateRefs []ast.ASTRef
	defer func() {
		closureMust(t, search.Close())
		for i := range stateRefs {
			closureMust(t, manager.DelRef(&stateRefs[i]))
		}
		closureMust(t, transformer.Close())
		closureMust(t, owner.Close())
		closeBase()
	}()
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	states := make([]ast.ASTHandle, 10)
	for i := range states {
		name, err := manager.Symbols.Intern(fmt.Sprintf("source_pdr_s%d", i))
		closureMust(t, err)
		states[i], err = manager.MkConst(name, boolean)
		closureMust(t, err)
		ref, err := manager.NewRef(states[i])
		closureMust(t, err)
		stateRefs = append(stateRefs, ref)
	}
	random := z3rand.New(10)
	nextState := func() ast.ASTHandle { return states[random.Bounded(uint32(len(states)))] }
	var addTree func(*ModelNode, bool)
	addTree = func(parent *ModelNode, force bool) {
		closureMust(t, search.AddLeaf(parent))
		if parent.Level() > 0 && (force || parent.IsGoal()) {
			closureMust(t, search.removeGoal(parent))
			for i := 0; i < 2; i++ {
				child, err := NewModelNode(parent, nextState(), transformer, parent.Level()-1)
				closureMust(t, err)
				addTree(child, false)
			}
			closureMust(t, parent.CheckPreClosed())
		}
	}
	for round := 0; round < 100; round++ {
		root, err := NewModelNode(nil, nextState(), transformer, 4)
		closureMust(t, err)
		closureMust(t, search.SetRoot(root))
		addTree(root, false)
		// Bound this stress test: the first source-seeded tree did not finish
		// within 100,000 mutations on the Go implementation.
		for step := 0; step < 500; step++ {
			if err := search.WellFormed(); err != nil {
				t.Fatalf("round %d step %d: %v", round, step, err)
			}
			leaf, found, err := search.Next()
			closureMust(t, err)
			if !found {
				break
			}
			if leaf.Level() == 0 {
				if random.Bounded(2) == 0 {
					closureMust(t, search.BacktrackLevel(false, leaf.Parent()))
				} else {
					closureMust(t, search.BacktrackLevel(false, leaf))
				}
			} else {
				addTree(leaf, true)
			}
		}
		closureMust(t, search.Reset())
	}
}
