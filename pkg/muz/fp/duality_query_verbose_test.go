package fp

// Source: duality_dl_interface.cpp:310, IF_VERBOSE(1, report=1), and
// duality_solver.cpp StreamReporter writing to std::cout.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type dualityVerboseRefusingWriter struct{}

func (dualityVerboseRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDualityQueryReportFollowsSourceVerbosityAndStreamState(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("duality"))
	must(e.AddRule(f.applications["p"], "fact"))
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	must(err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	must(err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	must(err)
	refusing, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget(), BorrowedWriter: dualityVerboseRefusingWriter{}})
	must(err)
	defer func() {
		must(diagnostic.SetProcessVerbosityLevel(oldLevel))
		must(diagnostic.SetProcessVerboseStream(oldStream))
		must(refusing.Close())
		must(stream.Close())
	}()
	must(diagnostic.SetProcessVerboseStream(stream))
	for _, level := range []uint32{0, 1, 2, 0, 1} {
		must(diagnostic.SetProcessVerbosityLevel(level))
		must(stream.ResetText())
		status, err := e.QueryPredicate(f.declarations["p"])
		must(err)
		if status != lbool.True {
			t.Fatal("reachable report fixture", level, status)
		}
		text, err := stream.String()
		must(err)
		if level == 0 && text != "" {
			t.Fatal("verbosity-zero Duality report", text)
		}
		if level >= 1 && (!strings.Contains(text, "]node ") || !strings.Contains(text, "]check ")) {
			t.Fatal("missing source Duality report events", level, text)
		}
	}

	// C++ ostream insertion does not turn a sticky output failure into solver
	// failure. NumericTextStream marks the transport bad and subsequent events
	// become no-ops, so the fixedpoint result must still publish.
	must(diagnostic.SetProcessVerboseStream(refusing))
	must(diagnostic.SetProcessVerbosityLevel(1))
	status, err := e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True || !refusing.FileBad {
		t.Fatal("refused verbose stream changed query result or stayed good", status, refusing.FileBad)
	}
	must(diagnostic.SetProcessVerboseStream(stream))
	must(stream.ResetText())
	status, err = e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("query after refused report", status)
	}
	text, err := stream.String()
	must(err)
	if text == "" || bytes.Contains([]byte(text), []byte("error")) {
		t.Fatal("report stream replacement", text)
	}
}
