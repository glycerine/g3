package fp

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source: dl_context.cpp property entry points call ensure_engine before
// dispatching to their PDR/Spacer owner, even before the first query.
func TestConfiguredPrequeryPropertyDispatch(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr", "spacer"} {
		for _, operation := range []string{"levels", "cover", "add-cover", "invariant"} {
			t.Run(backend+"/"+operation, func(t *testing.T) {
				f := newDirectFactFixture(t)
				e, m := f.engine, f.manager
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				must(e.globals.SourceSetName("fixedpoint.engine", backend))
				must(e.globals.SourceSetName("fixedpoint.xform.slice", "false"))
				p := f.declarations["p"]
				yes, err := m.MkTrue()
				must(err)
				switch operation {
				case "levels":
					_, err = e.NumLevels(p)
				case "cover":
					var ref ast.ASTRef
					ref, err = e.CoverDelta(-1, p)
					if err == nil {
						h, cause := m.RefGet(ref)
						must(cause)
						if !m.IsTrue(h) {
							t.Fatal("empty prequery cover", h)
						}
						must(m.DelRef(&ref))
					}
				case "add-cover":
					err = e.AddCover(0, p, yes)
				case "invariant":
					err = e.AddInvariant(p, yes)
				}
				must(err)
				selected, err := e.SelectedEngine()
				must(err)
				if selected.String() != backend {
					t.Fatal("property dispatched to wrong configured backend", selected)
				}
				must(e.globals.SourceSetName("fixedpoint.engine", "datalog"))
				_, err = e.NumLevels(p)
				must(err)
				selected, err = e.SelectedEngine()
				must(err)
				if selected.String() != backend {
					t.Fatal("property did not cache source selection", selected)
				}
			})
		}
	}
}

func TestConfiguredPropertyDispatchPreservesSlicingGuard(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.engine", "pdr"))
	yes, err := m.MkTrue()
	fragmentMust(t, err)
	err = e.AddCover(0, f.declarations["p"], yes)
	if err == nil || !strings.Contains(err.Error(), "Covers are incompatible with slicing") {
		t.Fatal("configured dispatch bypassed slicing guard", err)
	}
}
