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

func TestGeneratedGoalReplayOperationsAndTranslation(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0\nC 6\n= 0x42\nR\nP 0x41\nC 32\n= 0x43\n" +
		"R\nP 0x41\n$ |x|\nP 0x43\nC 51\n= 0x44\nR\nP 0x41\nI 1\nI 0\nI 0\nC 348\n= 0x45\n" +
		"R\nP 0x41\nP 0x45\nC 349\nR\nP 0x41\nP 0x45\nP 0x44\nC 352\n" +
		"R\nP 0x41\nP 0x45\nC 351\nR\nP 0x41\nP 0x45\nC 353\nR\nP 0x41\nP 0x45\nC 354\n" +
		"R\nP 0x41\nP 0x45\nC 356\nR\nP 0x41\nP 0x45\nU 0\nC 357\n= 0x46\n" +
		"R\nP 0x41\nP 0x45\nC 358\nR\nP 0x41\nP 0x45\nC 359\nR\nP 0x41\nP 0x45\nC 360\nR\nP 0x41\nP 0x45\nC 362\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nP 0x46\nC 900\nR\nP 0x41\nP 0x45\nP 0x42\nC 361\n= 0x47\n" +
		"R\nP 0x42\nP 0x47\nC 349\nR\nP 0x41\nP 0x45\nC 355\nR\nP 0x41\nP 0x45\nP 0x46\nC 901\n" +
		"R\nP 0x41\nP 0x45\nC 350\nR\nP 0x41\nC 8\nR\nP 0x42\nP 0x47\nU 0\nC 357\n= 0x48\n" +
		"R\nP 0x42\nP 0x47\nP 0x48\nC 902\nR\nP 0x42\nP 0x47\nC 350\n"
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
	checks := 0
	if err = r.RegisterCommand(900, "check-goal-formula", func(r *Z3Replayer) error {
		g, e := replayGetGoal(r)
		if e != nil {
			return e
		}
		a, e := r.astObject(g.owner, 2)
		if e != nil {
			return e
		}
		b, e := r.astObject(g.owner, 3)
		if e != nil {
			return e
		}
		size, e := g.value.Size()
		if e != nil {
			return e
		}
		if a.handle != b.handle || size != 1 {
			t.Fatal("goal formula", size, a.handle, b.handle)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-reset-formula", func(r *Z3Replayer) error {
		g, e := replayGetGoal(r)
		if e != nil {
			return e
		}
		size, e := g.value.Size()
		if e != nil {
			return e
		}
		saved, e := r.astObject(g.owner, 2)
		if e != nil {
			return e
		}
		if size != 0 || saved.handle.IsNull() {
			t.Fatal("goal reset or saved formula", size)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(902, "check-translated-goal", func(r *Z3Replayer) error {
		g, e := replayGetGoal(r)
		if e != nil {
			return e
		}
		result, e := r.astObject(g.owner, 2)
		if e != nil {
			return e
		}
		size, e := g.value.Size()
		if e != nil {
			return e
		}
		text, e := g.owner.context.ASTString(result.handle, APIASTPrintSMT2)
		if e != nil {
			return e
		}
		if size != 1 || text != "x" {
			t.Fatal("translated goal", size, text)
		}
		checks++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checks != 3 {
		t.Fatal("missing goal checks", checks)
	}
}
