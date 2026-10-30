package api

import "testing"

func TestAPIOptimizeParameterDescriptions(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	descriptions, err := f.opt.ParameterDescriptions()
	astTestMust(t, err)
	size, err := descriptions.Size()
	astTestMust(t, err)
	if size < 20 {
		t.Fatal("short optimize catalog", size)
	}
	kind, err := descriptions.Kind("priority")
	astTestMust(t, err)
	if kind != ParameterSymbol {
		t.Fatal("priority kind", kind)
	}
	documentation, err := descriptions.Documentation("maxsat_engine")
	astTestMust(t, err)
	if documentation == "" {
		t.Fatal("empty maxsat documentation")
	}
	astTestMust(t, descriptions.Close())
}
