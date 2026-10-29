package api

import "testing"

func TestPublicASTMapAcceptsAllASTKinds(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	sort, err := manager.MkUninterpretedSort(astTestSymbol(t, manager, "public-map-sort"), nil)
	astTestMust(t, err)
	constant, err := manager.MkConst(astTestSymbol(t, manager, "public-map-constant"), sort)
	astTestMust(t, err)
	declaration, err := manager.GetDecl(constant)
	astTestMust(t, err)
	astTestMust(t, mapping.Insert(sort, declaration))
	if got, err := mapping.Find(sort); err != nil || got != declaration {
		t.Fatal("sort-to-declaration entry", got, err)
	}
	astTestMust(t, mapping.Insert(declaration, constant))
	if size, err := mapping.Size(); err != nil || size != 2 {
		t.Fatal("mixed-kind size", size, err)
	}
}
