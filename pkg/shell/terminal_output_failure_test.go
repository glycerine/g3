package shell

import (
	"errors"
	"io"
	"testing"
)

type shellShortWriter struct{ remaining int }

func (w *shellShortWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, nil
	}
	if len(p) > w.remaining {
		p = p[:w.remaining]
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestShellTerminalOutputPreservesReachedShortWritePrefix(t *testing.T) {
	for _, write := range []func(io.Writer) error{WriteUsage, WriteVersion, func(out io.Writer) error { return WriteCommandError(out, "bad") }} {
		writer := &shellShortWriter{remaining: 7}
		if err := write(writer); !errors.Is(err, io.ErrShortWrite) || writer.remaining != 0 {
			t.Fatalf("short output err=%v remaining=%d", err, writer.remaining)
		}
	}
	if err := WriteUsage(nil); !errors.Is(err, ErrShellOptionService) {
		t.Fatal(err)
	}
	if err := WriteVersion(nil); !errors.Is(err, ErrShellOptionService) {
		t.Fatal(err)
	}
}
