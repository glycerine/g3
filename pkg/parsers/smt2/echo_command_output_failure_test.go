package smt2

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

type echoRefusingWriter struct{ remaining int }

func (w *echoRefusingWriter) Write(data []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(data) > w.remaining {
		written := w.remaining
		w.remaining = 0
		return written, io.ErrClosedPipe
	}
	w.remaining -= len(data)
	return len(data), nil
}

func TestSMT2CompliantEchoPropagatesPartialOutputFailure(t *testing.T) {
	for _, accepted := range []int{0, 1, 4, 7} {
		context, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		if err = context.SetBoolOption("smtlib2_compliant", true); err != nil {
			t.Fatal(err)
		}
		context.PrintSuccess = false
		err = ParseCommands(context, strings.NewReader("(echo \"quoted\")"),
			&echoRefusingWriter{remaining: accepted})
		if err == nil || !strings.Contains(err.Error(), io.ErrClosedPipe.Error()) {
			t.Fatal(accepted, err)
		}
		if closeErr := context.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}
