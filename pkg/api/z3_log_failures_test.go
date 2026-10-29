package api

import (
	"errors"
	"io"
	"testing"
)

type failingZ3LogWriter struct {
	writes int
	err    error
}

func (w *failingZ3LogWriter) Write(value []byte) (int, error) {
	w.writes++
	return 0, w.err
}

type shortZ3LogWriter struct{}

func (shortZ3LogWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }

func TestZ3LogWriterLatchesWriteFailures(t *testing.T) {
	want := errors.New("disk full")
	output := &failingZ3LogWriter{err: want}
	log, _ := NewZ3LogWriter(output)
	if err := log.Int64(1); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	if err := log.Int64(2); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	if output.writes != 1 {
		t.Fatalf("failure was not latched: %d writes", output.writes)
	}
	if err := log.Close(); !errors.Is(err, want) {
		t.Fatalf("close got %v", err)
	}
}

func TestZ3LogWriterRejectsNilAndShortWrites(t *testing.T) {
	if _, err := NewZ3LogWriter(nil); err != ErrZ3LogWriter {
		t.Fatalf("got %v", err)
	}
	log, _ := NewZ3LogWriter(shortZ3LogWriter{})
	if err := log.ResetArguments(); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got %v", err)
	}
	var nilLog *Z3LogWriter
	if err := nilLog.ResetArguments(); err != ErrZ3LogClosed {
		t.Fatalf("got %v", err)
	}
}
