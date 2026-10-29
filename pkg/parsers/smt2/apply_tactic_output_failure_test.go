package smt2

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

type applyTacticRefusingWriter struct {
	remaining int
}

func (w *applyTacticRefusingWriter) Write(p []byte) (int, error) {
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

func TestSMT2ApplyTacticPropagatesOutputFailure(t *testing.T) {
	const script = `(declare-const p Bool)(assert p)(apply skip)`
	for _, accepted := range []int{0, 2, 12, 18} {
		context, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		context.PrintSuccess = false
		writer := &applyTacticRefusingWriter{remaining: accepted}
		err = ParseCommands(context, strings.NewReader(script), writer)
		if err == nil || !strings.Contains(err.Error(), io.ErrClosedPipe.Error()) {
			t.Errorf("accepted %d bytes: got %v, want closed-pipe error", accepted, err)
		}
		if closeErr := context.Close(); closeErr != nil {
			t.Errorf("accepted %d bytes: close: %v", accepted, closeErr)
		}
	}
}
