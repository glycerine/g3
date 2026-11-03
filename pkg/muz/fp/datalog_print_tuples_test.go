package fp

import (
	"errors"
	"testing"
)

func TestDatalogPrintTuplesParametersAndGuard(t *testing.T) {
	for _, tc := range []struct {
		name, module, local string
		serviceFree, want   bool
	}{
		{name: "service-free", serviceFree: true, want: true},
		{name: "source-default", want: true},
		{name: "module-disabled", module: "false"},
		{name: "module-enabled", module: "true", want: true},
		{name: "local-disabled", module: "true", local: "false"},
		{name: "local-enabled", module: "false", local: "true", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			m, globals := newPDRLifecycleServices(t)
			if tc.module != "" {
				must(globals.SourceSetName("fixedpoint.datalog.print.tuples", tc.module))
			}
			var engine *PropositionalEngine
			var err error
			if tc.serviceFree {
				engine, err = NewPropositionalEngine(m)
			} else {
				engine, err = NewPropositionalEngineWithServices(m, globals.Context, globals)
			}
			must(err)
			t.Cleanup(func() { must(engine.Close()) })
			if tc.local != "" {
				local := globals.Context.EmptyValue()
				key := "datalog.print.tuples"
				must(globals.Context.SetBoolName(&local, &key, tc.local == "true"))
				must(engine.SetQueryParameters(&local))
				must(globals.Context.Del(&local))
			}
			for attempt := 0; attempt < 2; attempt++ {
				got, err := engine.DatalogPrintTuples()
				must(err)
				if got != tc.want {
					t.Fatalf("print output tuples = %v, want %v", got, tc.want)
				}
			}
			if tc.local != "" {
				must(engine.SetQueryParameters(nil))
				got, err := engine.DatalogPrintTuples()
				must(err)
				if got != (tc.module != "false") {
					t.Fatal("cleared local override did not restore module setting", got)
				}
			}
			must(engine.Close())
			if _, err := engine.DatalogPrintTuples(); !errors.Is(err, ErrPropositionalEngine) {
				t.Fatal("closed engine accepted output-tuple lookup", err)
			}
		})
	}
}
