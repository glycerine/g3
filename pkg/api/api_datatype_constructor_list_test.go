package api

import "testing"

func TestAPIDatatypeConstructorListIsNonOwning(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	constructor, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-list-owner", Recognizer: "is-api-list-owner"})
	astTestMust(t, err)
	first, err := f.c.NewDatatypeConstructorList(constructor)
	astTestMust(t, err)
	astTestMust(t, first.Close())
	second, err := f.c.NewDatatypeConstructorList(constructor)
	astTestMust(t, err)
	sorts, err := f.c.DatatypeSortsFromConstructorLists([]string{"api-list-owner-sort"}, []*APIDatatypeConstructorList{second})
	astTestMust(t, err)
	if len(sorts) != 1 || sorts[0].IsNull() {
		t.Fatal("reused constructor", sorts)
	}
	astTestMust(t, second.Close())
	if _, err = constructor.Query(); err != nil {
		t.Fatal("list close retired constructor", err)
	}
	astTestMust(t, constructor.Close())
}
