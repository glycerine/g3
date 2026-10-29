package api

import (
	"errors"
	"testing"
)

func TestPublicASTMapContainsSizeAndMissingCategory(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	key := publicASTMapAtom(t, manager, "public-map-query-key")
	value := publicASTMapAtom(t, manager, "public-map-query-value")
	if found, err := mapping.Contains(key); err != nil || found {
		t.Fatal("empty contains", found, err)
	}
	if _, err := mapping.Find(key); !errors.Is(err, ErrASTMapMissing) {
		t.Fatal("missing lookup category", err)
	}
	astTestMust(t, mapping.Insert(key, value))
	if found, err := mapping.Contains(key); err != nil || !found {
		t.Fatal("inserted contains", found, err)
	}
	if size, err := mapping.Size(); err != nil || size != 1 {
		t.Fatal("inserted size", size, err)
	}
}
