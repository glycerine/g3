package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDDNFEngineCloseRetriesUnreleasedTrailAfterClosed(t *testing.T) {
	engine, manager := newFragmentTestEngine(t, "ddnf")
	truth, err := manager.MkTrue()
	fragmentMust(t, err)
	reference, err := manager.NewRef(truth)
	fragmentMust(t, err)
	stale := reference
	fragmentMust(t, manager.DelRef(&reference))
	engine.ddnf.trail = append(engine.ddnf.trail, stale)

	firstErr := engine.Close()
	if !errors.Is(firstErr, ast.ErrASTHandle) {
		t.Fatal("initial DDNF trail release", firstErr)
	}
	if !engine.closed || len(engine.ddnf.trail) != 1 {
		t.Fatal("failed trail release was discarded", engine.closed, engine.ddnf.trail)
	}
	secondErr := engine.Close()
	if !errors.Is(secondErr, ast.ErrASTHandle) || len(engine.ddnf.trail) != 1 {
		t.Fatal("closed engine did not retry the outstanding DDNF release", secondErr, engine.ddnf.trail)
	}

	// The deliberately stale test reference cannot be repaired. Remove this
	// injected corruption so the fixture's deferred idempotent close succeeds.
	engine.ddnf.trail = nil
	if err := engine.Close(); err != nil {
		t.Fatal("close after removing injected stale reference", err)
	}
}
