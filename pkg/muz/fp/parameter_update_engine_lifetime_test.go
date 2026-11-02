package fp

import (
	"strings"
	"testing"
)

// Source: dl_context.cpp updt_params preserves m_engine; PDR/Spacer rebuild
// their solver contexts inside the same engine_base instance.
func TestParameterUpdatePreservesPrequeryEngineLifetime(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		for _, instantiate := range []bool{false, true} {
			name := backend + "/uninstantiated"
			if instantiate {
				name = backend + "/instantiated"
			}
			t.Run(name, func(t *testing.T) {
				f := newDirectFactFixture(t)
				e := f.engine
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				must(e.SelectEngine(backend))
				must(e.Push())
				if instantiate {
					_, err := e.NumLevels(f.declarations["p"])
					must(err)
				}
				value, err := e.params.NewValue()
				must(err)
				key := "pdr.max_num_contexts"
				must(e.params.SetUIntName(&value, &key, 37))
				must(e.SetQueryParameters(&value))
				must(e.params.Del(&value))
				must(e.SetQueryParameters(nil))
				if e.queried {
					t.Fatal("fixture unexpectedly queried")
				}
				err = e.Pop()
				if !instantiate {
					must(err)
					return
				}
				if err == nil || !strings.Contains(err.Error(), "pop operation is only supported by duality engine") {
					t.Fatal("parameter update forgot instantiated engine lifetime", err)
				}
				if len(e.scopes) != 1 {
					t.Fatal("refused pop consumed scope")
				}
				_, err = e.NumLevels(f.declarations["p"])
				must(err)
			})
		}
	}
}
