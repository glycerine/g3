package api

import "testing"

func TestAPIDatatypeConstructorListsBuildMutualRecursion(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	evenZero, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-owner-even-zero", Recognizer: "is-api-owner-even-zero"})
	astTestMust(t, err)
	evenNext, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-owner-even-next", Recognizer: "is-api-owner-even-next", Accessors: []APIDatatypeAccessor{{Name: "api-owner-even-pred", Recursive: true, RecursiveIndex: 1}}})
	astTestMust(t, err)
	oddNext, err := f.c.NewDatatypeConstructorObject(APIDatatypeConstructor{Name: "api-owner-odd-next", Recognizer: "is-api-owner-odd-next", Accessors: []APIDatatypeAccessor{{Name: "api-owner-odd-pred", Recursive: true, RecursiveIndex: 0}}})
	astTestMust(t, err)
	evens, err := f.c.NewDatatypeConstructorList(evenZero, evenNext)
	astTestMust(t, err)
	odds, err := f.c.NewDatatypeConstructorList(oddNext)
	astTestMust(t, err)
	sorts, err := f.c.DatatypeSortsFromConstructorLists([]string{"api-owner-even", "api-owner-odd"}, []*APIDatatypeConstructorList{evens, odds})
	astTestMust(t, err)
	if len(sorts) != 2 {
		t.Fatal("sort count", len(sorts))
	}
	for _, sort := range sorts {
		f.pin(sort, nil)
	}
	evenQuery, err := evenNext.Query()
	astTestMust(t, err)
	oddQuery, err := oddNext.Query()
	astTestMust(t, err)
	zeroQuery, err := evenZero.Query()
	astTestMust(t, err)
	zero := f.pin(f.c.Apply(zeroQuery.Constructor))
	odd := f.pin(f.c.Apply(oddQuery.Constructor, zero))
	even := f.pin(f.c.Apply(evenQuery.Constructor, odd))
	if sort := f.pin(f.c.SortOf(even)); sort != sorts[0] {
		t.Fatal("recursive result sort", sort, sorts[0])
	}
	astTestMust(t, odds.Close())
	astTestMust(t, evens.Close())
	astTestMust(t, oddNext.Close())
	astTestMust(t, evenNext.Close())
	astTestMust(t, evenZero.Close())
}
