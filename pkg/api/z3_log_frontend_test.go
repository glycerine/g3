package api

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/memory"
)

func TestZ3LogFrontendReplaysAndReports(t *testing.T) {
	var output, diagnostics bytes.Buffer
	r, err := NewZ3Replayer(strings.NewReader("V \"4.8.0\"\nM \"hello\"\nR\nC 7\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	calls := 0
	if err = r.RegisterCommand(7, "test", func(*Z3Replayer) error { calls++; return nil }); err != nil {
		t.Fatal(err)
	}
	clockValues := []uint64{0, 1500000000}
	clock := func() (uint64, error) { value := clockValues[0]; clockValues = clockValues[1:]; return value, nil }
	if err = ReplayZ3LogStream(r, &output, &diagnostics, memory.NewProcessAccounting(), clock); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || diagnostics.Len() != 0 || output.String() != "hello\nmax. heap size:     0 Mbytes\ntime:               1.5\n" {
		t.Fatal("log replay", calls, output.String(), diagnostics.String())
	}
}

func TestZ3LogFrontendReportsParseFailureAndWriterError(t *testing.T) {
	var output, diagnostics bytes.Buffer
	r, err := NewZ3Replayer(strings.NewReader("?\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	clock := func() (uint64, error) { return 0, nil }
	if err = ReplayZ3LogStream(r, &output, &diagnostics, memory.NewProcessAccounting(), clock); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(diagnostics.String(), "Error at line 2: ") || output.String() != "max. heap size:     0 Mbytes\ntime:               0\n" {
		t.Fatal("parse failure reports", diagnostics.String(), output.String())
	}
	r2, err := NewZ3Replayer(strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	if err = ReplayZ3LogStream(r2, closedReplayWriter{}, io.Discard, memory.NewProcessAccounting(), clock); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("output error", err)
	}
}

type closedReplayWriter struct{}

func (closedReplayWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
