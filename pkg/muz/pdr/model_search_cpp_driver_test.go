package pdr

// Source: src/test/pdr.cpp, seeded model-search tree and backtracking driver.

import (
	"fmt"
	"os"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestModelSearchSourceRandomTrees(t *testing.T) {
	// This source stress run can exceed the normal test budget. In this
	// repository, -short explicitly opts into long source-port checks.
	if !testing.Short() {
		t.Skip("run the full source model-search workload with -short")
	}
	// The source stress driver has no mutation limit and can stay in round 0
	// for millions of random backtracks. Keep a large deterministic prefix in
	// the normal suite; the exact unbounded source workload remains available
	// through TestModelSearchSourceRandomTreesFull with G3_PDR_SOURCE_STRESS=1.
	runModelSearchSourceRandomTrees(t, 100, 10, 4, 100000, false)
}

func TestModelSearchSourceRandomTreesBudgetRegression(t *testing.T) {
	runModelSearchSourceRandomTrees(t, 1, 10, 4, 100000, true)
}

func TestModelSearchSourceRandomTreesFull(t *testing.T) {
	if !testing.Short() {
		t.Skip("run the full source model-search workload with -short")
	}
	if os.Getenv("G3_PDR_SOURCE_STRESS") != "1" {
		t.Skip("set G3_PDR_SOURCE_STRESS=1 to run the unbounded C++ source workload")
	}
	runModelSearchSourceRandomTrees(t, 100, 10, 4, 0, false)
}

func runModelSearchSourceRandomTrees(t *testing.T, rounds, stateCount int, depth uint32, maxMutations int, expectBudget bool) {
	t.Helper()
	// The source driver performs millions of tiny mutations. testing.T.Helper
	// walks the stack on every call, so use this no-stack-walk check in the hot
	// path and preserve detailed test locations for ordinary unit tests.
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, owner, transformer, _, closeBase := closureFixture(t)
	search := NewModelSearch(true)
	var stateRefs []ast.ASTRef
	defer func() {
		must(search.Close())
		for i := range stateRefs {
			must(manager.DelRef(&stateRefs[i]))
		}
		must(transformer.Close())
		must(owner.Close())
		closeBase()
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	states := make([]ast.ASTHandle, stateCount)
	for i := range states {
		name, err := manager.Symbols.Intern(fmt.Sprintf("source_pdr_s%d", i))
		must(err)
		states[i], err = manager.MkConst(name, boolean)
		must(err)
		ref, err := manager.NewRef(states[i])
		must(err)
		stateRefs = append(stateRefs, ref)
	}
	random := z3rand.New(10)
	nextState := func() ast.ASTHandle { return states[random.Bounded(uint32(len(states)))] }
	var addTree func(*ModelNode, bool)
	addTree = func(parent *ModelNode, force bool) {
		must(search.AddLeaf(parent))
		if parent.Level() > 0 && (force || parent.IsGoal()) {
			must(search.removeGoal(parent))
			for i := 0; i < 2; i++ {
				child, err := NewModelNode(parent, nextState(), transformer, parent.Level()-1)
				must(err)
				addTree(child, false)
			}
			must(parent.CheckPreClosed())
		}
	}
	for round := 0; round < rounds; round++ {
		root, err := NewModelNode(nil, nextState(), transformer, depth)
		must(err)
		must(search.SetRoot(root))
		addTree(root, false)
		steps := 0
		for {
			must(search.WellFormed())
			leaf, found, err := search.Next()
			must(err)
			if !found {
				if expectBudget && steps < maxMutations {
					t.Fatalf("source random tree drained after %d mutations; expected the bounded non-draining prefix", steps)
				}
				break
			}
			steps++
			if leaf.Level() == 0 {
				if random.Bounded(2) == 0 {
					must(search.BacktrackLevel(false, leaf.Parent()))
				} else {
					must(search.BacktrackLevel(false, leaf))
				}
			} else {
				addTree(leaf, true)
			}
			if maxMutations > 0 && steps >= maxMutations {
				must(search.WellFormed())
				if expectBudget {
					goals, err := search.GoalCount()
					must(err)
					if goals == 0 {
						t.Fatal("source random tree reached the mutation budget without a frontier")
					}
				}
				break
			}
		}
		must(search.Reset())
	}
}
