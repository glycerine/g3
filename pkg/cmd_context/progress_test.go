package cmd_context

import (
	"bytes"
	"strings"
	"testing"
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
