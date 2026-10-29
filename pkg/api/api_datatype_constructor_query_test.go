package api

import "testing"

func TestAPIDatatypeConstructorBuildAndQuery(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	none, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-none", Recognizer: "is-api-none"})
	astTestMust(t, err)
	some, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{
		Name: "api-some", Recognizer: "is-api-some",
		Accessors: []APIDatatypeAccessor{{Name: "api-value", Sort: boolean}},
	})
	astTestMust(t, err)
	sort, err := f.c.DatatypeSortFromConstructors("api-option", none, some)
	astTestMust(t, err)
	f.pin(sort, nil)
	query, err := some.Query()
	astTestMust(t, err)
	if query.Constructor.IsNull() || query.Recognizer.IsNull() || len(query.Accessors) != 1 {
		t.Fatal("constructor query", query)
	}
	value := f.pin(f.c.Bool(true))
	application := f.pin(f.c.Apply(query.Constructor, value))
	if got := f.pin(f.c.SortOf(application)); got != sort {
		t.Fatal("constructed value sort", got, sort)
	}
	astTestMust(t, some.Close())
	astTestMust(t, none.Close())
}
