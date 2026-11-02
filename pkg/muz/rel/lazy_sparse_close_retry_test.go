package rel

import (
	"errors"
	"testing"
)

func TestLazySparseEngineCloseStopsAndRetriesPreservedNode(t *testing.T) {
	engine := NewLazySparseEngine()
	base, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := base.Project(nil)
	if err != nil {
		t.Fatal(err)
	}
	child := derived.root.left
	child.references = 0 // simulate a child cleanup boundary refusing consumption
	if err = engine.Close(); !errors.Is(err, ErrLazySparse) {
		t.Fatal("engine close did not report staged refusal", err)
	}
	if derived.closed || derived.root == nil || derived.root.left != child || derived.root.references != 1 || len(engine.handles) != 2 {
		t.Fatal("refused close consumed handle, node, or child owner")
	}
	child.references = 2 // base handle plus the preserved derived edge
	if err = engine.Close(); err != nil {
		t.Fatal("retry", err)
	}
	if !derived.closed || !base.closed || len(engine.handles) != 0 || !engine.closed {
		t.Fatal("retry did not finish engine cleanup")
	}
}
