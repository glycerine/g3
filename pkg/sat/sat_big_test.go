package sat

// Source: src/sat/sat_big.cpp:72-226.

import (
	"testing"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

func TestSATBIGExternalGraphDFSIntervalsAndRoots(t *testing.T) {
	b := newSATBIG(z3rand.New(0))
	satOK(t, b.initAddingEdges(4, false))
	chain := []Literal{MakeLiteral(0, false), MakeLiteral(1, false), MakeLiteral(2, false), MakeLiteral(3, false)}
	for i := 0; i+1 < len(chain); i++ {
		satOK(t, b.addSolverEdge(chain[i], chain[i+1]))
	}
	satOK(t, b.doneAddingEdges())
	for i, u := range chain {
		if b.getLeft(u) >= b.getRight(u) {
			t.Fatal("invalid DFS interval", u, b.getLeft(u), b.getRight(u))
		}
		root, err := b.getRoot(u)
		satOK(t, err)
		if root != chain[0] {
			t.Fatal("root", i, root)
		}
		if i != 0 && b.getParent(u) != chain[i-1] {
			t.Fatal("parent", i, b.getParent(u))
		}
	}
	if !b.reaches(chain[0], chain[3]) || !b.connected(chain[0], chain[3]) || b.reaches(chain[3], chain[0]) {
		t.Fatal("reachability intervals")
	}
}

func TestSATBIGSafeReachObservesScheduledDeletion(t *testing.T) {
	b := newSATBIG(z3rand.New(0))
	satOK(t, b.initAddingEdges(3, false))
	a, c, d := MakeLiteral(0, false), MakeLiteral(1, false), MakeLiteral(2, false)
	satOK(t, b.addSolverEdge(a, c))
	satOK(t, b.addSolverEdge(c, d))
	satOK(t, b.doneAddingEdges())
	yes, err := b.safeReach(a, d)
	satOK(t, err)
	if !yes {
		t.Fatal("tree path should be safe")
	}
	b.deleted = make([][]Literal, b.numLiterals())
	b.addDeleted(a.Negated(), c)
	yes, err = b.safeReach(a, d)
	satOK(t, err)
	if yes {
		t.Fatal("scheduled clause deletion must invalidate path")
	}
}
