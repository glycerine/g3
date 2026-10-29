package cmd_context

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func echoGoal(t *testing.T, c *Context) *tactic.Goal {
	t.Helper()
	manager, err := c.Manager()
	contextMust(t, err)
	goal, err := tactic.NewGoal(manager, false, false, false)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, goal.Close()) })
	return goal
}

func applyEcho(t *testing.T, selected tactic.Tactic, goal *tactic.Goal) {
	t.Helper()
	contextMust(t, selected.IncRef())
	var result tactic.TacticResult
	contextMust(t, selected.Apply(goal, &result))
	if len(result.Goals) != 1 || result.Goals[0] != goal {
		t.Fatal("echo tactic did not preserve its input goal")
	}
	contextMust(t, result.Close())
	contextMust(t, selected.DecRef())
}

func TestEchoTacticsUseSeparateContextStreamsAndSourceFormatting(t *testing.T) {
	c := contextFixture(t)
	goal := echoGoal(t, c)
	var regular, diagnostic bytes.Buffer
	contextMust(t, c.SetRegularOutput(&regular))
	contextMust(t, c.SetDiagnosticOutput(&diagnostic))

	first, err := NewEchoTactic(c, "prefix", false)
	contextMust(t, err)
	applyEcho(t, first, goal)
	last, err := NewEchoTactic(c, " suffix", true)
	contextMust(t, err)
	applyEcho(t, last, goal)
	if regular.String() != "prefix suffix\n" || diagnostic.Len() != 0 {
		t.Fatal("regular echo output", regular.String(), diagnostic.String())
	}

	probe, err := tactic.NewProbe(goal.Manager().Budget(), tactic.ProbeConstant, 1.23456789)
	contextMust(t, err)
	message := "value"
	probeEcho, err := NewProbeValueTactic(c, &message, probe, true)
	contextMust(t, err)
	applyEcho(t, probeEcho, goal)
	if diagnostic.String() != "value 1.23457\n" || regular.String() != "prefix suffix\n" {
		t.Fatal("probe echo output", regular.String(), diagnostic.String())
	}
	if err = probe.Check(); err == nil {
		t.Fatal("probe reference survived final tactic release")
	}
}

type echoRefusalWriter struct {
	writes int
	err    error
}

func (w *echoRefusalWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.err != nil {
		return 0, w.err
	}
	return len(p) - 1, nil
}

func TestEchoTacticOutputFailuresPrecedeGoalPublication(t *testing.T) {
	c := contextFixture(t)
	goal := echoGoal(t, c)
	want := errors.New("echo output refused")
	for _, writer := range []*echoRefusalWriter{{err: want}, {}} {
		contextMust(t, c.SetRegularOutput(writer))
		selected, err := NewEchoTactic(c, "text", true)
		contextMust(t, err)
		contextMust(t, selected.IncRef())
		var result tactic.TacticResult
		err = selected.Apply(goal, &result)
		if writer.err != nil {
			if !errors.Is(err, want) {
				t.Fatal("lost output failure", err)
			}
		} else if !errors.Is(err, io.ErrShortWrite) {
			t.Fatal("short write accepted", err)
		}
		if len(result.Goals) != 0 {
			t.Fatal("failed output published a goal")
		}
		contextMust(t, selected.DecRef())
	}
}
