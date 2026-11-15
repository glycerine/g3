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
)

type dualityVerboseRefusingWriter struct{}

func (dualityVerboseRefusingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDualityQueryReportFollowsSourceVerbosityAndStreamState(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("duality"))
	must(e.addRuleEager(f.applications["p"], "fact"))
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	must(err)
	defer func() { must(diagnostic.SetProcessVerbosityLevel(oldLevel)) }()
	// The report is std::cout's (CreateStdoutReporter), not verbose_stream's.
	var report bytes.Buffer
	must(e.SetDualityReportOutput(&report))
	for _, level := range []uint32{0, 1, 2, 0, 1} {
		must(diagnostic.SetProcessVerbosityLevel(level))
		report.Reset()
		status, err := e.QueryPredicate(f.declarations["p"])
		must(err)
		if status != lbool.True {
			t.Fatal("reachable report fixture", level, status)
		}
		text := report.String()
		if level == 0 && text != "" {
			t.Fatal("verbosity-zero Duality report", text)
		}
		if level >= 1 && (!strings.Contains(text, "]node ") || !strings.Contains(text, "]check ")) {
			t.Fatal("missing source Duality report events", level, text)
		}
	}

	// C++ ostream insertion does not turn a sticky output failure into solver
	// failure: later events are no-ops and the fixedpoint result publishes.
	must(e.SetDualityReportOutput(dualityVerboseRefusingWriter{}))
	must(diagnostic.SetProcessVerbosityLevel(1))
	status, err := e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("refused report stream changed query result", status)
	}
	report.Reset()
	must(e.SetDualityReportOutput(&report))
	status, err = e.QueryPredicate(f.declarations["p"])
	must(err)
	if status != lbool.True {
		t.Fatal("query after refused report", status)
	}
	if text := report.String(); text == "" || bytes.Contains([]byte(text), []byte("error")) {
		t.Fatal("report stream replacement", text)
	}
}
