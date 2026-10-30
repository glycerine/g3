package cmd_context

import (
	"errors"
	"io"
	"testing"
)

type echoShortWriter struct{}

func (echoShortWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	return len(value) - 1, nil
}

func TestWriteEchoRejectsNilShortAndClosedDestinations(t *testing.T) {
	context, err := NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.WriteEcho(nil, "x"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err = context.WriteEcho(echoShortWriter{}, "x"); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if err = context.WriteEcho(io.Discard, "x"); err == nil {
		t.Fatal("closed context accepted echo")
	}
}
