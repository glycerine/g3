package api

import "testing"

func TestSMTContextMutuallyRecursiveDatatypeDefinitions(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sorts, err := f.c.DatatypeSorts([]APIDatatypeDefinition{
		{Name: "api-even", Constructors: []APIDatatypeConstructor{
			{Name: "even-zero", Recognizer: "is-even-zero"},
			{Name: "even-next", Recognizer: "is-even-next", Accessors: []APIDatatypeAccessor{{Name: "even-predecessor", Recursive: true, RecursiveIndex: 1}}},
		}},
		{Name: "api-odd", Constructors: []APIDatatypeConstructor{
			{Name: "odd-next", Recognizer: "is-odd-next", Accessors: []APIDatatypeAccessor{{Name: "odd-predecessor", Recursive: true, RecursiveIndex: 0}}},
		}},
	})
	astTestMust(t, err)
	if len(sorts) != 2 {
		t.Fatal("recursive sort count", len(sorts))
	}
	for i := range sorts {
		sorts[i] = f.pin(sorts[i], nil)
		if recursive, e := f.c.DatatypeIsRecursive(sorts[i]); e != nil || !recursive {
			t.Fatal("recursive sort", i, recursive, e)
		}
	}
	evenZero := f.pin(f.c.DatatypeConstructor(sorts[0], 0))
	evenNext := f.pin(f.c.DatatypeConstructor(sorts[0], 1))
	oddNext := f.pin(f.c.DatatypeConstructor(sorts[1], 0))
	zero := f.pin(f.c.Apply(evenZero))
	odd := f.pin(f.c.Apply(oddNext, zero))
	even := f.pin(f.c.Apply(evenNext, odd))
	if sort := f.pin(f.c.SortOf(even)); sort != sorts[0] {
		t.Fatal("mutually recursive value", sort, sorts[0])
	}
	if _, err := f.c.DatatypeAccessor(sorts[0], 1, 1); err == nil {
		t.Fatal("out-of-bounds recursive accessor accepted")
	}
}
