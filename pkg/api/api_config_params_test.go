package api

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestAPIConfigSetCaughtWarningsAndOwnerLifetime(t *testing.T) {
	c, err := MkConfig()
	satOK(t, err)
	t.Cleanup(func() {
		if c.params != nil {
			satOK(t, c.Close())
		}
		satOK(t, engine.FinalizeDefaultProcess())
	})
	warnings := diagnostic.BorrowProcessWarningService()
	oldEnabled := warnings.Enabled()
	var output bytes.Buffer
	satOK(t, warnings.SetWarningStream(&output))
	satOK(t, warnings.Enable(true))
	t.Cleanup(func() {
		satOK(t, warnings.SetWarningStream(nil))
		satOK(t, warnings.Enable(oldEnabled))
	})
	satOK(t, SetParamValue(c, "PROOF", "true"))
	state, err := c.params.State()
	satOK(t, err)
	if !state.Proof || output.Len() != 0 {
		t.Fatalf("valid config setting: proof=%v warning=%q", state.Proof, output.String())
	}
	satOK(t, SetParamValue(c, "PROOF", "maybe"))
	if output.String() != "WARNING: invalid value 'maybe' for Boolean parameter 'PROOF'\n" {
		t.Fatalf("caught Boolean exception: %q", output.String())
	}
	state, err = c.params.State()
	satOK(t, err)
	if !state.Proof {
		t.Fatal("failed setting changed published config")
	}
	output.Reset()
	satOK(t, SetParamValue(c, "unknown-option", "ignored"))
	if !strings.HasPrefix(output.String(), "WARNING: unknown parameter 'unknown_option'\nLegal parameters are:\n") {
		t.Fatalf("caught unknown-name exception: %q", output.String())
	}
	satOK(t, c.Close())
	if err = SetParamValue(c, "proof", "false"); err != ErrConfigOwner {
		t.Fatalf("closed config accepted: %v", err)
	}
}

func TestAPIContextUpdateParamValueSourcePath(t *testing.T) {
	c, err := NewSMTContext()
	satOK(t, err)
	if err = c.UpdateParamValue("proof", "true"); err != nil {
		t.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	if enabled, e := m.ProofsEnabled(); e != nil || !enabled {
		t.Fatalf("proof setting = %v, %v", enabled, e)
	}
	if err = c.UpdateParamValue("proof", "not-a-boolean"); err == nil {
		t.Fatal("malformed context parameter accepted")
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if err = c.UpdateParamValue("model", "true"); err != ErrSMTContextOwner {
		t.Fatalf("closed context update error = %v", err)
	}
}
