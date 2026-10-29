package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestPublicASTMapKeysOwnIndependentRoots(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	key := publicASTMapAtom(t, manager, "public-map-owned-key")
	value := publicASTMapAtom(t, manager, "public-map-owned-value")
	astTestMust(t, mapping.Insert(key, value))
	keys, err := mapping.Keys()
	astTestMust(t, err)
	if n, err := keys.Len(); err != nil || n != 1 {
		t.Fatal("key count", n, err)
	}
	if _, err := keys.At(1); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("key bounds", err)
	}
	got, err := keys.At(0)
	astTestMust(t, err)
	if got != key {
		t.Fatal("key changed", got, key)
	}
	astTestMust(t, mapping.Reset())
	if _, err := manager.GetKind(value); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("map value survived reset", err)
	}
	if _, err := manager.GetKind(got); err != nil {
		t.Fatal("key snapshot borrowed map root", err)
	}
	astTestMust(t, keys.Close())
	if _, err := manager.GetKind(got); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("key snapshot failed to release root", err)
	}
}

func TestPublicASTMapKeysValuesSnapshot(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	for _, name := range []string{"keys-a", "keys-b", "keys-c"} {
		key := publicASTMapAtom(t, manager, name)
		astTestMust(t, mapping.Insert(key, key))
	}
	keys, err := mapping.Keys()
	astTestMust(t, err)
	defer keys.Close()
	values, err := keys.Values()
	astTestMust(t, err)
	if len(values) != 3 {
		t.Fatal("snapshot length", len(values))
	}
}
