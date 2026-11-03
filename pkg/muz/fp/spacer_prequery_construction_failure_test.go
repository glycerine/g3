package fp

// Source: spacer_dl_interface construction and its pre-query get_num_levels
// consumer. Failed parameter construction must release the unpublished owner.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestSpacerPrequeryConstructionAllocationFailureAndRetry(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprintf("local=%t", local), func(t *testing.T) {
			failures, completed := 0, false
			for stop := int64(0); stop < 256 && !completed; stop++ {
				t.Run(fmt.Sprint(stop), func(t *testing.T) {
					var budget *arena.Budget
					t.Cleanup(func() {
						if budget != nil && budget.Used() != 0 {
							t.Error("pre-query construction fixture leaked bytes", budget.Used())
						}
					})
					f := newPDRLifecycleFixture(t)
					e, m := f.engine, f.manager
					budget = m.Budget()
					must := func(err error) { t.Helper(); fragmentMust(t, err) }
					must(e.SelectEngine("spacer"))
					if local {
						parameters, err := e.params.NewValue()
						must(err)
						t.Cleanup(func() { must(e.params.Del(&parameters)) })
						seed, equality, mbqi := "spacer.random_seed", "spacer.eq_prop", "spacer.mbqi"
						must(e.params.SetUIntName(&parameters, &seed, 17))
						must(e.params.SetBoolName(&parameters, &equality, false))
						must(e.params.SetBoolName(&parameters, &mbqi, false))
						must(e.SetQueryParameters(&parameters))
					}
					before, references := e.params.Stats(), m.Stats().LiveReferences
					checkOwners := func() {
						t.Helper()
						after := e.params.Stats()
						if after.LiveBlocks != before.LiveBlocks || after.References != before.References || after.Entries != before.Entries || m.Stats().LiveReferences != references {
							t.Fatalf("constructor changed owners: params %+v -> %+v; AST references %d -> %d", before, after, references, m.Stats().LiveReferences)
						}
					}
					budget.FailAfter(stop)
					levels, err := e.NumLevels(f.declarations["p"])
					budget.FailAfter(-1)
					if err != nil {
						failures++
						var cleanup memory.ConstructionFailure
						if errors.As(err, &cleanup) {
							must(cleanup.Cleanup())
						}
						var native memory.Failure
						if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &native) || native.Kind != memory.FailureMemout) {
							t.Fatal("unexpected construction failure", err)
						}
						if e.spacer != nil {
							t.Fatal("failed constructor published a context")
						}
						checkOwners()
						levels, err = e.NumLevels(f.declarations["p"])
						must(err)
					} else {
						completed = true
					}
					if levels != 0 || e.spacer == nil {
						t.Fatal("pre-query context did not recover", levels, e.spacer)
					}
					initialized, err := e.spacer.Initialized()
					must(err)
					if initialized {
						t.Fatal("property query installed a rule program")
					}
					must(e.closeSpacerForReplacement())
					checkOwners()
				})
				if t.Failed() {
					break
				}
			}
			if failures == 0 || !completed {
				t.Fatal("campaign missed constructor failure or completion", failures, completed)
			}
		})
	}
}
