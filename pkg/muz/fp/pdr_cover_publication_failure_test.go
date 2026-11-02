package fp

// Source: pdr_dl_interface::get_cover_delta owning expr_ref publication.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestPDRCoverDeltaAllocationFailurePublishesNoReference(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e, manager := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	parameters, err := e.params.NewValue()
	must(err)
	defer func() { must(e.params.Del(&parameters)) }()
	for _, name := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		must(e.params.SetBoolName(&parameters, &name, false))
	}
	must(e.SetQueryParameters(&parameters))
	status, err := e.QueryPredicate(f.declarations["q"])
	must(err)
	if status != lbool.True {
		t.Fatal("fixture query", status)
	}
	property, err := manager.MkFalse()
	must(err)
	must(e.AddCover(2, f.declarations["q"], property))
	expected, err := e.CoverDelta(2, f.declarations["q"])
	must(err)
	defer func() { must(manager.DelRef(&expected)) }()
	expectedHandle, err := manager.RefGet(expected)
	must(err)
	baseline := manager.Stats().LiveReferences
	failures, success := 0, false
	for stop := int64(0); stop <= 2048 && !success; stop++ {
		manager.Budget().FailAfter(stop)
		cover, cause := e.CoverDelta(2, f.declarations["q"])
		manager.Budget().FailAfter(-1)
		if cause != nil {
			failures++
			if cover != (ast.ASTRef{}) {
				t.Fatal("failed cover publication returned an owning reference", stop, cover, cause)
			}
			var failure memory.Failure
			if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &failure) || failure.Kind != memory.FailureMemout) {
				t.Fatal("unexpected cover failure", stop, cause)
			}
			if got := manager.Stats().LiveReferences; got != baseline {
				t.Fatal("failed cover publication leaked references", stop, baseline, got)
			}
			continue
		}
		success = true
		handle, err := manager.RefGet(cover)
		must(err)
		if handle != expectedHandle {
			t.Fatal("published cover changed the installed property", handle)
		}
		must(manager.DelRef(&cover))
	}
	if failures == 0 || !success {
		t.Fatal("missing cover allocation failure/success frontier", failures, success)
	}
	cover, err := e.CoverDelta(2, f.declarations["q"])
	must(err)
	handle, err := manager.RefGet(cover)
	must(err)
	if handle != expectedHandle {
		t.Fatal("retry cover changed the installed property", handle)
	}
	must(manager.DelRef(&cover))
}
