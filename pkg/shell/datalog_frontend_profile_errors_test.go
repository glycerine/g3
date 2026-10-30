package shell

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type datalogProfileRefusingWriter struct{ remaining int }

func (w *datalogProfileRefusingWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestDatalogFrontendProfileLifecycleAndWriterErrors(t *testing.T) {
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = frontend.Profile(); !errors.Is(err, ErrDatalogFrontend) {
		t.Fatalf("pre-run profile error %v", err)
	}
	if err = frontend.WriteProfile(nil); !errors.Is(err, ErrDatalogFrontend) {
		t.Fatalf("nil writer error %v", err)
	}
	path := filepath.Join(t.TempDir(), "profile.dl")
	if err = os.WriteFile(path, []byte("\nP\nP.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(path); err != nil {
		t.Fatal(err)
	}
	if err = frontend.WriteProfile(&datalogProfileRefusingWriter{remaining: 10}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer error %v", err)
	}
}
