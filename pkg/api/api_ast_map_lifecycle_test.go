package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPublicASTMapLifecycleAndOwnership(t *testing.T) {
	c, manager, mapping := newPublicASTMapFixture(t)
	key := publicASTMapAtom(t, manager, "public-map-lifecycle-key")
	value := publicASTMapAtom(t, manager, "public-map-lifecycle-value")
	astTestMust(t, mapping.Insert(key, value))
	astTestMust(t, mapping.Close())
	astTestMust(t, mapping.Close())
	if _, err := mapping.Size(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("closed map operation", err)
	}
	if _, err := manager.GetKind(key); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("close retained key", err)
	}
	if _, err := manager.GetKind(value); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("close retained value", err)
	}
	again, err := c.NewASTMap()
	if err != nil {
		t.Fatal("map close damaged context", err)
	}
	astTestMust(t, again.Close())
}
