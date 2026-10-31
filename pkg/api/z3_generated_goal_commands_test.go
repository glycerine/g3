package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedGoalReplayLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nI 1\nI 0\nI 0\nC 348\n= 0x42\n" +
		"R\nP 0x41\nP 0x42\nC 349\nR\nP 0x41\nP 0x42\nC 900\nR\nP 0x41\nP 0x42\nC 350\nR\nP 0x41\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	var checked *replayGoal
	if err = r.RegisterCommand(900, "check-goal-flags", func(r *Z3Replayer) error {
		g, e := replayGetGoal(r)
		if e != nil {
			return e
		}
		models, e := g.value.goal.ModelsEnabled()
		if e != nil {
			return e
		}
		cores, e := g.value.goal.UnsatCoresEnabled()
		if e != nil {
			return e
		}
		if !models || cores {
			t.Fatal("goal flags", models, cores)
		}
		checked = g
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checked == nil || !checked.value.closed || len(r.goals) != 0 {
		t.Fatal("goal not cleaned up")
	}
}

func TestGeneratedGoalReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nI 0\nI 0\nI 0\nC 348\n= 0x42\nR\nP 0x41\nP 0x42\nC 349\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var goal *replayGoal
	for _, g := range r.goals {
		goal = g
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if goal == nil || !goal.value.closed || !goal.object.closed {
		t.Fatal("unreleased goal not closed")
	}
}
