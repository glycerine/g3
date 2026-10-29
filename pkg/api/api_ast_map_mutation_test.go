package api

import "testing"

func TestPublicASTMapInsertReplaceEraseAndReset(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	key := publicASTMapAtom(t, manager, "public-map-key")
	first := publicASTMapAtom(t, manager, "public-map-first")
	second := publicASTMapAtom(t, manager, "public-map-second")
	keyRef, err := manager.NewRef(key)
	astTestMust(t, err)
	firstRef, err := manager.NewRef(first)
	astTestMust(t, err)
	secondRef, err := manager.NewRef(second)
	astTestMust(t, err)
	defer func() {
		astTestMust(t, manager.DelRef(&secondRef))
		astTestMust(t, manager.DelRef(&firstRef))
		astTestMust(t, manager.DelRef(&keyRef))
	}()

	astTestMust(t, mapping.Insert(key, first))
	if got, err := mapping.Find(key); err != nil || got != first {
		t.Fatal("initial lookup", got, err)
	}
	astTestMust(t, mapping.Insert(key, second))
	if got, err := mapping.Find(key); err != nil || got != second {
		t.Fatal("replacement lookup", got, err)
	}
	astTestMust(t, mapping.Erase(key))
	if size, err := mapping.Size(); err != nil || size != 0 {
		t.Fatal("erase size", size, err)
	}
	astTestMust(t, mapping.Erase(key))
	astTestMust(t, mapping.Insert(key, first))
	astTestMust(t, mapping.Reset())
	if size, err := mapping.Size(); err != nil || size != 0 {
		t.Fatal("reset size", size, err)
	}
}
