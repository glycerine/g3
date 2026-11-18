package ast

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// TestASTCloseReportsRefusedRootDeletion makes one root's deletion fail on
// every attempt: its stored hash no longer finds its table entry, so the
// table check (or erase) refuses it. Close must report that refusal instead
// of retrying the same roots forever. A numeral manager left with a pending
// temporary refused root deletions the same way and hung the qe
// allocation-frontier test for hours.
func TestASTCloseReportsRefusedRootDeletion(t *testing.T) {
	budget := arena.NewBudget(1 << 30)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	astTestMust(t, err)
	integers := unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: integers})
	astTestMust(t, err)
	m, err := NewASTManager(ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	h, err := m.MkFreshConst("k", boolean)
	astTestMust(t, err)
	r, err := m.internal(h)
	astTestMust(t, err)
	m.nodeAt(r.slot).hash ^= 0x5a5a5a5a

	done := make(chan error, 1)
	go func() { done <- m.Close() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Close succeeded although a root deletion was refused")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Close did not return: refused root deletions retried forever")
	}
}
