package shell_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/shell"
)

type rejectingWriter struct{ err error }

func (w rejectingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestSMTLIBFrontendReturnsScannerErrorAndStillAttemptsStatistics(t *testing.T) {
	frontend, _, output := newBorrowedFrontend(t, "(assert true) (assert", true, steppedClock(0, 1_000_000_000))
	err := frontend.Run()
	var scannerError *smt2.ScannerError
	if err == nil || !errors.As(err, &scannerError) {
		t.Fatalf("error %v", err)
	}
	if !strings.Contains(output.String(), ":total-time   1.00") {
		t.Fatalf("missing final statistics %q", output.String())
	}
	result, resultErr := frontend.Result()
	if resultErr != nil || result.ParseError == nil || result.StatisticsError != nil {
		t.Fatalf("result %#v, error %v", result, resultErr)
	}
}

func TestSMTLIBFrontendReturnsOutputTransportFailure(t *testing.T) {
	sentinel := errors.New("transport unavailable")
	frontend, err := shell.NewSMTLIBFrontend(shell.SMTLIBFrontendConfig{
		Input: strings.NewReader("(check-sat)"), Output: rejectingWriter{sentinel},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer frontend.Close()
	if err = frontend.Run(); err == nil || !strings.Contains(err.Error(), sentinel.Error()) {
		t.Fatalf("error %v", err)
	}
}

func TestSMTLIBFrontendRejectsMissingTransports(t *testing.T) {
	for _, config := range []shell.SMTLIBFrontendConfig{
		{Input: strings.NewReader(""), Output: nil},
		{Input: nil, Output: io.Discard},
	} {
		if _, err := shell.NewSMTLIBFrontend(config); err == nil {
			t.Fatal("accepted missing transport")
		}
	}
}
