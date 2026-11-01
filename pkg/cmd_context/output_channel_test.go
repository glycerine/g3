package cmd_context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestDiagnosticOutputChannelEchoAndReplacement(t *testing.T) {
	c := contextFixture(t)
	path := filepath.Join(t.TempDir(), "diagnostic.out")
	contextMust(t, c.SetStringOption("diagnostic-output-channel", path))
	name, ok, err := c.GetOption("diagnostic-output-channel")
	contextMust(t, err)
	if !ok || name != path {
		t.Fatal("diagnostic channel name", name, ok)
	}
	goal := echoGoal(t, c)
	probe, err := tactic.NewProbe(goal.Manager().Budget(), tactic.ProbeConstant, 2)
	contextMust(t, err)
	message := "probe"
	selected, err := NewProbeValueTactic(c, &message, probe, true)
	contextMust(t, err)
	applyEcho(t, selected, goal)
	content, err := os.ReadFile(path)
	contextMust(t, err)
	if string(content) != "probe 2\n" {
		t.Fatal("diagnostic file", string(content))
	}
	contextMust(t, c.SetStringOption("diagnostic-output-channel", "stderr"))
	name, ok, err = c.GetOption("diagnostic-output-channel")
	contextMust(t, err)
	if !ok || name != "stderr" {
		t.Fatal("replacement channel", name, ok)
	}
}
