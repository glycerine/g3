package fp

import (
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
	// An out-of-range cache mode is accepted unchecked, as z3 casts it
	// (AUDIT muz_fp_04-F4); the update still rebuilds the context.
	_, err = e.NumLevels(query)
	must(err)
	unchecked := e.pdr
	if unchecked == nil || unchecked == prior {
		t.Fatal("parameter update did not rebuild the pre-query context")
	}
	if _, err := prior.NumLevels(query); err == nil {
		t.Fatal("parameter update left the old pre-query context open")
	}
	must(e.params.SetUIntName(&parameters, &name, 2))
	must(e.SetQueryParameters(&parameters))
	levels, err := e.NumLevels(query)
	must(err)
	if levels != 0 || e.pdr == prior || e.pdr == unchecked {
		t.Fatal("second parameter update did not rebuild a fresh context", levels)
	}
}
