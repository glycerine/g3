package fp

import (
	"strings"
	"testing"
)

func TestPDRPrequeryContextRebuildsForParameterUpdates(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e := f.engine
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	query := f.declarations["u"]
	_, err := e.NumLevels(query)
	must(err)
	prior := e.pdr
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	name := "pdr.cache_mode"
	must(e.params.SetUIntName(&parameters, &name, 99))
	must(e.SetQueryParameters(&parameters))
	_, err = e.NumLevels(query)
	if err == nil || !strings.Contains(err.Error(), "invalid pdr.cache_mode 99") {
		t.Fatal("pre-query context hid changed constructor parameters", err)
	}
	if e.pdr != nil {
		t.Fatal("failed parameterized construction published a context")
	}
	if _, err := prior.NumLevels(query); err == nil {
		t.Fatal("parameter update left the old pre-query context open")
	}
	must(e.params.SetUIntName(&parameters, &name, 2))
	must(e.SetQueryParameters(&parameters))
	levels, err := e.NumLevels(query)
	must(err)
	if levels != 0 || e.pdr == prior {
		t.Fatal("valid parameter retry did not rebuild a fresh context", levels)
	}
}
