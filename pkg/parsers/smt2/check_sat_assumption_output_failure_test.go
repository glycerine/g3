package smt2

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2DirectCheckSatPropagatesStatusOutputFailure(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	err = ParseCommands(context, strings.NewReader("(declare-const a Bool)(check-sat a)"),
		&directCheckSatRefusingWriter{remaining: 1})
	if err == nil || !strings.Contains(err.Error(), io.ErrClosedPipe.Error()) {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
}

type directCheckSatRefusingWriter struct{ remaining int }

func (w *directCheckSatRefusingWriter) Write(data []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(data) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(data)
	return len(data), nil
}
