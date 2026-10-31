package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedTacticReplayLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"skip\"\nC 363\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n" +
		"R\nP 0x41\nC 377\n= 0x43\nR\nP 0x41\nP 0x43\nC 364\nR\nP 0x41\nC 378\n= 0x44\nR\nP 0x41\nP 0x44\nC 364\n" +
		"R\nP 0x41\nC 380\n= 0x45\nR\nP 0x41\nP 0x45\nC 364\nR\nP 0x41\nP 0x42\nC 900\n" +
		"R\nP 0x41\nP 0x42\nC 365\nR\nP 0x41\nP 0x43\nC 365\nR\nP 0x41\nP 0x44\nC 365\nR\nP 0x41\nP 0x45\nC 365\nR\nP 0x41\nC 8\n"
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
	var checked *replayTactic
	if err = r.RegisterCommand(900, "check-tactic", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		checked, e = replayTacticAt(r, owner, 1)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checked == nil || !checked.value.closed || len(r.tactics) != 0 {
		t.Fatal("tactic cleanup")
	}
}

func TestGeneratedTacticReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 377\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n"
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
	var value *replayTactic
	for _, tactic := range r.tactics {
		value = tactic
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if value == nil || !value.value.closed || !value.object.closed {
		t.Fatal("unreleased tactic not closed")
	}
}

func TestGeneratedTacticCompositionReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 377\n= 0x42\nR\nP 0x41\nP 0x42\nC 364\n" +
		"R\nP 0x41\nC 378\n= 0x43\nR\nP 0x41\nP 0x43\nC 364\nR\nP 0x41\nC 13\n= 0x44\nR\nP 0x41\nP 0x44\nC 14\n" +
		"R\nP 0x41\nP 0x42\nP 0x42\nC 369\n= 0x45\nR\nP 0x41\nP 0x45\nC 364\n" +
		"R\nP 0x41\nP 0x43\nP 0x42\nC 370\n= 0x46\nR\nP 0x41\nP 0x46\nC 364\n" +
		"R\nP 0x41\nU 2\nP 0x43\nP 0x42\np 2\nC 371\n= 0x47\nR\nP 0x41\nP 0x47\nC 364\n" +
		"R\nP 0x41\nP 0x42\nP 0x42\nC 372\n= 0x48\nR\nP 0x41\nP 0x48\nC 364\n" +
		"R\nP 0x41\nP 0x42\nU 100\nC 373\n= 0x49\nR\nP 0x41\nP 0x49\nC 364\n" +
		"R\nP 0x41\nP 0x42\nU 2\nC 376\n= 0x50\nR\nP 0x41\nP 0x50\nC 364\n" +
		"R\nP 0x41\nP 0x42\nP 0x44\nC 381\n= 0x51\nR\nP 0x41\nP 0x51\nC 364\n" +
		"R\nP 0x41\nP 0x42\nC 365\nR\nP 0x41\nP 0x43\nC 365\nR\nP 0x41\nP 0x44\nC 15\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x47\nP 0x48\nP 0x49\nP 0x50\nP 0x51\nC 900\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedASTCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-composed-tactics", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 7; pos++ {
			command, e := replayTacticAt(r, owner, pos)
			if e != nil {
				return e
			}
			goal, e := owner.context.NewGoal(false, false, false)
			if e != nil {
				return e
			}
			result, e := command.value.Apply(goal)
			if e != nil {
				_ = goal.Close()
				return e
			}
			n, e := result.NumSubgoals()
			if e != nil {
				_ = result.Close()
				_ = goal.Close()
				return e
			}
			if n != 1 {
				t.Fatal("composed tactic result", pos, n)
			}
			if e = result.Close(); e != nil {
				return e
			}
			if e = goal.Close(); e != nil {
				return e
			}
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("tactic composition not checked")
	}
}
