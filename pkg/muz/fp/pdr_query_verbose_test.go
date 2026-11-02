package fp

// Source: pdr_dl_interface.cpp:153, IF_VERBOSE(2, m_ctx.display_rules(...)).

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestPDRQueryDisplaysTransformedRulesAtSourceVerbosity(t *testing.T) {
	for _, backend := range []string{"pdr", "qpdr"} {
		t.Run(backend, func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			fragmentMust(t, f.engine.SelectEngine(backend))
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
			fragmentMust(t, diagnostic.SetProcessVerboseStream(stream))
			for _, level := range []uint32{2, 0, 1, 2, 3, 0} {
				fragmentMust(t, diagnostic.SetProcessVerbosityLevel(level))
				fragmentMust(t, stream.ResetText())
				status, err := f.engine.QueryPredicate(f.declarations["q"])
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("query", status)
				}
				var expected bytes.Buffer
				if level >= 2 {
					fragmentMust(t, f.engine.pdrRules.WriteDisplay(&expected))
				}
				got, err := stream.String()
				fragmentMust(t, err)
				if got != expected.String() {
					t.Fatalf("verbosity %d: got %q; want %q", level, got, expected.String())
				}
			}
		})
	}
}
