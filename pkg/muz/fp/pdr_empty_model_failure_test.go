package fp

// Source: pdr_dl_interface.cpp empty-program model diagnostic temporary model.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDREmptyProgramModelDiagnosticFailureOwnershipAndRetry(t *testing.T) {
	complete, failures := false, 0
	for ordinal := int64(0); ordinal < 512 && !complete; ordinal++ {
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			fragmentMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			fragmentMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: f.manager.Budget()})
			fragmentMust(t, err)
			defer func() {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				fragmentMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				fragmentMust(t, stream.Close())
			}()
			fragmentMust(t, diagnostic.SetProcessVerbosityLevel(0))
			fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
			status, err := f.engine.QueryPredicate(f.declarations["u"])
			fragmentMust(t, err)
			if status != lbool.False {
				t.Fatal("warm empty-program query", status)
			}
			fragmentMust(t, diagnostic.SetProcessVerbosityLevel(1))
			before := f.manager.Stats().LiveReferences
			f.manager.Budget().FailAfter(ordinal)
			failure := f.engine.writePDREmptyProgramModel(f.declarations["u"])
			f.manager.Budget().FailAfter(-1)
			if failure != nil {
				failures++
				var cleanup memory.ConstructionFailure
				if errors.As(failure, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
				}
				var resource memory.Failure
				if !errors.Is(failure, arena.ErrInjectedFailure) && (!errors.As(failure, &resource) || resource.Kind != memory.FailureMemout) {
					t.Fatal("unexpected diagnostic failure", failure)
				}
			} else {
				complete = true
			}
			if f.manager.Stats().LiveReferences != before {
				t.Fatal("diagnostic leaked references", ordinal, failure)
			}
			fragmentMust(t, stream.ResetText())
			fragmentMust(t, f.engine.writePDREmptyProgramModel(f.declarations["u"]))
			output, err := stream.String()
			fragmentMust(t, err)
			if output == "" {
				t.Fatal("retry omitted converted model")
			}
			if f.manager.Stats().LiveReferences != before {
				t.Fatal("diagnostic retry leaked references")
			}
		})
	}
	if !complete || failures == 0 {
		t.Fatal("incomplete diagnostic allocation campaign", complete, failures)
	}
}
