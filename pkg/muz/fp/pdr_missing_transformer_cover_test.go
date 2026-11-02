package fp

// Source: pdr_context.cpp and spacer_context.cpp context::get_cover_delta.
// A missing predicate transformer returns true before applying the context's
// model converter.

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestHornCoverMissingTransformerBypassesModelConversion(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, manager := f.engine, f.manager
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			must(e.SelectEngine(backend))
			status, err := e.QueryPredicate(f.declarations["q"])
			must(err)
			if status != lbool.True {
				t.Fatal("fixture query", status)
			}
			falsity, err := manager.MkFalse()
			must(err)
			formula, err := manager.NewRef(falsity)
			must(err)
			conversion := hornModelConversion{kind: hornConvertCone, cone: transforms.ConeModelStep{{Declaration: f.declarations["u"], Formula: formula}}}
			if backend == "spacer" {
				prior := len(e.spacerConversions)
				e.spacerConversions = append(e.spacerConversions, conversion)
				defer func() {
					e.spacerConversions = e.spacerConversions[:prior]
					must(manager.DelRef(&formula))
				}()
			} else {
				prior := len(e.pdrConversions)
				e.pdrConversions = append(e.pdrConversions, conversion)
				defer func() {
					e.pdrConversions = e.pdrConversions[:prior]
					must(manager.DelRef(&formula))
				}()
			}
			cover, err := e.CoverDelta(2, f.declarations["u"])
			must(err)
			defer func() { must(manager.DelRef(&cover)) }()
			handle, err := manager.RefGet(cover)
			must(err)
			if !manager.IsTrue(handle) {
				t.Fatal("missing transformer applied model conversion", handle)
			}
		})
	}
}
