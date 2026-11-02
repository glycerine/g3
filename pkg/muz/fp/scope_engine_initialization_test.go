package fp

import (
	"strings"
	"testing"
)

func TestFixedpointPopRejectsInstantiatedPrequeryEngine(t *testing.T) {
	for _, backend := range []string{"spacer"} {
		t.Run(backend, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			f := newPDRLifecycleFixture(t)
			e := f.engine
			must(e.SelectEngine(backend))
			must(e.Push())
			must(e.Pop())
			must(e.Push())
			_, err := e.NumLevels(f.declarations["u"])
			must(err)
			if e.queried {
				t.Fatal("fixture unexpectedly ran a query")
			}
			err = e.Pop()
			if err == nil || !strings.Contains(err.Error(), "pop operation is only supported by duality engine") {
				t.Fatal("instantiated engine allowed prequery pop", err)
			}
			if len(e.scopes) != 1 {
				t.Fatal("rejected pop consumed its scope")
			}
		})
	}
}
