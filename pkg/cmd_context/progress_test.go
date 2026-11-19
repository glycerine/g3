package cmd_context

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestCommandContextCancelTracksLiveManagerWithoutEagerInitialization(t *testing.T) {
	c := contextFixture(t)
	if err := c.SetCancel(true); err != nil {
		t.Fatal(err)
	}
	if c.manager != nil {
		t.Fatal("cancel initialized a cold manager")
	}
	m, err := c.Manager()
	contextMust(t, err)
	contextMust(t, c.SetCancel(true))
	canceled, err := m.Canceled()
	contextMust(t, err)
	if !canceled {
		t.Fatal("cancel flag was not set")
	}
	contextMust(t, c.SetCancel(false))
	canceled, err = m.Canceled()
	contextMust(t, err)
	if canceled {
		t.Fatal("cancel flag was not reset")
	}
}

func TestCommandContextProgressCallbacksMatchSourceOutputSurface(t *testing.T) {
	c := contextFixture(t)
	contextMust(t, c.SetLogic("QF_UF"))
	_, err := c.Manager()
	contextMust(t, err)
	var output bytes.Buffer
	contextMust(t, c.SetRegularOutput(&output))
	c.FastProgressSample()
	if output.Len() != 0 {
		t.Fatal("fast source progress callback produced output", output.String())
	}
	c.SlowProgressSample()
	got := output.String()
	if !strings.HasPrefix(got, "(progress\n") || !strings.Contains(got, "(labels") || !strings.HasSuffix(got, "))\n") {
		t.Fatal("slow progress output", got)
	}
}

func TestCommandContextRunSignalsAcrossReset(t *testing.T) {
	c := contextFixture(t)
	done := make(chan struct{})
	signals := &control.RunSignals{Done: done}
	previous := c.SetRunSignals(signals)
	if c.manager != nil || previous != nil {
		t.Fatal("attaching run signals initialized the manager")
	}
	m, err := c.Manager()
	contextMust(t, err)
	if m.ResourceLimit().RunSignals != signals {
		t.Fatal("first manager lost run signals")
	}
	contextMust(t, c.Reset())
	m, err = c.Manager()
	contextMust(t, err)
	limit := m.ResourceLimit()
	close(done)
	contextMust(t, limit.Push(100))
	before := limit.Count()
	if limit.Inc() || !limit.CancelFlag() || limit.CancelMessage() != "canceled" || limit.Count() != before+1 {
		t.Fatal("new manager failed to observe timeout at its existing resource check")
	}
	limit.Pop()
	contextMust(t, c.SetCancel(false))
	if limit.Inc() {
		t.Fatal("command-local reset cleared the run deadline")
	}
	c.SetRunSignals(previous)
	if !limit.Inc() {
		t.Fatal("run deadline leaked into the next use of this context")
	}
}
