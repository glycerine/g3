package cmd_context

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type reasonUnknownWriter struct{ remaining int }

func (w *reasonUnknownWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestCommandContextReasonUnknownStateAndEscaping(t *testing.T) {
	c := contextFixture(t)
	wantReason := func(want string) {
		t.Helper()
		got, err := c.ReasonUnknown()
		contextMust(t, err)
		if got != want {
			t.Fatalf("reason %q, want %q", got, want)
		}
	}
	const noCheck = "state of the most recent check-sat command is not known"
	wantReason(noCheck)
	failed := tactic.NewFailTactic()
	contextMust(t, failed.IncRef())
	t.Cleanup(func() { contextMust(t, failed.DecRef()) })
	status, err := c.CheckSatUsing(failed, nil)
	contextMust(t, err)
	if status != lbool.Undef {
		t.Fatal("failed tactic status", status)
	}
	wantReason("fail tactic")
	contextMust(t, c.tacticResult.SetReasonUnknown("a\"b\\c\nend\x00ignored"))
	var output bytes.Buffer
	contextMust(t, c.WriteReasonUnknown(&output))
	if got, want := output.String(), "(:reason-unknown \"a\\\"b\\c\nend\")\n"; got != want {
		t.Fatalf("escaped output %q, want %q", got, want)
	}
	for _, allowance := range []int{0, len("(:reason-unknown \""), len(output.String()) - 3} {
		if err := c.WriteReasonUnknown(&reasonUnknownWriter{remaining: allowance}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("writer failure lost", allowance, err)
		}
	}
	if err := c.WriteReasonUnknown(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("nil writer", err)
	}
	contextMust(t, c.Push(1))
	wantReason(noCheck)
	status, err = c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("recovery status", status)
	}
	// The incremental smt solver reports last_failure_as_string(): m_unknown
	// ("unknown") after an OK search, as the fork prints.
	wantReason("unknown")
	contextMust(t, c.Pop(1))
	wantReason(noCheck)
	contextMust(t, c.Close())
	if _, err = c.ReasonUnknown(); err == nil {
		t.Fatal("closed context accepted reason query")
	}
}
