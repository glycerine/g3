package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp:1499-1502.

import (
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type nonlinearBMCRefusingWriter struct{}

func (nonlinearBMCRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func installBMCVerboseStream(t *testing.T, f *pdrLifecycleFixture) *utext.NumericTextStream {
	t.Helper()
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	fragmentMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	fragmentMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget()})
	fragmentMust(t, err)
	fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
	t.Cleanup(func() {
		fragmentMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		fragmentMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		fragmentMust(t, stream.Close())
	})
	return stream
}

func addNonlinearBMCFixtureRule(t *testing.T, f *pdrLifecycleFixture) {
	t.Helper()
	body, err := f.manager.MkAnd(f.applications["p"], f.applications["q"])
	fragmentMust(t, err)
	rule, err := f.manager.MkImplies(body, f.applications["u"])
	fragmentMust(t, err)
	fragmentMust(t, f.engine.AddRule(rule, "nonlinear"))
	params, err := f.engine.params.NewValue()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, f.engine.params.Del(&params)) }()
	for _, key := range []string{"xform.inline_eager", "xform.inline_linear", "xform.subsumption_checker"} {
		name := key
		fragmentMust(t, f.engine.params.SetBoolName(&params, &name, false))
	}
	fragmentMust(t, f.engine.SetQueryParameters(&params))
}

func TestNonlinearBMCWarningMatchesSourceDispatch(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			fragmentMust(t, f.engine.SelectEngine(backend))
			addNonlinearBMCFixtureRule(t, f)
			stream := installBMCVerboseStream(t, f)
			for _, level := range []uint32{0, 2} {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(level))
				fragmentMust(t, stream.ResetText())
				status, err := f.engine.QueryPredicate(f.declarations["u"])
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("nonlinear query", level, status)
				}
				got, err := stream.String()
				fragmentMust(t, err)
				if got != nonlinearBMCWarning {
					t.Fatalf("verbosity %d warning: got %q, want %q", level, got, nonlinearBMCWarning)
				}
			}
		})
	}
}

func TestLinearBMCDoesNotEmitNonlinearWarning(t *testing.T) {
	for _, backend := range []string{"bmc", "qbmc"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			fragmentMust(t, f.engine.SelectEngine(backend))
			stream := installBMCVerboseStream(t, f)
			fragmentMust(t, diagnostic.SetProcessVerbosityLevel(2))
			status, err := f.engine.QueryPredicate(f.declarations["q"])
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("linear query", status)
			}
			got, err := stream.String()
			fragmentMust(t, err)
			if got != "" {
				t.Fatal("linear query emitted nonlinear warning", got)
			}
		})
	}
}

func TestNonlinearBMCWarningRefusalDoesNotChangeResult(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	fragmentMust(t, f.engine.SelectEngine("bmc"))
	addNonlinearBMCFixtureRule(t, f)
	stream := installBMCVerboseStream(t, f)
	refusing, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget(), BorrowedWriter: nonlinearBMCRefusingWriter{}})
	fragmentMust(t, err)
	t.Cleanup(func() {
		// Keep the process-global borrowed stream live even when an assertion
		// stops the test before the explicit replacement below.
		fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
		fragmentMust(t, refusing.Close())
	})
	fragmentMust(t, diagnostic.SetProcessVerboseStream(refusing))
	status, err := f.engine.QueryPredicate(f.declarations["u"])
	fragmentMust(t, err)
	if status != lbool.True || !refusing.FileBad {
		t.Fatal("warning refusal changed query or left stream good", status, refusing.FileBad)
	}
	fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
	fragmentMust(t, stream.ResetText())
	status, err = f.engine.QueryPredicate(f.declarations["u"])
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("query after warning refusal", status)
	}
	got, err := stream.String()
	fragmentMust(t, err)
	if got != nonlinearBMCWarning {
		t.Fatalf("replacement warning: got %q, want %q", got, nonlinearBMCWarning)
	}
}
